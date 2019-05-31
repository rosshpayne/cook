package main

import (
	_ "encoding/json"
	"fmt"
	"strconv"
	"strings"

	_ "github.com/aws/aws-sdk-go/aws"
	_ "github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/dynamodb/expression"

	_ "github.com/aws/aws-lambda-go/lambdacontext"
)

// instance data from dynamo "activity" table
type Container struct {
	Cid       string     `json:"SortK"`
	Type      string     `json:"type"` // value used to aggregate and sort mulitple containers
	Purpose   string     `json:"purpose"`
	Coord     [2]float32 `json:"coord"`
	Contains  string     `json:"contents"`
	Prelabel  string     `json:"prelabel"`  // may be redundant
	Postlabel string     `json:"postlabel"` // may be redundant
	Scale     bool       `json:"scale"`
	// two instances of this container
	Label      string     `json:"label"`  // name used in graphics system and String() which generates name using "label requirement"
	Slabel     string     `json:"slabel"` // short name
	Measure    *MeasureCT `json:"measure"`
	AltLabel   string     `json:"altLabel"`
	AltMeasure *MeasureCT `json:"altMeasure"`
	// non-dynamo
	start      int     // first id in recipe tasks where container is used
	last       int     // last id in recipe tasks where container is sourced from or recipe is complete.
	physicalId int     // each container is assigned a physical container id : 1..n
	Activity   []taskT // slice of tasks (Prep and Task activites) associated with container
}

type ContainerMap map[string]*Container // key: container.Cid e.g. CakeTin

var ContainerM ContainerMap

type DeviceT struct {
	Type      string `json:"type"`
	Set       string `json:"set"`
	Purpose   string `json:"purpose"`
	Alternate string `json:"alternate"`
	Temp      string `json:"temp"`
	Unit      string `json:"unit"`
}

type DevicesMap map[string]string

type DeviceMap map[string]DeviceT

