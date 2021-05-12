package main

import (
	_ "encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/rosshpayne/cook/global"

	_ "github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	_ "github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/dynamodb/expression"

	_ "github.com/aws/aws-lambda-go/lambdacontext"
)

func (s *sessCtx) loadActivities() (Activities, error) {
	//
	// Table:  Activity
	//
	fmt.Println("entered loadActivities: PKey=", "A-"+s.pkey)
	kcond := expression.KeyEqual(expression.Key("PKey"), expression.Value("A-"+s.pkey))
	expr, err := expression.NewBuilder().WithKeyCondition(kcond).Build()
	if err != nil {
		return nil, fmt.Errorf("Error: in loadActivities Query - %s", err.Error())
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
		return nil, fmt.Errorf("Error: in loadActivities Query - %s", err.Error())
	}
	fmt.Println("loadActivities: Query: A-?-? ConsumedCapacity: %#v\n", result.ConsumedCapacity)
	if int(*result.Count) == 0 {
		return nil, fmt.Errorf("No data found for recipe %s in loadActivities  ", s.pkey)
	}
	//ActivityS := make([]Activity, int(*result.Count))
	ActivityS := make(Activities, int(*result.Count))
	err = dynamodbattribute.UnmarshalListOfMaps(result.Items, &ActivityS)
	if err != nil {
		return nil, fmt.Errorf("** Error during UnmarshalListOfMaps in loadActivities - %s", err.Error())
	}
	// link up activities via next,prev pointers
	for i := 0; i < len(ActivityS)-1; i++ {
		ActivityS[i].next = &ActivityS[i+1]
		if i > 0 {
			ActivityS[i].prev = &ActivityS[i-1]
		}
	}

	return ActivityS, nil
}

