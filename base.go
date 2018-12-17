package main

import (
	_ "encoding/json"
	"errors"
	"fmt"
	"log"
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
const (
	prep PrepTask = iota
	task
)
const (
	EOT int = iota
	NOTEOT
)
const jsonKey string = "task"

type PrepTask int

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
	Quantity string `json:"qty"`
	Size     string
	Diameter float32
	Height   float32
	Unit     string
}

type taskT struct {
	Type      PrepTask // Prep or Task Activity
	Activityp *Activity
}

type Container struct {
	Rid      string     `json:"rId"`
	Cid      string     `json:"cId"`
	Label    string     `json:"label"`
	Type     string     `json:"type"`
	Purpose  string     `json:"purpose"`
	Coord    [2]float32 `json:"coord"`
	Measure  MeasureCT  `json:"measure"`
	Contains string     `json:"contains"`
	Message  string     `json:"message"`
	Task     []taskT    // slice of tasks (Prep and Task activites) associated with container
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
	Text      string       `json:"txt"` // original from db - contains {tag}
	text      string       // has {tag} replaced
	Verbal    string       `json:"say"` // original from db - contains {tag}
	verbal    string       // has {tag} replaced
	Time      float32      `json:"time"`
	Tplus     float32      `json:"tPlus"`
	Unit      string       `json:"unit"`
	UseDevice *DeviceT     `json:"useD"`
	AddToC    []string     `json:"addToC"`
	UseC      []string     `json:"useC"`
	SourceC   []string     `json:"sourceC"`
	Parallel  bool         `json:"parallel"`
	Link      bool         `json:"link"`
	AddToCp   []*Container // it is thought that only one addToC will be used per activity - but lets be flexible.
	UseCp     []*Container
	SourceCp  []*Container
}

type MeasureT struct {
	Quantity  string `json:"qty"`
	VerbalQty string `json:"verbalQty"`
	Size      string `json:"size"`
	Unit      string `json:"unit"`
}

// used for alternative ingredients only
type IngredientT struct {
	Name          string
	IngrdQualifer string `json:"iQual"` // (append) to ingredient
	QualiferIngrd string `json:"quali"` // prepend  to ingredient.
	Type          string `json:"iType"`
	Measure       MeasureT
}

type Activity struct {
	RId           string     `json:"rId"`
	AId           int        `json:"aId"`
	Label         string     `json:"label"`
	Ingredient    string     `json:"ingrd"`    //
	IngrdQualifer string     `json:"iQual"`    // (append) to ingredient
	QualiferIngrd string     `json:"quali"`    // prepend  to ingredient.
	AltIngrd      []string   `json:"altIngrd"` // key into Ingredient table - used for alternate ingredients only
	IngrdType     string     `json:"iType"`
	Measure       MeasureT   `json:"measure"`
	Overview      string     `json:"ovv"`
	Coord         [2]float32 // X,Y
	Task          *PerformT  `json:"task"`
	Prep          *PerformT  `json:"prep"`
	next          *Activity
	prev          *Activity
	nextTask      *Activity
	nextPrep      *Activity
}

type ContainerMap map[string]*Container

var ContainerM ContainerMap

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