func (s *sessCtx) loadBaseContainers() (ContainerS, error) {
	//
	// Table:  Activity
	//
	kcond := expression.KeyEqual(expression.Key("PKey"), expression.Value("A-"+s.pkey))
	expr, err := expression.NewBuilder().WithKeyCondition(kcond).Build()
	if err != nil {
		panic(err)
	}
	input := &dynamodb.QueryInput{
		KeyConditionExpression:    expr.KeyCondition(),
		FilterExpression:          expr.Filter(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
	}
	input = input.SetTableName("Recipe").SetReturnConsumedCapacity("TOTAL").SetConsistentRead(false)
	//*dynamodb.DynamoDB,
	result, err := s.dynamodbSvc.Query(input)
	if err != nil {
		return nil, fmt.Errorf("Error: in readBaseRecipeForContainers Query - %s", err.Error())
	}
	fmt.Println("loadBaseContainers: Query: A-  ConsumedCapacity: %#v\n", result.ConsumedCapacity)
	if int(*result.Count) == 0 {
		return nil, fmt.Errorf("No data found for reqRId %s in processBaseRecipe for Activity - ", s.pkey)
	}
	//ActivityS := make([]Activity, int(*result.Count))
	ActivityS := make(Activities, int(*result.Count))
	err = dynamodbattribute.UnmarshalListOfMaps(result.Items, &ActivityS)
	if err != nil {
		return nil, fmt.Errorf("** Error during UnmarshalListOfMaps in processBaseRecipe - %s", err.Error())
	}
	//
	// link activities together via next, prev, nextTask, nextPrep pointers. Order in ActivityS is sorted from dynamodb sort key.
	// not sure how useful have next, prev pointers will be but its easy to setup so keep for time being. Do use prev in other part of code.
	activityStart = &ActivityS[0]
	for i := 0; i < len(ActivityS)-1; i++ {
		ActivityS[i].next = &ActivityS[i+1]
		if i > 0 {
			ActivityS[i].prev = &ActivityS[i-1]
		}
	}
	//
	// Create maps based on AId, Ingredient (plural and singular) and Label (plural and singular)
	//
	idx := 0
	//TODO does id get used??
	s.activityS = ActivityS
	//	for _, p := range ActivityS {
	for p := &ActivityS[0]; p != nil; p = p.next {
		for _, p := range p.Prep {
			idx++
			p.id = idx
		}
		for _, p := range p.Task {
			idx++
			p.id = idx
		}
	}
	//
	ActivityM := make(map[string]*Activity)
	IngredientM := make(map[string]*Activity)
	LabelM := make(map[string]*Activity)
	//
	//for i, v := range ActivityS { // inefficient - ActivityS struct entry must be copied into v foreach loop
	// These maps are only used in generateAndSaveIndex
	//
	for v := &ActivityS[0]; v != nil; v = v.next {
		aid := strconv.Itoa(v.AId)
		ActivityM[aid] = v
		ingrd := v.Ingredient
		if len(v.Alias) > 0 {
			ingrd = v.Alias
		}
		if len(ingrd) > 0 {
			ingrd := strings.ToLower(ingrd)
			// check if ingrd not already entered into map
			if found, ok := IngredientM[ingrd]; ok {
				fmt.Println("Duplicate ingredients - find largest amount - ", found.Ingredient)
				var fe float64
				var fn float64
				//TODO algorithm to compare measures. Here we choose largest by quantity presuming its a number
				if m := found.Measure; m != nil {

					if len(m.Quantity) > 0 {
						fe, err = strconv.ParseFloat(m.Quantity, 64)
						if err != nil {
							fe = 0.0
						}
					}
					if v.Measure != nil && len(v.Measure.Quantity) > 0 {
						fn, err = strconv.ParseFloat(v.Measure.Quantity, 64)
						if err != nil {
							fn = 0.0
						}
					}
					fmt.Println("fe,fn:", fe, fn)
					if fn > fe {
						// replace
						IngredientM[ingrd] = v
						if ingrd[len(ingrd)-1] == 's' {
							// make singular entry as well
							IngredientM[ingrd[:len(ingrd)-1]] = v
						}
					}
				}
			} else {
				IngredientM[ingrd] = v
				if ingrd[len(ingrd)-1] == 's' {
					// make singular entry as well
					IngredientM[ingrd[:len(ingrd)-1]] = v
				}
			}
		}
		if len(v.Label) > 0 {
			label := strings.ToLower(v.Label)
			LabelM[label] = v
			if label[len(label)-1] == 's' {
				// make singular entry as well
				LabelM[label[:len(label)-1]] = v
			}
		}
	}
	//
	// aggregate activities to recipe partitions
	// if no partition then all activities will be aggregated to the "nopart_" partition
	//
	// partM := make(map[string][]*Activity)
	// // find if there are any parts to recipe
	// for a := &ActivityS[0]; a != nil; a = a.next {
	// 	if len(a.Part) > 0 {
	// 		partM[a.Part] = append(partM[a.Part], a)
	// 	} else {
	// 		partM["nopart_"] = append(partM["nopart_"], a)
	// 	}
	// }
	// sum time for each activity so we know how long each partition will take.
	//
	// link Task Activities - taskctl is a package variable.
	//
	//var j int
	for i, v := 0, &ActivityS[0]; v != nil; v = v.next {
		if v.Task != nil {
			taskctl.start = v
			j := i
			taskctl.cnt++
			for i := j + 1; i < len(ActivityS); i++ {
				if len(ActivityS[i].Task) > 0 {
					ActivityS[j].nextTask = &ActivityS[i]
					j = i
					taskctl.cnt++
				}
			}
			break
		}
		i++
	}
	//
	// link Prep Activities - prepctl is a package variable. - emulates a Go slice structure
	//
	for i, v := 0, &ActivityS[0]; v != nil; v = v.next {
		if v.Prep != nil {
			prepctl.start = &ActivityS[i]
			j := i
			prepctl.cnt++
			for i := j + 1; i < len(ActivityS); i++ {
				if len(ActivityS[i].Prep) > 0 {
					ActivityS[j].nextPrep = &ActivityS[i]
					j = i
					prepctl.cnt++
				}
			}
			break
		}
		i++
	}
	//
	//
	// Parse Activity and generate Containers
	//  If C-0-0 type container then one its a single-activity-container (SAC) ie. a single-ingredient-container (SIC)
	//  if not a member of C-0-0 then maybe shared amoung activities.
	//
	// Table:  Container
	//
	kcond = expression.KeyEqual(expression.Key("PKey"), expression.Value("C-"+s.pkey))
	expr, err = expression.NewBuilder().WithKeyCondition(kcond).Build()
	if err != nil {
		panic(err)
	}
	input = &dynamodb.QueryInput{
		KeyConditionExpression:    expr.KeyCondition(),
		FilterExpression:          expr.Filter(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		//		ProjectionExpression:      expr.Projection(),
	}
	input = input.SetTableName("Ingredient").SetReturnConsumedCapacity("TOTAL").SetConsistentRead(false)
	//
	result, err = s.dynamodbSvc.Query(input)
	if err != nil {
		return nil, fmt.Errorf("%s", "Error in Query of container table: "+err.Error())
	}
	fmt.Println("loadBaseContainers: Query: ConsumedCapacity: %#v\n", result.ConsumedCapacity)
	if int(*result.Count) == 0 {
		fmt.Println("No container data..")
	}
	// Container lookup - given Cid give me pointer to the continer.
	ContainerM = make(ContainerMap, int(*result.Count))
	itemc := make([]*Container, int(*result.Count))
	err = dynamodbattribute.UnmarshalListOfMaps(result.Items, &itemc)
	if err != nil {
		return nil, fmt.Errorf("%s", "Error in UnmarshalMap of container table: "+err.Error())
	}
	for _, v := range itemc {
		ContainerM[v.Cid] = v
	}
	// for k, v := range ContainerM {
	// 	fmt.Printf("%s - %#v\n", k, v)
	// }
	// common containers - not recipe specific
	kcond = expression.KeyEqual(expression.Key("PKey"), expression.Value("C-0-0"))
	expr, err = expression.NewBuilder().WithKeyCondition(kcond).Build()
	if err != nil {
		panic(err)
	}
	input = &dynamodb.QueryInput{
		KeyConditionExpression:    expr.KeyCondition(),
		FilterExpression:          expr.Filter(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		//		ProjectionExpression:      expr.Projection(),
	}
	input = input.SetTableName("Ingredient").SetReturnConsumedCapacity("TOTAL").SetConsistentRead(false)
	//
	result, err = s.dynamodbSvc.Query(input)
	if err != nil {
		return nil, fmt.Errorf("%s", "Error in Query of container table: "+err.Error())
	}
	fmt.Println("loadBaseContainers: Query C-0-0: ConsumedCapacity: %#v\n", result.ConsumedCapacity)
	if int(*result.Count) == 0 {
		fmt.Println("No container data..")
	}
	ContainerSAM := make(ContainerMap, int(*result.Count))
	itemc = nil
	itemc = make([]*Container, int(*result.Count))
	err = dynamodbattribute.UnmarshalListOfMaps(result.Items, &itemc)
	if err != nil {
		return nil, fmt.Errorf("%s", "Error in UnmarshalMap of container table: "+err.Error())
	}
	for _, v := range itemc {
		ContainerSAM[v.Cid] = v
	}
	// for k, v := range ContainerSAM {
	// 	fmt.Printf("%s - %#v\n", k, v)
	// }
	itemc = nil
	//
	//
	//  Post fetch processing - assign container pointers in Activity and validate that all containers referenced exist
	//
	// for all prep, tasks
	//
	// parse Activities for containers.  Dynamically create single-activity containers and add to ContainerM as required.
	//
	for _, l := range []PrepTask{prep, task} {
		for ap := activityStart; ap != nil; ap = ap.next {
			var p []*PerformT
			switch l {
			case task:
				p = ap.Task
			case prep:
				p = ap.Prep
			}
			if len(p) == 0 {
				continue
			}
			// now compare contains defined in each activity with those registered for
			// the recipe and those that are single-activity-containers
			for idx, p := range p {
				// a prep or task in order specified in JSON not listed in dynamo - beware
				if len(p.AddToC) > 0 {
					//  containers are held in []string
					// check if container is registered or must be dynamically created
					for i := 0; i < len(p.AddToC); i++ {
						// ContainerM contains registered containers
						cId, ok := ContainerM[strings.TrimSpace(p.AddToC[i])]
						if !ok {
							// not a registered container. Now check if its a SAM container.
							sac := strings.Split(strings.TrimSpace(p.AddToC[i]), ".")
							p.AddToC[i] = sac[0]
							if cId, ok = ContainerSAM[sac[0]]; !ok {
								// is not a single activity container or not a registered container
								fmt.Printf("Error:   Container [%s] not found for %s %d\n", strings.TrimSpace(p.AddToC[i]), ap.Label, ap.AId)
								continue
							}
							// Single-Activity-Containers are not pre-configured by the user - to make life easier.
							// dynamically create a container with a new Cid, and add to ContainerM and update all references to it.
							cs := sac[0] // original Single-activity container name
							c := new(Container)
							// append Aid to make unique name
							c.Cid = p.AddToC[i] + "-" + strconv.Itoa(ap.AId)
							c.Measure = cId.Measure
							c.Label = cId.Label
							c.Type = cId.Type
							switch len(cId.Contains) {
							case 0:
								c.Contains = ap.Ingredient
							default:
								c.Contains = cId.Contains // use label attached to SAM
							}
							switch len(sac) {
							case 1:
								if len(cId.Purpose) > 0 {
									c.Purpose = cId.Purpose
								} else {
									c.Purpose = "holds"
								}
							default:
								c.Purpose = sac[1]
							}
							// register container by adding to map
							ContainerM[c.Cid] = c
							// update container id in activity
							p.AddToC[i] = c.Cid
							// search for other references in prep/tasks within the same activity and change its name
							if len(ap.Task) > 0 {
								for _, t := range ap.Task {
									for i := 0; i < len(t.SourceC); i++ {
										if t.SourceC[i] == cs {
											t.SourceC[i] = c.Cid
											break
										}
									}
									for i := 0; i < len(t.UseC); i++ {
										if t.UseC[i] == cs {
											t.UseC[i] = c.Cid
											break
										}
									}
									for i := 0; i < len(t.AddToC); i++ {
										if t.AddToC[i] == cs {
											t.AddToC[i] = c.Cid
											break
										}
									}
								}
							}
							cId = c
						}
						// activity to container edge
						p.addToCp = append(p.addToCp, cId)
						p.allCp = append(p.allCp, cId)
						// container to activity edge
						associatedTask := taskT{Type: l, Activityp: ap, Idx: idx} // task that uses the container - don't actually make use of this (?)
						cId.Activity = append(cId.Activity, associatedTask)
					}
				}

				if len(p.UseC) > 0 {
					for i := 0; i < len(p.UseC); i++ {
						// ContainerM contains registered containers
						cId, ok := ContainerM[strings.TrimSpace(p.UseC[i])]
						if !ok {
							// ContainerSAM contains single activity containers
							sac := strings.Split(strings.TrimSpace(p.UseC[i]), ".")
							p.UseC[i] = sac[0]
							if cId, ok = ContainerSAM[sac[0]]; !ok {
								// is not a single ingredient container or not a registered container
								fmt.Printf("Error:   Container [%s] not found for %s %d\n", strings.TrimSpace(p.UseC[i]), ap.Label, ap.AId)
								continue
							}
							// container referened in activity is a single-activity-container (SAP)
							// manually create container and add to ContainerM and update all references to it.
							cs := sac[0] // original non-activity-specific container name
							c := new(Container)
							c.Cid = p.UseC[i] + "-" + strconv.Itoa(ap.AId)
							c.Measure = cId.Measure
							c.Label = cId.Label
							c.Type = cId.Type
							switch len(cId.Contains) {
							case 0:
								c.Contains = ap.Ingredient
							default:
								c.Contains = cId.Contains // use label attached to SAM
							}
							switch len(sac) {
							case 1:
								if len(cId.Purpose) > 0 {
									c.Purpose = cId.Purpose
								} else {
									c.Purpose = "holds"
								}
							default:
								c.Purpose = sac[1]
							}
							// register container by adding to map
							ContainerM[c.Cid] = c
							// update name of container in Activity to <name>-AId
							p.UseC[i] = c.Cid
							// search for other references and change its name
							if len(ap.Task) > 0 {
								for _, t := range ap.Task {
									for i := 0; i < len(t.SourceC); i++ {
										if t.SourceC[i] == cs {
											t.SourceC[i] = c.Cid
											break
										}
									}
									for i := 0; i < len(t.UseC); i++ {
										if t.UseC[i] == cs {
											t.UseC[i] = c.Cid
											break
										}
									}
									for i := 0; i < len(t.AddToC); i++ {
										if t.AddToC[i] == cs {
											t.AddToC[i] = c.Cid
											break
										}
									}
								}
							}
							cId = c
						}
						p.useCp = append(p.useCp, cId)
						p.allCp = append(p.allCp, cId)
						associatedTask := taskT{Type: l, Activityp: ap, Idx: idx}
						cId.Activity = append(cId.Activity, associatedTask)
					}
				}
				if len(p.SourceC) > 0 {
					// ContainerM contains registered containers
					for i := 0; i < len(p.SourceC); i++ {
						cId, ok := ContainerM[strings.TrimSpace(p.SourceC[i])]
						if !ok {
							// ContainerSAM contains single activity containers
							sac := strings.Split(strings.TrimSpace(p.SourceC[i]), ".")
							p.SourceC[i] = sac[0]
							if cId, ok = ContainerSAM[sac[0]]; !ok {
								// is not a single ingredient container or not a registered container
								fmt.Printf("Error:   Container [%s] not found for %s %d\n", strings.TrimSpace(p.SourceC[i]), ap.Label, ap.AId)
								continue
							}
							// container referened in activity is a single-activity-container (SAP)
							// manually create container and add to ContainerM and update all references to it.
							cs := sac[0] // original non-activity-specific container name
							c := new(Container)
							c.Cid = p.SourceC[i] + "-" + strconv.Itoa(ap.AId)
							c.Measure = cId.Measure
							c.Label = cId.Label
							c.Type = cId.Type
							switch len(cId.Contains) {
							case 0:
								c.Contains = ap.Ingredient
							default:
								c.Contains = cId.Contains // use label attached to SAM
							}
							switch len(sac) {
							case 1:
								if len(cId.Purpose) > 0 {
									c.Purpose = cId.Purpose
								} else {
									c.Purpose = "holds"
								}
							default:
								c.Purpose = sac[1]
							}
							// register container by adding to map
							ContainerM[c.Cid] = c
							// update name of container in Activity to <name>-AId
							p.SourceC[i] = c.Cid
							// search for other references and change its name
							if len(ap.Task) > 0 {
								for _, t := range ap.Task {
									for i := 0; i < len(t.SourceC); i++ {
										if t.SourceC[i] == cs {
											t.SourceC[i] = c.Cid
											break
										}
									}
									for i := 0; i < len(t.UseC); i++ {
										if t.UseC[i] == cs {
											t.UseC[i] = c.Cid
											break
										}
									}
									for i := 0; i < len(t.AddToC); i++ {
										if t.AddToC[i] == cs {
											t.AddToC[i] = c.Cid
											break
										}
									}
								}
							}
							cId = c
						}
						p.sourceCp = append(p.sourceCp, cId)
						p.allCp = append(p.allCp, cId)
						associatedTask := taskT{Type: l, Activityp: ap, Idx: idx}
						cId.Activity = append(cId.Activity, associatedTask)
					}
				}
			}
		}
	}

	// check container is associated with an activity. if not delete from container map.

	for _, c := range ContainerM {
		if len(c.Activity) == 0 {
			delete(ContainerM, c.Cid)
		}
	}
	for _, c := range ContainerM {
		fmt.Printf("Container: Id: [%s]  Type: [%s]  Size:[%s]  Label: [%s] \n", c.Cid, c.Type, c.Measure.Size, c.Label)
	}
	ptS := ActivityS.GenerateTasks("T-"+s.pkey, s.recipe, s) // Read task items from recipe table instead of generate them.
	//
	// populate device map using device type as key. Maintains latest attribute values for DeviceT which
	// . can be referenced at any point in txt using {device.<deviceType>.<attribute>}
	//
	// for k, v := range DeviceM {
	// 	fmt.Printf("DeviceM  %s %v\n", k, v)
	// }
	//
	// Post processing of Containers
	//
	// find first reference to Container in the ordered instruction (ptS) list
	for _, v := range ContainerM {
		var found bool
		v.start = 99999
		for _, pt := range ptS {
			for _, c := range pt.taskp.allCp {
				if c == v {
					if c.start > pt.SortK {
						c.start = pt.SortK
						found = true
						break
					}
				}
			}
			if found {
				break
			}
		}
	}
	// find last reference to container in ptS list.
	// in most cases a container is sourced from to represent its last use
	for _, v := range ContainerM {
		var found bool
		for i := len(ptS) - 1; i >= 0; i-- {
			// find last appearance (typically sourceC). Start at last ptS and work backwards
			for _, c := range ptS[i].taskp.allCp {
				if c == v {
					if ptS[i].SortK > c.last {
						c.last = ptS[i].SortK
						found = true
						break
					}
				}
			}
			if found {
				break
			}
		}
	}
	//
	ctS := ContainerM.generateContainerUsage(s.dynamodbSvc)
	//
	DevicesM := make(DevicesMap)
	for p := activityStart; p != nil; p = p.next {
		for _, pp := range p.Prep {
			if pp.UseDevice != nil {
				typ := strings.ToLower(pp.UseDevice.Type)
				if _, ok := DevicesM[typ]; !ok {
					var str string

					pp := pp.UseDevice
					//str = pp.String()
					if len(pp.Purpose) > 0 {
						str += pp.Purpose
					}
					if len(pp.Alternate) > 0 {
						str += " Alternative: " + pp.Alternate
					}
					DevicesM[typ] = str
				}
			}
		}
		fmt.Printf("XXX %#v\n", DevicesM)
		for _, pp := range p.Task {
			if pp.UseDevice != nil {
				typ := strings.ToLower(pp.UseDevice.Type)
				if _, ok := DevicesM[typ]; !ok {
					var str string
					pp := pp.UseDevice
					//	str = pp.String()
					if len(pp.Purpose) > 0 {
						str += pp.Purpose
					}
					if len(pp.Alternate) > 0 {
						str += "Alternative: " + pp.Alternate
					}
					DevicesM[typ] = str
				}
			}
		}
	}
	ctS = append(ctS, "\n")
	ctS = append(ctS, "Utensils:")
	ctS = append(ctS, "\n")
	//
	for k, _ := range DevicesM {
		//r := &Pkey{PKey: "D-" + s.pkey, SortK: row, Device: k, Comment: v}
		ctS = append(ctS, " "+k)
	}
	for _, v := range ctS {
		fmt.Printf("%s\n", v)
	}
	return ctS, nil

} //
