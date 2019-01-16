package main

import (
	_ "encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	_ "github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/dynamodb/expression"

	_ "github.com/aws/aws-lambda-go/lambdacontext"
)

var errExceedEOL error = errors.New("Exceed EOL")
var errFailValidation error = errors.New("Failed Session Validation")

//TODO
// change float32 to float64 as this is what dynamoAttribute.Unmarshal uses
type PrepTask int

const (
	prep PrepTask = iota
	task
)
const (
	EOT int = iota
	NOTEOT
)
const jsonKey string = "task"

func (p PrepTask) string() string {
	switch p {
	case prep:
		return "Prep"
	case task:
		return "Task"
	}
	return "Error"
}

type respT struct {
	Error error
	msg   string
}

type Unit struct {
	Unit   string `json:"unit"`
	Slabel string `json:"slabel"` // short label
	Llabel string `json:"llabel"` // long label
	Desc   string `json:"desc"`
}

type MeasureCT struct {
	Quantity  string `json:"qty"`
	Size      string `json:"size"`
	Shape     string `json:"shape"`
	Dimension string `json:"dim"`
	//	Diameter string `json:"diameter"`
	//	Square   string `json:"square"`
	//	Rect     string `json:"rect"`
	Height string `json:"height"`
	Unit   string `json:"unit`
}

type taskT struct {
	Type      PrepTask // Prep or Task Activity
	Idx       int      // slice index
	Activityp *Activity
}

type Container struct {
	// Rid      string     `json:"PKey"`
	Cid        string     `json:"SortK"`
	Label      string     `json:"label"`
	Type       string     `json:"type"`
	Purpose    string     `json:"purpose"`
	Coord      [2]float32 `json:"coord"`
	Measure    *MeasureCT `json:"measure"`
	AltMeasure *MeasureCT `json:"altMeasure"`
	Contains   string     `json:"contents"`
	Message    string     `json:"message"`
	start      int        // first id in recipe tasks where container is used
	last       int        // last id in recipe tasks where container is sourced from or recipe is complete.
	reused     int        // links containers that can be the same physical container.
	Activity   []taskT    // slice of tasks (Prep and Task activites) associated with container
}

type DeviceT struct {
	Type      string `json:"type"`
	Set       string `json:"set"`
	Purpose   string `json:"purpose"`
	Alternate string `json:"alternate"`
	Temp      string `json:"temp"`
	Unit      string `json:"unit"`
}

type PerformT struct {
	//	type      PrepTask q // Prep or Task Activity
	id          int      // order as appears in Activity JSON
	Text        string   `json:"txt"` // original from db - contains {tag}
	text        string   // has {tag} replaced
	Verbal      string   `json:"say"` // original from db - contains {tag}
	verbal      string   // has {tag} replaced
	Label       string   `json:"label"`
	IngredientS []string `json:"ingrd"` // case where ingredient prepping produces other ingredients e.g. separating eggs
	Time        float32  `json:"time"`
	Tplus       float32  `json:"tPlus"`
	Unit        string   `json:"unit"`
	UseDevice   *DeviceT `json:"useD"`
	WaitOn      int      `json:"waitOn"` // depenency on other activity to complete
	//DeviceT
	AddToC   []string     `json:"addToC"`
	UseC     []string     `json:"useC"`
	SourceC  []string     `json:"sourceC"`
	Parallel bool         `json:"parallel"`
	Link     bool         `json:"link"`
	AddToCp  []*Container // it is thought that only one addToC will be used per activity - but lets be flexible.
	UseCp    []*Container // ---"---
	SourceCp []*Container // ---"---
	AllCp    []*Container // all containers (addTo, use, source) get saved here.
}

type MeasureT struct {
	type_     int    // 0 - normal qty being vol/weigth, 1 - qty being number of, 2 - qty being weight each, 3 - qty being vol each
	Quantity  string `json:"qty"`
	VerbalQty string `json:"vQty"`
	Weight    string `json:"wgt"`
	Volume    string `json:"vol"`
	Size      string `json:"size"`
	Unit      string `json:"unit"`
	Comment   string `json:"comment"`
}