func readBaseRecipeForContainers(svc *dynamodb.DynamoDB, reqRId_ string) (ContainerMap, error) {
	// var svc *dynamodb.DynamoDB
	// var reqRId_ string
	//
	// Table:  Activity
	//
	kcond := expression.KeyEqual(expression.Key("rId"), expression.Value(reqRId_))
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
	input = input.SetTableName("Activity").SetReturnConsumedCapacity("TOTAL").SetConsistentRead(false)
	//*dynamodb.DynamoDB,
	result, err := svc.Query(input)
	if err != nil {
		log.Print("error from Query:  " + err.Error())
		//return nil//, err
	}
	if int(*result.Count) == 0 {
		fmt.Println("No activity data for reqRId " + reqRId_)
		//return nil //, fmt.Errorf("No data found for reqRId %s", reqRId_)
	}
	ActivityS := make([]Activity, int(*result.Count))
	err = dynamodbattribute.UnmarshalListOfMaps(result.Items, &ActivityS)
	//
	activityStart = &ActivityS[0]
	// link activities together via next, prev, nextTask, nextPrep pointers. Order in ActivityS is sorted from dynamodb sort key.
	// not sure how useful have next, prev pointers will be but its easy to setup so keep for time being. Do use prev in other part of code.
	for i := 0; i < len(ActivityS)-1; i++ {
		ActivityS[i].next = &ActivityS[i+1]
		if i > 0 {
			ActivityS[i].prev = &ActivityS[i-1]
		}
	}
	//
	// Table:  Container
	//
	kcond = expression.KeyEqual(expression.Key("rId"), expression.Value(reqRId_))
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
	input = input.SetTableName("Container").SetReturnConsumedCapacity("TOTAL").SetConsistentRead(false)
	//
	result, err = svc.Query(input)
	if err != nil {
		fmt.Println()
		//return nil //, fmt.Errorf("%s", "Error in Query of container table: "+err.Error())
	}
	if int(*result.Count) == 0 {
		fmt.Println("No container data..")
	}
	// Container lookup - given Cid give me pointer to the continer.
	ContainerM = make(ContainerMap, int(*result.Count))
	var itemc *Container
	for _, i := range result.Items {
		itemc = new(Container)
		err = dynamodbattribute.UnmarshalMap(i, itemc)
		if err != nil {
			fmt.Println("Got error unmarshalling:")
			fmt.Println(err.Error())
			//return nil //, fmt.Errorf("%s", "Error in UnmarshalMap of container table: "+err.Error())
		}
		ContainerM[itemc.Cid] = itemc
	}
	//
	// Table:  Unit
	//
	proj := expression.NamesList(expression.Name("slabel"), expression.Name("llabel"), expression.Name("desc"))
	expr, err = expression.NewBuilder().WithProjection(proj).Build()
	if err != nil {
		fmt.Println()
		//return nil, fmt.Errorf("%s", "Error in expression build of unit table: "+err.Error())
	}
	// Build the query input parameters
	params := &dynamodb.ScanInput{
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		ProjectionExpression:      expr.Projection(),
		TableName:                 aws.String("Unit"),
	}
	resultS, err := svc.Scan(params)
	if err != nil {
		fmt.Println()
		//return nil //, fmt.Errorf("%s", "Error in scan of unit table: "+err.Error())
	}
	if int(*result.Count) == 0 {
		fmt.Println()
		//return nil //, fmt.Errorf("%s", "no-data-found in unit table: "+err.Error())
	}
	unitM := make(map[string]*Unit, int(*result.Count))
	var unit *Unit
	for _, i := range resultS.Items {
		unit = new(Unit)
		err = dynamodbattribute.UnmarshalMap(i, unit)
		if err != nil {
			fmt.Println("Got error unmarshalling:")
			fmt.Println(err.Error())
			//return nil, fmt.Errorf("%s", "Error in UnmarshalMap of unit table: "+err.Error())
		}
		unitM[unit.Slabel] = unit
	}
	//
	//  Post fetch processing - assign container pointers in Activity and validate that all containers referenced exist
	//
	// for all prep, tasks
	//
	for _, l := range []PrepTask{prep, task} {
		for i, ap := 0, activityStart; ap != nil; ap = ap.next {
			var p *PerformT
			switch l {
			case task:
				p = ap.Task
			case prep:
				p = ap.Prep
			}
			if p == nil {
				continue
			}
			if len(p.AddToC) > 0 {
				for i := 0; i < len(p.AddToC); i++ {
					if cId, ok := ContainerM[p.AddToC[i]]; !ok {
						if cId, ok = ContainerM[strings.TrimSpace(p.AddToC[i])]; !ok {
							fmt.Printf("Error:   Container [%s] not found for %s %d\n", strings.TrimSpace(p.AddToC[i]), ap.Label, ap.AId)
							continue
						}
					} else {
						// activity to container edge
						p.AddToCp = append(p.AddToCp, cId)
						// container to activity edge
						associatedTask := taskT{Type: l, Activityp: ap}
						cId.Task = append(cId.Task, associatedTask)
					}
				}
			}
			if len(p.UseC) > 0 {
				for i := 0; i < len(p.UseC); i++ {
					if cId, ok := ContainerM[p.UseC[i]]; !ok {
						if cId, ok = ContainerM[strings.TrimSpace(p.UseC[i])]; !ok {
							fmt.Printf("Error:   Container [%s] not found for %s %d\n", strings.TrimSpace(p.UseC[i]), ap.Label, ap.AId)
							continue
						}
					} else {
						p.UseCp = append(p.UseCp, cId)
						//cId.Activityp = append(cId.Activityp, t)
						associatedTask := taskT{Type: l, Activityp: ap}
						cId.Task = append(cId.Task, associatedTask)
					}
				}
			}
			if len(p.SourceC) > 0 {
				for i := 0; i < len(p.SourceC); i++ {
					if cId, ok := ContainerM[p.SourceC[i]]; !ok {
						if cId, ok = ContainerM[strings.TrimSpace(p.SourceC[i])]; !ok {
							fmt.Printf("Error:   Container [%s] not found for %s %d\n", strings.TrimSpace(p.SourceC[i]), ap.Label, ap.AId)
							continue
						}
					} else {
						p.SourceCp = append(p.SourceCp, cId)
						associatedTask := taskT{Type: l, Activityp: ap}
						cId.Task = append(cId.Task, associatedTask)
					}
				}
			}
			i++
		}
	}
	// check container is associated with an activity. if not delete from container map.
	for _, c := range ContainerM {
		if len(c.Task) == 0 {
			delete(ContainerM, c.Cid)
		}
	}

	devices := make(map[string]struct{})
	for p := activityStart; p != nil; p = p.next {
		if p.Task != nil {
			if p.Task.UseDevice != nil {
				if _, ok := devices[strings.Title(p.Task.UseDevice.Type)]; !ok {
					devices[strings.Title(p.Task.UseDevice.Type)] = struct{}{}
				}
			}
		}
		if p.Prep != nil && p.Prep.UseDevice != nil {
			if _, ok := devices[strings.Title(p.Prep.UseDevice.Type)]; !ok {
				devices[strings.Title(p.Prep.UseDevice.Type)] = struct{}{}
			}
		}
	}
	return ContainerM, nil
}