func (s *sessCtx) loadBaseRecipe() error {
	//
	// Table:  Activity
	//
	fmt.Println("in loadBaseRecipe pkey: ", s.pkey)
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
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case dynamodb.ErrCodeProvisionedThroughputExceededException:
				fmt.Println(dynamodb.ErrCodeProvisionedThroughputExceededException, aerr.Error())
			case dynamodb.ErrCodeResourceNotFoundException:
				fmt.Println(dynamodb.ErrCodeResourceNotFoundException, aerr.Error())
			//case dynamodb.ErrCodeRequestLimitExceeded:
			//	fmt.Println(dynamodb.ErrCodeRequestLimitExceeded, aerr.Error())
			case dynamodb.ErrCodeInternalServerError:
				fmt.Println(dynamodb.ErrCodeInternalServerError, aerr.Error())
			default:
				fmt.Println(aerr.Error())
			}
		} else {
			// Print the error, cast err to awserr.Error to get the Code and
			// Message from an error.
			fmt.Println(err.Error())
		}
		return err
	}
	fmt.Println("loadBaseRecipe: Query Recipe A- ConsumedCapacity: %#v\n", result.ConsumedCapacity)
	if int(*result.Count) == 0 {
		return fmt.Errorf("No data found for reqRId %s in loadBaseRecipe for Activity - ", s.pkey)
	} else {
		fmt.Println(" count: ", int(*result.Count))
	}
	//ActivityS := make([]Activity, int(*result.Count))
	ActivityS := make(Activities, int(*result.Count))
	err = dynamodbattribute.UnmarshalListOfMaps(result.Items, &ActivityS)
	if err != nil {
		return fmt.Errorf("** Error during UnmarshalListOfMaps in loadBaseRecipe - %s", err.Error())
	}
	if len(ActivityS) != int(*result.Count) {
		fmt.Println(" num ACtivity records: ", len(ActivityS))
	}
	//
	// link activities together via next, prev, nextTask, nextPrep pointers. Order in ActivityS is sorted from dynamodb sort key.
	// not sure how useful have next, prev pointers will be but its easy to setup so keep for time being. Do use prev in other part of code.
	activityStart = &ActivityS[0]
	for i := 0; i < len(ActivityS)-1; i++ {
		fmt.Println("Aid ", ActivityS[i].AId)
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
	// link Prep Activities - prepctl is a package variable.
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
		return fmt.Errorf("%s", "Error in Query of container table: "+err.Error())
	}
	fmt.Println("loadBaseRecipe: Query C-containers ConsumedCapacity: %#v\n", result.ConsumedCapacity)
	if int(*result.Count) == 0 {
		fmt.Println("No container data..")
	}
	// Container lookup - given Cid give me pointer to the continer.
	ContainerM = make(ContainerMap, int(*result.Count))
	itemc := make([]*Container, int(*result.Count))
	err = dynamodbattribute.UnmarshalListOfMaps(result.Items, &itemc)
	if err != nil {
		return fmt.Errorf("%s", "Error in UnmarshalMap of container table: "+err.Error())
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
		return fmt.Errorf("%s", "Error in Query of container table: "+err.Error())
	}
	fmt.Println("loadBaseRecipe: Query C-0-0  ConsumedCapacity: %#v\n", result.ConsumedCapacity)
	if int(*result.Count) == 0 {
		fmt.Println("No container data..")
	}
	ContainerSAM := make(ContainerMap, int(*result.Count))
	itemc = nil
	itemc = make([]*Container, int(*result.Count))
	err = dynamodbattribute.UnmarshalListOfMaps(result.Items, &itemc)
	if err != nil {
		return fmt.Errorf("%s", "Error in UnmarshalMap of container table: "+err.Error())
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
							// ContainerSAM contains single activity containers
							// format: <SA?>.<purpose>
							sac := strings.Split(strings.TrimSpace(p.AddToC[i]), ".")
							p.AddToC[i] = sac[0]
							if cId, ok = ContainerSAM[sac[0]]; !ok {
								// is not a single ingredient container or not a registered container
								fmt.Printf("Error:   Container [%s] not found for %s %d\n", strings.TrimSpace(p.AddToC[i]), ap.Label, ap.AId)
								continue
							}
							// Single-Activity-Containers are not pre-configured by the user into the Container repo - to make life easier.
							// dynamically create a container with a new Cid, and add to ContainerM and update all references to it.
							cs := sac[0] // original Single-activity container name
							c := new(Container)
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
								}
							default:
								c.Purpose = sac[1]
							}
							// register container by adding to map
							ContainerM[c.Cid] = c
							// update container id in activity
							p.AddToC[i] = c.Cid
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
						// activity to container edge
						p.addToCp = append(p.addToCp, cId)
						p.allCp = append(p.allCp, cId)
						// container to activity edge
						associatedTask := taskT{Type: l, Activityp: ap, Idx: idx}
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
		fmt.Printf("Container: Id: [%s]  Type: [%s]  Size:[%s]  Label: [%s] \n", c.Cid, c.Type, c.Measure.Size, c.Label)
	}
	for _, c := range ContainerM {
		if len(c.Activity) == 0 {
			delete(ContainerM, c.Cid)
		}
	}

	// populate prep/task id
	for i, p := 0, activityStart; p != nil; p = p.next {
		for _, pp := range p.Prep {
			i++
			pp.id = i
		}
		for _, pp := range p.Task {
			i++
			pp.id = i
		}
	}
	//
	// populate device map using device type as key. Maintains latest attribute values for DeviceT which
	// . can be referenced at any point in txt using {device.<deviceType>.<attribute>}
	//
	var ovenOn bool
	DeviceM := make(DeviceMap)

	for p := activityStart; p != nil; p = p.next {
		for _, pp := range p.Prep {
			if pp.UseDevice != nil {
				dt := *pp.UseDevice
				if dt.Type == "oven" {
					ovenOn = true
				}
				typ := strings.ToLower(dt.Type)
				if dt_, ok := DeviceM[typ]; ok {
					// only preserve attributes that have values
					// NB. DeviceM value is a struct not *struct
					ppU := pp.UseDevice
					if len(ppU.Set) > 0 {
						dt_.Set = ppU.Set
					}
					if len(ppU.Purpose) > 0 {
						dt_.Purpose = ppU.Purpose
					}
					if len(ppU.Alternate) > 0 {
						dt_.Alternate = ppU.Alternate
					}
					if len(ppU.Temp) > 0 {
						dt_.Temp = ppU.Temp
					}
					if len(ppU.Unit) > 0 {
						dt_.Unit = ppU.Unit
					}
					DeviceM[typ] = dt_
					dt = dt_
				} else {
					DeviceM[typ] = dt
				}
				// preserve state of Device for the prep/task id
				key := strconv.Itoa(pp.id) + "-" + dt.Type
				DeviceM[key] = dt
			}
		}
		for _, pp := range p.Task {
			if ovenOn {
				key := strconv.Itoa(pp.id) + "-" + "oven"
				DeviceM[key] = DeviceM["oven"]
			}
			if pp.UseDevice != nil {
				dt := *pp.UseDevice
				if dt.Type == "oven" {
					ovenOn = true
				}
				typ := strings.ToLower(dt.Type)
				if dt_, ok := DeviceM[typ]; ok {
					// only preserve attributes that have values
					// NB. DeviceM value is a struct not *struct
					ppU := pp.UseDevice
					if len(ppU.Set) > 0 {
						dt_.Set = ppU.Set
					}
					if len(ppU.Purpose) > 0 {
						dt_.Purpose = ppU.Purpose
					}
					if len(ppU.Alternate) > 0 {
						dt_.Alternate = ppU.Alternate
					}
					if len(ppU.Temp) > 0 {
						dt_.Temp = ppU.Temp
					}
					if len(ppU.Unit) > 0 {
						dt_.Unit = ppU.Unit
					}
					DeviceM[typ] = dt_
					dt = dt_
				} else {
					DeviceM[typ] = dt
				}
				// preserve state of Device for the Activity
				key := strconv.Itoa(pp.id) + "-" + dt.Type
				DeviceM[key] = dt
			}
		}
	}
	// for k, v := range DeviceM {
	// 	fmt.Printf("DeviceM  %s %v\n", k, v)
	// }
	//
	doubleSpace := strings.NewReplacer("  ", " ")
	//
	const (
		time int = iota
		measure
		device
		text
		voice
	)
	var (
		b       strings.Builder // supports io.Write write expanded text/verbal text to this buffer before saving to Task or Verbal fields
		context int
		str     string
		tclose  int
		topen   int
		ok      bool
	)
	//
	//  replace all {tag} in text and verbal for each activity. Ignore Link'd activites - they are only relevant at print time
	//
	// for _, v := range ActivityS {
	// 	fmt.Printf("Activity [%d] \n", v.AId)
	// 	for _, v := range v.Task {
	// 		fmt.Printf("text: [%s]\n", v.Text)
	// 	}
	// }
	var pt []*PerformT
	for _, taskType := range []PrepTask{prep, task} {
		for _, interactionType := range []int{text, voice} {
			for p := activityStart; p != nil; p = p.next {
				fmt.Println("Activity: ", p.AId, p.Ingredient)
				switch taskType {
				case prep:
					pt = p.Prep
				case task:
					pt = p.Task
				}
				for _, pt := range pt {
					//fmt.Printf("\nText:   [%s]\n", pt.Text)
					// perform over slice of preps, tasks
					switch interactionType {
					case text:
						global.Set_WriteCtx(global.UDisplay) // package variable to deterine String() formating
						str = strings.TrimLeft(pt.Text, " ")
						s := str[0]
						str = strings.ToUpper(string(s)) + str[1:]
					case voice:
						global.Set_WriteCtx(global.USay) // package variable to deterine String() formating
						if len(pt.Verbal) == 0 {
							pt.verbal = pt.text
							continue
						} else {
							str = pt.Verbal
						}
					}
					// if no {} then print and return to top of the loop
					t1 := strings.IndexByte(str, '{')
					if t1 < 0 {
						b.WriteString(str + " ")
						switch interactionType {
						case text:
							pt.text = doubleSpace.Replace(b.String())
						case voice:
							pt.verbal = doubleSpace.Replace(b.String())
						}
						b.Reset()
						continue
					}
					for tclose, topen = 0, strings.IndexByte(str, '{'); topen != -1; {
						var (
							el  string
							el2 string
						)
						p := p
						b.WriteString(str[tclose:topen])
						nextclose := strings.IndexByte(str[topen:], '}')
						if nextclose == -1 {
							return fmt.Errorf("Error: closing } not found in Activity [%d] string [%s] ", p.AId, str)
						}
						nextopen := strings.IndexByte(str[topen+1:], '{')
						if nextopen != -1 {
							if nextclose > nextopen {
								return fmt.Errorf("Error: closing } not found in Activity [%d] string [%s] ", p.AId, str)
							}
						}
						tclose += strings.IndexByte(str[tclose:], '}')
						tclose_ := tclose
						// examine tag to see if it references entities outside of current activity
						//   currenlty only device oven and noncurrent activity is supported
						if tdot := strings.IndexByte(str[topen+1:tclose], '.'); tdot > 0 {
							// dot notation used. Breakdown object being referenced.
							fmt.Println(str[topen+1 : tclose])
							s := strings.SplitN(strings.ToLower(str[topen+1:tclose]), ".", 2)
							el, el2 = s[0], strings.TrimSpace(s[1])
							// examples {ingrd.30}, {ingrd.label}
							_, err = strconv.Atoi(el2)
							if err == nil {
								p_ := p
								if el == "ingrd" {
									if p, ok = ActivityM[str[topen+1+tdot+1:tclose]]; !ok {
										return fmt.Errorf("Error: in loadBaseRecipe. Reference to non-existent activity in [%d]\n", p_.AId)
									}
									tclose_ -= len(str[topen+1+tdot+1:tclose]) + 1
								}
							}
						} else {
							el, el2 = strings.ToLower(str[topen+1:tclose_]), ""
						}
						switch el {
						case "device":
							s := strings.Split(el2, ".")
							if ov, ok := DeviceM[strconv.Itoa(pt.id)+"-"+s[0]]; ok {
								switch s[1] {
								case "temp":
									if len(ov.Unit) == 0 {
										return fmt.Errorf("in processBaseRecipe. No Unit defined for oven temperature for activity [%d, %d]\n", p.AId, pt.id)
									}
									fmt.Fprintf(&b, "%s", ov.String())
								case "set":
									fmt.Fprintf(&b, "%s", ov.Set)
								case "alternate":
									fmt.Fprintf(&b, "%s", ov.Alternate)
								case "purpose":
									fmt.Fprintf(&b, "%s", ov.Purpose)
								}
							} else {
								return fmt.Errorf("in processBaseRecipe. No device [%s] found for activity [%d, %d]\n", s[0], p.AId, pt.id)
							}
						case "iqual":
							fmt.Fprintf(&b, "%s", p.IngrdQualifer)
						case "quali":
							fmt.Fprintf(&b, "%s", p.QualiferIngrd)
						case "usec", "addtoc":
							var c *Container
							idx := 0
							if len(el2) == 1 { // 0,1,2,3..
								var err error
								idx, err = strconv.Atoi(el2)
								if err != nil {
									panic(fmt.Errorf(`Error: addtoC/useC not suitable in AId [%d] [%s]`, p.AId, el+el2))
								}
							}
							if el == "usec" {
								if len(pt.useCp) == 0 {
									panic(fmt.Errorf(`Error: useC not suitable in AId [%d] [%s]`, p.AId, str))
								}

								c = pt.useCp[idx]
							} else {
								if len(pt.addToCp) == 0 {
									panic(fmt.Errorf(`Error: addtoC not suitable in AId [%d] [%s]`, p.AId, str))
								}
								c = pt.addToCp[idx]
							}
							// useC.form
							if len(el2) > 1 {
								switch el2 {
								case "form": // depreciated - only alt used now. Still here to support existing
									b.WriteString(c.String())
								case "label":
									b.WriteString(c.Label)
								case "slabel":
									b.WriteString(c.Slabel)
								default:
									panic(fmt.Errorf(`Error: useC or addtoC tag not followed by "form" or "type" type in AId [%d] [%s]`, p.AId, str))
								}
							} else {
								if c.Scale {
									typ := c.Type
									m := c.Measure
									if len(c.Label) > 0 {
										typ = c.Label
									}
									fmt.Fprintf(&b, "%s", "{c:"+typ+"|"+m.Quantity+"|"+m.Size+"|"+m.Shape+"|"+m.Dimension+"|"+m.Height+"|"+m.Unit+"}")
								} else {
									b.WriteString("a " + c.String())
								}
							}
						case "altmeasure":
							context = measure
							if p.AltMeasure == nil {
								return fmt.Errorf("in loadBaseRecipe. Ingredient AltMeasure not defined for Activity [%d]\n", p.AId)
							}
							m := p.AltMeasure
							fmt.Fprintf(&b, "%s", "{m:"+m.Quantity+"|"+m.Unit+"|"+m.Size+"|"+m.Num+"}")
							//
						case "measure":
							context = measure
							// heirarchy - look for a task measure. If not present, use the activity measure.
							// is it the task measure
							if pt.Measure != nil {
								//fmt.Fprintf(&b, "%s", pt.Measure.String())
								m := pt.Measure
								fmt.Fprintf(&b, "%s", "{m:"+m.Quantity+"|"+m.Unit+"|"+m.Size+"|"+m.Num+"}")
								break
							}
							// is it the activity measure
							if p.Measure == nil {
								return fmt.Errorf("in loadBaseRecipe. Ingredient measure not defined for Activity [%d]\n", p.AId)
							}
							m := p.Measure
							fmt.Fprintf(&b, "%s", "{m:"+m.Quantity+"|"+m.Unit+"|"+m.Size+"|"+m.Num+"}")
							//
							//fmt.Fprintf(&b, "%s", p.Measure.String(formatonly))
						case "actmeasure", "ameasure", "imeasure": // ameasure, imeasure - activity/ingredient measure as opposed to measure attached to task/instruction
							// TODO: what do these mean, ameasure may mean alternative measure
							context = measure
							// is it the activity measure
							if p.Measure != nil {
								m := p.Measure
								fmt.Fprintf(&b, "%s", "{m:"+m.Quantity+"|"+m.Unit+"|"+m.Size+"|"+m.Num+"}")
								//fmt.Fprintf(&b, "%s", p.Measure.String())
							}
						case "qty":
							context = measure
							if p.Measure == nil {
								return fmt.Errorf("in loadBaseRecipe. Ingredient measure not defined for Activity [%d]\n", p.AId)
							}
							m := p.Measure
							fmt.Fprintf(&b, "%s", "{"+m.Quantity+"|||}")
							//fmt.Fprintf(&b, "%s", p.Measure.String())
						case "size":
							if p.Measure == nil {
								return fmt.Errorf("in loadBaseRecipe. Ingredient measure not defined for Activity [%d]\n", p.AId)
							}
							fmt.Fprintf(&b, "%s", p.Measure.Size)
						case "used":
							if pt.UseDevice == nil {
								return fmt.Errorf("in loadBaseRecipe. UseDevice attribute not defined for Activity [%d]\n", p.AId)
							}
							fmt.Fprintf(&b, "%s", strings.ToLower(pt.UseDevice.Type))
							context = device
						case "alternate", "devicealt":
							if pt.UseDevice == nil {
								return fmt.Errorf("in loadBaseRecipe. UseDevice attribute not defined for Activity [%d]\n", p.AId)
							}
							fmt.Fprintf(&b, "%s", strings.ToLower(pt.UseDevice.Alternate))
							context = device
						case "qualm": // this maybe redundant. Measure.String() should incorporate qualm.
							fmt.Fprintf(&b, "%s", strings.ToLower(p.QualMeasure))
						case "temp":
							if pt.UseDevice == nil {
								return fmt.Errorf("in loadBaseRecipe. UseDevice attribute not defined for Activity [%d]\n", p.AId)
							}
							context = device
							//if len(pt.UseDevice.Set) > 0 {
							fmt.Fprintf(&b, "{T:%s|%s|%s}", pt.UseDevice.Unit, pt.UseDevice.Temp, pt.UseDevice.Set)
							// } else {
							// 	fmt.Fprintf(&b, "{T:%s|%s|}", pt.UseDevice.Unit, pt.UseDevice.Temp)
							// }
							//fmt.Fprintf(&b, "%s", pt.UseDevice.String())
						case "label", "alabel":
							fmt.Fprintf(&b, "%s", p.Label)
						case "tlabel":
							fmt.Fprintf(&b, "%s", pt.Label)
						case "unit":
							var (
								u      *Unit
								plural bool
							)
							switch context {
							case measure:
								if p.Measure == nil {
									return fmt.Errorf("in loadBaseRecipe. Measure not defined for Activity [%d]\n", p.AId)
								}
								m := p.Measure
								if !(strings.IndexByte(m.Quantity, '/') > 0 || strings.IndexByte(m.Quantity, '.') > 0 || m.Quantity == "1") {
									plural = true
								}
								if len(p.Measure.Unit) == 0 {
									return fmt.Errorf("in loadBaseRecipe. Unit for time, [%s], not defined for Measure in Activity [%d]\n", pt.Unit, p.AId)
								}
								if u, ok = UnitMap[p.Measure.Unit]; !ok {
									return fmt.Errorf("in loadBaseRecipe. Unit for measure, [%s], not defined in unitM for Activity [%d]\n", p.Measure.Unit, p.AId)
								}
							case time:
								if pt.Time > 0 {
									plural = true
								}
								if len(pt.Unit) > 0 && len(pt.Unit) == 1 {
									return fmt.Errorf("in loadBaseRecipe. Unit for time, [%s], not defined for Activity [%d]\n", pt.Unit, p.AId)
								}
								if len(pt.Unit) > 0 {
									if u, ok = UnitMap[pt.Unit]; !ok {
										return fmt.Errorf("in loadBaseRecipe. Unit for time, [%s], not defined in unitM for Activity [%d]\n", pt.Unit, p.AId)
									}
								}
							}
							if context == device {
								// ignore device as unit now printed with temp tag.
								break
							}
							if plural && interactionType == voice && u.Say == "l" {
								fmt.Fprintf(&b, "%s", u.String()+"s")
							} else {
								fmt.Fprintf(&b, "%s", u.String())
							}
						case "ingrd":
							if el2 == "label" {
								if len(p.Label) > 0 {
									fmt.Fprintf(&b, "%s", strings.ToLower(p.Label))
									break
								}
							}
							fmt.Fprintf(&b, "%s", strings.ToLower(p.Ingredient))
						case "altingrd":
							fmt.Fprintf(&b, "%s", strings.ToLower(p.AltIngrd))
						case "ingrd_": // make singular if plural
							if strings.ToLower(p.Ingredient[:len(p.Ingredient)-1]) == "s" {
								fmt.Fprintf(&b, "%s", strings.ToLower(p.Ingredient[:len(p.Ingredient)-1]))
							} else {
								fmt.Fprintf(&b, "%s", strings.ToLower(p.Ingredient))
							}
						case "timeu":
							fmt.Fprintf(&b, "%d", pt.Time)
							if pt.Tplus > 0 {
								context = time
								fmt.Fprintf(&b, " to %d ", pt.Tplus+pt.Time)
							}
							fmt.Fprintf(&b, "%s", UnitMap[pt.Unit].String(pt))
						case "time":
							context = time
							fmt.Fprintf(&b, "%d", pt.Time)

						case "tplus":
							{
								context = time
								fmt.Fprintf(&b, "%d", pt.Tplus+pt.Time)
							}
						default:
							// pass literal measures straight through
							b.WriteString(strings.ToLower(str[topen : tclose_+1]))
						}
						tclose += 1
						topen = strings.IndexByte(str[tclose:], '{')
						if topen == -1 {
							b.WriteString(str[tclose:])
						} else {
							topen += tclose
						}
					}
					if topen == -1 && strings.IndexByte(str[tclose:], '}') != -1 {
						return fmt.Errorf("Error: closing } found with no open { in Activity [%d] string [%s] ", p.AId, str)
					}
					switch interactionType {
					case text:
						pt.text = doubleSpace.Replace(b.String())
						b.Reset()
					case voice:
						pt.verbal = doubleSpace.Replace(b.String())
						b.Reset()
					}
				}
			}
		}
	}
	//
	//  Generate and save metadata from base Activities to Dyanmodb
	//
	ptS, err := ActivityS.generateAndSaveTasks(s)
	if err != nil {
		return fmt.Errorf("Error in generateAndSaveTasks in loadBaseRecipe - %s", err.Error())
	}
	err = s.generateAndSaveIndex(LabelM, IngredientM)
	if err != nil {
		return fmt.Errorf("Error in generateAndSaveIndex in loadBaseRecipe  - %s", err.Error())
	}
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
	// for _, v := range ContainerM {
	// 	var found bool
	// 	for i := len(ptS) - 1; i >= 0; i-- {
	// 		// find last appearance (typically sourceC). Start at last ptS and work backwards
	// 		for _, c := range ptS[i].taskp.allCp {
	// 			if c == v {
	// 				if ptS[i].SortK > c.last {
	// 					c.last = ptS[i].SortK
	// 					found = true
	// 					break
	// 				}
	// 			}
	// 		}
	// 		if found {
	// 			break
	// 		}
	// 	}
	// }
	// //
	// err = ContainerM.saveContainerUsage(s)
	// if err != nil {
	// 	return fmt.Errorf("Error in readBaseRecipe after saveContainerUsage - %s", err.Error())
	// }
	// DevicesM := make(DevicesMap)
	// for p := activityStart; p != nil; p = p.next {
	// 	for _, pp := range p.Prep {
	// 		if pp.UseDevice != nil {
	// 			typ := strings.ToLower(pp.UseDevice.Type)
	// 			if _, ok := DevicesM[typ]; !ok {
	// 				var str string

	// 				pp := pp.UseDevice
	// 				str = pp.String()
	// 				if len(pp.Purpose) > 0 {
	// 					str += pp.Purpose
	// 				}
	// 				if len(pp.Alternate) > 0 {
	// 					str += " Alternative: " + pp.Alternate
	// 				}
	// 				DevicesM[typ] = str
	// 			}
	// 		}
	// 	}
	// 	for _, pp := range p.Task {
	// 		if pp.UseDevice != nil {
	// 			typ := strings.ToLower(pp.UseDevice.Type)
	// 			if _, ok := DevicesM[typ]; !ok {
	// 				var str string
	// 				pp := pp.UseDevice
	// 				str = pp.String()
	// 				if len(pp.Purpose) > 0 {
	// 					str += pp.Purpose
	// 				}
	// 				if len(pp.Alternate) > 0 {
	// 					str += "Alternative: " + pp.Alternate
	// 				}
	// 				DevicesM[typ] = str
	// 			}
	// 		}
	// 	}
	// }
	// err = DevicesM.saveDevices(s)
	// if err != nil {
	// 	return fmt.Errorf("Error in readBaseRecipe after saveDevice - %s", err.Error())
	// }

	return nil

} //