// used for alternative ingredients only
type IngredientT struct {
	Name          string
	IngrdQualifer string `json:"iQual"` // (append) to ingredient
	QualiferIngrd string `json:"quali"` // prepend  to ingredient.
	Type          string `json:"iType"`
	Measure       *MeasureT
}

type Activity struct {
	// Pkey          string     `json:"PKey"`
	AId           int         `json:"SortK"`
	Label         string      `json:"label"`    // used in container listing rather than using ingredient
	Ingredient    string      `json:"ingrd"`    //
	IngrdQualifer string      `json:"iQual"`    // (append) to ingredient
	QualiferIngrd string      `json:"quali"`    // prepend  to ingredient.
	QualMeasure   string      `json:"qualm"`    // qualifer before measure in ingredients output e.g. <qualm> of <measure> a <ingrd>
	AltIngrd      []string    `json:"altIngrd"` // key into Ingredient table - used for alternate ingredients only
	Measure       *MeasureT   `json:"measure"`
	AltMeasure    *MeasureT   `json:"altMeasure"`
	Overview      string      `json:"ovv"`
	Coord         [2]float32  // X,Y
	Task          []*PerformT `json:"task"`
	Prep          []*PerformT `json:"prep"`
	next          *Activity
	prev          *Activity
	nextTask      *Activity
	nextPrep      *Activity
}

type ContainerMap map[string]*Container

var ContainerM ContainerMap

type DevicesMap map[string]string
type DeviceMap map[string]DeviceT

var activityStart *Activity

type Activities []Activity

// links all activities with Tasks
type taskCtl struct {
	start *Activity // ptr to first task
	cnt   int       // task count
}

var taskctl taskCtl = taskCtl{}

// links all Prep activities
type prepCtl struct {
	start *Activity // ptr to first task
	cnt   int       // task count
}

var prepctl prepCtl = prepCtl{}

func (d *DeviceT) String() (string, error) {
	var s string
	if len(d.Temp) > 0 {
		if len(d.Unit) == 0 {
			return "", fmt.Errorf("No Unit specified for device [%s] with Temp in Activity", d.Type)
		}
		t := strings.Split(d.Temp, "/")
		if len(t) > 0 {
			// for an oven device, a/b means <a><unit> fan/ <b><unit> nofan / setting
			s = t[0] + "\u00B0" + d.Unit + "/" + t[1] + "\u00B0" + d.Unit + " Fan"
		} else {
			s = d.Temp + "\u00B0" + d.Unit
		}
	}
	if len(d.Set) > 0 {
		if len(s) > 0 {
			s += "/" + d.Set
		} else {
			s = d.Set
		}
	}
	return s, nil
}

func (m *MeasureCT) String() string {
	var s string
	if len(m.Shape) > 0 {
		s = m.Shape + " "
	}
	if len(m.Dimension) > 0 {
		s += m.Dimension
	}
	if len(m.Height) > 0 {
		s += "x" + m.Height
	}
	if len(m.Unit) > 0 {
		s += m.Unit
	}
	return s
}