func readBaseRecipeForTasks(svc *dynamodb.DynamoDB, reqRId_ string) (Activities, error) {
	//
	// Table:  Activity
	//
	kcond := expression.KeyEqual(expression.Key("rId"), expression.Value(reqRId_))
	//kcond := expression.KeyAnd(expression.KeyEqual(expression.Key("rId"), expression.Value("XYZ")), expression.KeyLessThan(expression.Key("aId"), expression.Value(30)))
	//	projection := expression.NamesList(expression.Name("coord[0]"), expression.Name("prep.txt"))
	//fcond := expression.Equal(expression.Name("aId"), expression.Value(30))
	//expr, err := expression.NewBuilder().WithKeyCondition(kcond).WithProjection(projection).Build()
	expr, err := expression.NewBuilder().WithKeyCondition(kcond).Build()
	if err != nil {
		panic(err)
	}
	input := &dynamodb.QueryInput{
		KeyConditionExpression:    expr.KeyCondition(),
		FilterExpression:          expr.Filter(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		//		ProjectionExpression:      expr.Projection(),
	}
	input = input.SetTableName("Activity").SetReturnConsumedCapacity("TOTAL").SetConsistentRead(false)
	//
	result, err := svc.Query(input)
	if err != nil {
		log.Print("error from Query:  " + err.Error())
		return nil, err
	}
	if int(*result.Count) == 0 {
		//fmt.Println("No activity data for reqRId " + reqRId_)
		return nil, fmt.Errorf("No data found for reqRId %s", reqRId_)
	}
	ActivityS := make([]Activity, int(*result.Count))
	err = dynamodbattribute.UnmarshalListOfMaps(result.Items, &ActivityS)
	//
	activityStart = &ActivityS[0]
	for _, v := range ActivityS {
		if v.Task != nil {
			fmt.Println("Activity data: ", v.AId, v.Task.Text)
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
	// link Task Activities - taskctl is a package variable. It is being assigned here only.
	//
	var j int
	for i, v := range ActivityS {
		if v.Task != nil {
			taskctl.start = &ActivityS[i]
			j = i
			taskctl.cnt++
			for i := j + 1; i < len(ActivityS); i++ {
				if ActivityS[i].Task != nil {
					ActivityS[j].nextTask = &ActivityS[i]
					j = i
					taskctl.cnt++
				}
			}
			break
		}
	}
	//
	// link Prep Activities - prepctl is a package variable. It is being assigned here only.
	//
	for i, v := range ActivityS {
		if v.Prep != nil {
			prepctl.start = &ActivityS[i]
			j = i
			prepctl.cnt++
			for i := j + 1; i < len(ActivityS); i++ {
				if ActivityS[i].Prep != nil {
					ActivityS[j].nextPrep = &ActivityS[i]
					j = i
					prepctl.cnt++
				}
			}
			break
		}
	}
	//
	// Table:  Container
	//
	kcond = expression.KeyEqual(expression.Key("rId"), expression.Value(reqRId_))
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
	input = input.SetTableName("Container").SetReturnConsumedCapacity("TOTAL").SetConsistentRead(false)
	//
	result, err = svc.Query(input)
	if err != nil {
		log.Print(err)
		return ActivityS, err
	}
	if int(*result.Count) == 0 {
		fmt.Println("No container data..")
	}
	// Container lookup - given Cid give me pointer to the continer.
	ContainerM = make(ContainerMap, int(*result.Count))
	var itemc *Container
	for _, i := range result.Items {
		itemc = new(Container)
		err = dynamodbattribute.UnmarshalMap(i, itemc)
		if err != nil {
			fmt.Println("Got error unmarshalling:")
			fmt.Println(err.Error())
			return ActivityS, err
		}
		ContainerM[itemc.Cid] = itemc
	}

	//
	// Table:  Unit
	//
	proj := expression.NamesList(expression.Name("slabel"), expression.Name("llabel"), expression.Name("desc"))
	expr, err = expression.NewBuilder().WithProjection(proj).Build()
	if err != nil {
		fmt.Println("Got error building expression:")
		fmt.Println(err.Error())
		return ActivityS, err
	}
	// Build the query input parameters
	params := &dynamodb.ScanInput{
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		ProjectionExpression:      expr.Projection(),
		TableName:                 aws.String("Unit"),
	}
	resultS, err := svc.Scan(params)
	if err != nil {
		fmt.Println("Query API call failed:")
		fmt.Println((err.Error()))
		return ActivityS, err
	}
	if int(*result.Count) == 0 {
		fmt.Println("No unit data..")
		return ActivityS, fmt.Errorf("No Unit data found")
	}
	unitM := make(map[string]*Unit, int(*result.Count))
	var unit *Unit
	for _, i := range resultS.Items {
		unit = new(Unit)
		err = dynamodbattribute.UnmarshalMap(i, unit)
		if err != nil {
			fmt.Println("Got error unmarshalling:")
			fmt.Println(err.Error())
			return ActivityS, err
		}
		unitM[unit.Slabel] = unit
	}
	// for k, v := range unitM {
	// 	fmt.Println(k, *v)
	// }
	//
	//  Post fetch processing - assign container pointers in Activity and validate that all containers referenced exist
	//
	doubleSpace := strings.NewReplacer("  ", " ")
	//
	// for all prep, tasks
	//
	for _, l := range []PrepTask{prep, task} {
		for i, ap := 0, activityStart; ap != nil; ap = ap.next {
			var p *PerformT
			switch l {
			case task:
				p = ap.Task
			case prep:
				p = ap.Prep
			}
			if p == nil {
				continue
			}
			//fmt.Println("Task: ", p.Text)
			if len(p.AddToC) > 0 {
				for i := 0; i < len(p.AddToC); i++ {
					if cId, ok := ContainerM[p.AddToC[i]]; !ok {
						if cId, ok = ContainerM[strings.TrimSpace(p.AddToC[i])]; !ok {
							fmt.Printf("Error:   Container [%s] not found for %s %d\n", strings.TrimSpace(p.AddToC[i]), ap.Label, ap.AId)
							continue
						}
					} else {
						// activity to container edge
						p.AddToCp = append(p.AddToCp, cId)
						// container to activity edge
						associatedTask := taskT{Type: l, Activityp: ap}
						cId.Task = append(cId.Task, associatedTask)
					}
				}
			}
			if len(p.UseC) > 0 {
				for i := 0; i < len(p.UseC); i++ {
					if cId, ok := ContainerM[p.UseC[i]]; !ok {
						if cId, ok = ContainerM[strings.TrimSpace(p.UseC[i])]; !ok {
							fmt.Printf("Error:   Container [%s] not found for %s %d\n", strings.TrimSpace(p.UseC[i]), ap.Label, ap.AId)
							continue
						}
					} else {
						p.UseCp = append(p.UseCp, cId)
						//cId.Activityp = append(cId.Activityp, t)
						associatedTask := taskT{Type: l, Activityp: ap}
						cId.Task = append(cId.Task, associatedTask)
					}
				}
			}
			if len(p.SourceC) > 0 {
				for i := 0; i < len(p.SourceC); i++ {
					if cId, ok := ContainerM[p.SourceC[i]]; !ok {
						if cId, ok = ContainerM[strings.TrimSpace(p.SourceC[i])]; !ok {
							fmt.Printf("Error:   Container [%s] not found for %s %d\n", strings.TrimSpace(p.SourceC[i]), ap.Label, ap.AId)
							continue
						}
					} else {
						p.SourceCp = append(p.SourceCp, cId)
						associatedTask := taskT{Type: l, Activityp: ap}
						cId.Task = append(cId.Task, associatedTask)
					}
				}
			}
			i++
		}
	}
	// check container is associated with an activity. if not delete from container map.
	for _, c := range ContainerM {
		if len(c.Task) == 0 {
			delete(ContainerM, c.Cid)
		}
	}
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
		s       string
	)
	//
	//  replace all {tag} in text and verbal for each activity. Ignore Link'd activites - they are only relevant at print time
	//
	var pt *PerformT
	for _, taskType := range []PrepTask{prep, task} {
		for _, interactionType := range []int{text, voice} {
			for p := activityStart; p != nil; p = p.next {
				switch taskType {
				case prep:
					pt = p.Prep
					if p.Prep == nil {
						continue
					}
				case task:
					pt = p.Task
					if p.Task == nil {
						continue
					}
				}
				switch interactionType {
				case text:
					s = pt.Text
				case voice:
					s = pt.Verbal
				}
				// if no {} then print and return to top of the loop
				t1 := strings.IndexByte(s, '{')
				if t1 < 0 {
					b.WriteString(s + " ")
					switch interactionType {
					case text:
						pt.text = doubleSpace.Replace(b.String())
					case voice:
						pt.verbal = doubleSpace.Replace(b.String())
					}
					b.Reset()
					continue
				}
				for tclose, topen := 0, strings.IndexByte(s, '{'); topen != -1; {
					fmt.Println(s)
					b.WriteString(s[tclose:topen])
					tclose += strings.IndexByte(s[tclose:], '}')
					switch strings.ToLower(s[topen+1 : tclose]) {
					case "iqual":
						{
							fmt.Fprintf(&b, "%s", p.IngrdQualifer)
						}
					case "csize":
						{
							fmt.Fprintf(&b, "%s", pt.AddToCp[0].Measure.Size)
						}
					case "addtoc":
						{
							fmt.Fprintf(&b, "%s", pt.AddToCp[0].Label)
						}
					case "qty":
						{
							context = measure
							fmt.Fprintf(&b, "%s", p.Measure.Quantity)
						}
					case "size":
						fmt.Fprintf(&b, "%s", p.Measure.Size)
					case "device":
						{
							fmt.Fprintf(&b, "%s", pt.UseDevice.Type)
							context = device
						}
					case "temp":
						{
							fmt.Fprintf(&b, "%s", pt.UseDevice.Temp)
						}
					case "unit":
						{
							switch context {
							case device:
								if u, ok := unitM[pt.UseDevice.Unit]; !ok {
									fmt.Printf("\n\n unit not defined %d %s ", p.AId, p.Measure.Unit)
									panic(fmt.Errorf("unit not defined"))
								} else {
									switch interactionType {
									case text:
										fmt.Fprintf(&b, "%s", u.Slabel)
									case voice:
										fmt.Fprintf(&b, "%s", u.Llabel+"s")
									}
								}
							case measure:
								if u, ok := unitM[p.Measure.Unit]; !ok {
									fmt.Printf("\n\n unit not defined %d %s ", p.AId, p.Measure.Unit)
									panic(fmt.Errorf("unit not defined"))
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
									fmt.Printf("\n\n unit not defined %d %s ", p.AId, p.Measure.Unit)
									panic(fmt.Errorf("unit not defined"))
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
					topen = strings.IndexByte(s[tclose:], '{')
					if topen == -1 {
						b.WriteString(s[tclose:])
					} else {
						topen += tclose
					}
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
	return ActivityS, nil

} // readBaseRecipeForTasks