func (s *sessCtx) processBaseRecipe() error {
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
		return fmt.Errorf("Error: in readBaseRecipeForContainers Query - %s", err.Error())
	}
	if int(*result.Count) == 0 {
		return fmt.Errorf("No data found for reqRId %s in processBaseRecipe for Activity - ", s.pkey)
	}
	//ActivityS := make([]Activity, int(*result.Count))
	ActivityS := make(Activities, int(*result.Count))
	err = dynamodbattribute.UnmarshalListOfMaps(result.Items, &ActivityS)
	if err != nil {
		return fmt.Errorf("** Error during UnmarshalListOfMaps in processBaseRecipe - %s", err.Error())
	}
	//
	// Create maps based on AId, Ingredient (plural and singular) and Label (plural and singular)
	//
	idx := 0
	//TODO does id get used??
	for _, p := range ActivityS {
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
	activityStart = &ActivityS[0]
	ActivityM := make(map[string]*Activity)
	IngredientM := make(map[string]*Activity)
	LabelM := make(map[string]*Activity)
	for i, v := range ActivityS {
		aid := strconv.Itoa(v.AId)
		ActivityM[aid] = &ActivityS[i]
		if len(v.Ingredient) > 0 {
			ingrd := strings.ToLower(v.Ingredient)
			IngredientM[ingrd] = &ActivityS[i]
			if ingrd[len(ingrd)-1] == 's' {
				// make singular entry as well
				IngredientM[ingrd[:len(ingrd)-1]] = &ActivityS[i]
			}
		}
		if len(v.Label) > 0 {
			label := strings.ToLower(v.Label)
			LabelM[label] = &ActivityS[i]
			if label[len(label)-1] == 's' {
				// make singular entry as well
				LabelM[label[:len(label)-1]] = &ActivityS[i]
			}
		}
	}
	// link activities together via next, prev, nextTask, nextPrep pointers. Order in ActivityS is sorted from dynamodb sort key.
	// not sure how useful have next, prev pointers will be but its easy to setup so keep for time being. Do use prev in other part of code.
	for i := 0; i < len(ActivityS)-1; i++ {
		ActivityS[i].next = &ActivityS[i+1]
		if i > 0 {
			ActivityS[i].prev = &ActivityS[i-1]
		}
	}
	//
	// link Task Activities - taskctl is a package variable.
	//
	var j int
	for i, v := range ActivityS {
		if v.Task != nil {
			taskctl.start = &ActivityS[i]
			j = i
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
	}
	//
	// link Prep Activities - prepctl is a package variable.
	//
	for i, v := range ActivityS {
		if v.Prep != nil {
			prepctl.start = &ActivityS[i]
			j = i
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
	// Table:  Unit
	//
	proj := expression.NamesList(expression.Name("slabel"), expression.Name("llabel"), expression.Name("desc"))
	expr, err = expression.NewBuilder().WithProjection(proj).Build()
	if err != nil {
		return fmt.Errorf("%s", "Error in expression build of unit table: "+err.Error())
	}
	// Build the query input parameters
	params := &dynamodb.ScanInput{
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		ProjectionExpression:      expr.Projection(),
		TableName:                 aws.String("Unit"),
	}
	resultS, err := s.dynamodbSvc.Scan(params)
	if err != nil {
		return fmt.Errorf("%s", "Error in scan of unit table: "+err.Error())
	}
	unitM := make(map[string]*Unit, int(*result.Count))
	unit := make([]*Unit, int(*result.Count))
	err = dynamodbattribute.UnmarshalListOfMaps(resultS.Items, &unit)
	if err != nil {
		return fmt.Errorf("%s", "Error in UnmarshalMap of container table: "+err.Error())
	}
	for _, v := range unit {
		unitM[v.Slabel] = v
	}
	// for k, v := range unitM {
	// 	fmt.Printf("%s - %#v\n", k, v)
	// }
	unit = nil
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
							// purpose of SAM is defined after a "."
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
							switch len(sac) {
							case 1, 2:
								c.Contains = ap.Ingredient
							default:
								c.Contains = sac[2] // prefer to use label as its bit more informative for container listing.
							}
							c.Measure = cId.Measure
							c.Label = cId.Label
							c.Type = cId.Type
							switch len(sac) {
							case 1:
								c.Purpose = cId.Purpose
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
						p.AddToCp = append(p.AddToCp, cId)
						p.AllCp = append(p.AllCp, cId)
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
							c.Cid = p.UseC[i] + "-" + strconv.Itoa(ap.AId)
							switch len(sac) {
							case 0, 1:
								c.Contains = ap.Ingredient
							default:
								c.Contains = sac[2] // prefer to use label as its bit more informative for container listing.
							}
							c.Measure = cId.Measure
							c.Label = cId.Label
							c.Type = cId.Type
							switch len(sac) {
							case 1, 2:
								c.Purpose = cId.Purpose
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
						p.UseCp = append(p.UseCp, cId)
						p.AllCp = append(p.AllCp, cId)
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
							switch len(sac) {
							case 1, 2:
								c.Contains = ap.Ingredient
							default:
								c.Contains = sac[2] // prefer to use label as its bit more informative for container listing.
							}
							c.Measure = cId.Measure
							c.Label = cId.Label
							c.Type = cId.Type
							switch len(sac) {
							case 1:
								c.Purpose = cId.Purpose
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
						p.SourceCp = append(p.SourceCp, cId)
						p.AllCp = append(p.AllCp, cId)
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
	)
	//
	//  replace all {tag} in text and verbal for each activity. Ignore Link'd activites - they are only relevant at print time
	//
	var pt []*PerformT
	for _, taskType := range []PrepTask{prep, task} {
		for _, interactionType := range []int{text, voice} {
			for p := activityStart; p != nil; p = p.next {
				switch taskType {
				case prep:
					pt = p.Prep
				case task:
					pt = p.Task
				}
				for _, pt := range pt {
					// perform over slice of preps, tasks
					switch interactionType {
					case text:
						str = strings.TrimLeft(pt.Text, " ")
						s := str[0]
						str = strings.ToUpper(string(s)) + str[1:]
					case voice:
						str = pt.Verbal
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
							s := strings.SplitN(strings.ToLower(str[topen+1:tclose]), ".", 2)
							el, el2 = s[0], s[1]
							if el == "ingrd" {
								// reference to attribute in noncurrent activity e.g. {ingrd.30}
								p = ActivityM[str[topen+1+tdot+1:tclose]]
								tclose_ -= len(str[topen+1+tdot+1:tclose]) + 1
								//el = str[topen+1 : tclose_]
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
									fmt.Fprintf(&b, "%s", ov.Temp+" "+ov.Unit)
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
							if el == "usec" {
								c = pt.UseCp[0]
							} else {
								c = pt.AddToCp[0]
							}
							var m *MeasureCT
							// useC.form
							if len(el2) > 0 {
								switch el2 {
								case "form":
									m = c.Measure
								case "alt":
									m = c.AltMeasure
								default:
									return fmt.Errorf(`Error: useC or addtoC tag not followed by "form" or "alt" type in AId [%d] [%s]`, p.AId, str)
								}
								if m != nil {
									s := m.String()
									fmt.Fprintf(&b, "%s", strings.ToLower(s)+" "+c.Label)
								}
							} else {
								m = c.Measure
								if m != nil {
									if len(m.Size) == 0 {
										fmt.Fprintf(&b, "%s", strings.ToLower("medium "+c.Label))
									} else {
										fmt.Fprintf(&b, "%s", strings.ToLower(m.Size+" "+c.Label))
									}
								} else {
									fmt.Fprintf(&b, "%s", strings.ToLower("medium "+c.Label))
								}
							}
						case "qty":
							context = measure
							if p.Measure == nil {
								return fmt.Errorf("in processBaseRecipe. Ingredient measure not defined for Activity [%d]\n", p.AId)
							}
							fmt.Fprintf(&b, "%s", p.Measure.Quantity)
						case "wgt":
							context = measure
							if p.Measure == nil {
								return fmt.Errorf("in processBaseRecipe. Ingredient measure not defined for Activity [%d]\n", p.AId)
							}
							fmt.Fprintf(&b, "%s", p.Measure.Weight)
						case "vol":
							context = measure
							if p.Measure == nil {
								return fmt.Errorf("in processBaseRecipe. Ingredient measure not defined for Activity [%d]\n", p.AId)
							}
							fmt.Fprintf(&b, "%s", p.Measure.Volume)
						case "size":
							if p.Measure == nil {
								return fmt.Errorf("in processBaseRecipe. Ingredient measure not defined for Activity [%d]\n", p.AId)
							}
							fmt.Fprintf(&b, "%s", p.Measure.Size)
						case "used":
							if pt.UseDevice == nil {
								return fmt.Errorf("in processBaseRecipe. UseDevice attribute not defined for Activity [%d]\n", p.AId)
							}
							fmt.Fprintf(&b, "%s", strings.ToLower(pt.UseDevice.Type))
							context = device
						case "alternate", "devicealt":
							if pt.UseDevice == nil {
								return fmt.Errorf("in processBaseRecipe. UseDevice attribute not defined for Activity [%d]\n", p.AId)
							}
							fmt.Fprintf(&b, "%s", strings.ToLower(pt.UseDevice.Alternate))
							context = device
						case "qualm":
							fmt.Fprintf(&b, "%s", strings.ToLower(p.QualMeasure))
						case "temp":
							if pt.UseDevice == nil {
								return fmt.Errorf("in processBaseRecipe. UseDevice attribute not defined for Activity [%d]\n", p.AId)
							}
							context = device
							s, err := pt.UseDevice.String()
							if err != nil {
								return err
							}
							fmt.Fprintf(&b, "%s", s)
						case "label":
							fmt.Fprintf(&b, "%s", pt.Label)
						case "unit":
							{
								switch context {
								case device:
									if pt.UseDevice == nil {
										return fmt.Errorf("in processBaseRecipe. UseDevice attribute not defined for Activity [%d]\n", p.AId)
									}
									if u, ok := unitM[pt.UseDevice.Unit]; !ok {
										return fmt.Errorf("in processBaseRecipe. Unit for device, [%s], not defined in unitM for Activity [%d]\n", p.Measure.Unit, p.AId)
									} else {
										switch interactionType {
										case text:
											fmt.Fprintf(&b, "%s", u.Slabel)
										case voice:
											fmt.Fprintf(&b, "%s", u.Llabel+"s")
										}
									}
								case measure:
									if p.Measure == nil {
										return fmt.Errorf("in processBaseRecipe. Measure not defined for Activity [%d]\n", p.AId)
									}
									if u, ok := unitM[p.Measure.Unit]; !ok {
										return fmt.Errorf("in processBaseRecipe. Unit for measure, [%s], not defined in unitM for Activity [%d]\n", p.Measure.Unit, p.AId)
									} else {
										switch interactionType {
										case text:
											fmt.Fprintf(&b, "%s", u.Slabel)
										case voice:
											fmt.Fprintf(&b, "%s", u.Llabel+"s")
										}
									}
								case time:
									if u, ok := unitM[pt.Unit]; !ok {
										return fmt.Errorf("in processBaseRecipe. Unit for time, [%s], not defined in unitM for Activity [%d]\n", pt.Unit, p.AId)
									} else {
										switch interactionType {
										case text:
											fmt.Fprintf(&b, "%s", u.Llabel+"s")
										case voice:
											fmt.Fprintf(&b, "%s", u.Llabel+"s")
										}
									}
								}
							}
						case "ingrd":
							fmt.Fprintf(&b, "%s", strings.ToLower(p.Ingredient))
						case "time":
							{
								context = time
								fmt.Fprintf(&b, "%2.0f", pt.Time)
							}
						case "tplus":
							{
								context = time
								fmt.Fprintf(&b, "%2.0f", pt.Tplus+pt.Time)
							}
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
		return fmt.Errorf("Error in generateAndSaveTasks in processBaseRecipe - %s", err.Error())
	}
	err = s.generateAndSaveIndex(LabelM, IngredientM)
	if err != nil {
		return fmt.Errorf("Error in generateAndSaveIndex in processBaseRecipe  - %s", err.Error())
	}
	//
	// Post processing of Containers
	//
	// find first reference to Container in the ordered instruction (ptS) list
	for _, v := range ContainerM {
		var found bool
		v.start = 99999
		for _, pt := range ptS {
			for _, c := range pt.taskp.AllCp {
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
			for _, c := range ptS[i].taskp.AllCp {
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
	err = ContainerM.saveContainerUsage(s)
	if err != nil {
		return fmt.Errorf("Error in readBaseRecipe after saveContainerUsage - %s", err.Error())
	}
	DevicesM := make(DevicesMap)
	for p := activityStart; p != nil; p = p.next {
		for _, pp := range p.Prep {
			if pp.UseDevice != nil {
				typ := strings.ToLower(pp.UseDevice.Type)
				if _, ok := DevicesM[typ]; !ok {
					var str string
					pp := pp.UseDevice
					if len(pp.Set) > 0 {
						str = "Set to " + pp.Set + ". "
					}
					if len(pp.Temp) > 0 {
						str = "Set to " + pp.Temp + " " + pp.Unit + ". "
					}
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
		for _, pp := range p.Task {
			if pp.UseDevice != nil {
				typ := strings.ToLower(pp.UseDevice.Type)
				if _, ok := DevicesM[typ]; !ok {
					var str string
					pp := pp.UseDevice
					if len(pp.Set) > 0 {
						str = "Set to " + pp.Set + ". "
					}
					if len(pp.Temp) > 0 {
						str = "Set to " + pp.Temp + " " + pp.Unit + ". "
					}
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
	err = DevicesM.saveDevices(s)
	if err != nil {
		return fmt.Errorf("Error in readBaseRecipe after saveDevice - %s", err.Error())
	}

	return nil

} //
