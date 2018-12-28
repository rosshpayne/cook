package main

import (
	_ "encoding/json"
	"errors"
	"fmt"
	"log"
	"sort"
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
	// Rid      string     `json:"PKey"`
	Cid      string     `json:"SortK"`
	Label    string     `json:"label"`
	Type     string     `json:"type"`
	Purpose  string     `json:"purpose"`
	Coord    [2]float32 `json:"coord"`
	Measure  MeasureCT  `json:"measure"`
	Contains string     `json:"contains"`
	Message  string     `json:"message"`
	start    int        // first id in recipe tasks where container is used
	last     int        // last id in recipe tasks where container is sourced from or recipe is complete.
	Activity []taskT    // slice of tasks (Prep and Task activites) associated with container
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
	UseCp     []*Container // ---"---
	SourceCp  []*Container // ---"---
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
	// Pkey          string     `json:"PKey"`
	AId           int        `json:"SortK"`
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

func (cm ContainerMap) generateContainerUsage(svc *dynamodb.DynamoDB) []string {
	type ctCount struct {
		C   []*Container
		num int
	}
	var b strings.Builder
	output_ := []string{}
	if len(cm) > 0 {
		// map[Size Type]count
		containerList := make(map[mkey]*ctCount)
		// ContainerM - map[Cid]*Container
		// group by  container type and size - as both represent attribues of a single physical container we want to count.
		for _, v := range cm {
			z := mkey{v.Measure.Size, v.Type}
			if y, ok := containerList[z]; !ok {
				// key does not exist, go returns zero value of *ctCount, ie. nil
				// create new pointer value and assign values
				y := new(ctCount)
				y.num = 1
				// zero value of slice is nil (metadata only), first append will allocate underlying array
				y.C = append(y.C, v)
				containerList[z] = y
			} else {
				y.num += 1
				y.C = append(y.C, v)
			}
		}
		// populate slice which satisfies sort interface. After sorting containers of same type together but of different sizes
		// 2xlarge glass bowel 1xsmall glass bowel
		clsorted := clsort{}
		for k, _ := range containerList {
			clsorted = append(clsorted, k)
		}
		// use sorted keys to access ma
		sort.Sort(clsorted)
		for _, v := range clsorted {
			if containerList[v].num > 1 {
				b.WriteString(fmt.Sprintf(" %d %s ", containerList[v].num, strings.Title(v.size+" "+v.typE+"s")))
				for i, d := range containerList[v].C {
					switch i {
					case 0:
						b.WriteString(fmt.Sprintf(" one for %s", d.Purpose+" "+d.Contains+" "))
					default:
						b.WriteString(fmt.Sprintf("%s ", " another for "+d.Purpose+" "+d.Contains))
					}
				}
			} else {
				c := containerList[v].C[0]
				if len(v.size) != 0 {
					b.WriteString(fmt.Sprintf(" %d %s ", containerList[v].num, strings.Title(v.size+" "+v.typE)))
				} else {
					b.WriteString(fmt.Sprintf(" %d %.0fx%.0f%s %s ", containerList[v].num, c.Measure.Diameter, c.Measure.Height, c.Measure.Unit, strings.Title(v.typE)))
				}
				for _, d := range containerList[v].C {
					b.WriteString(fmt.Sprintf(" for %s ", d.Purpose+" "+d.Contains+"  "))
				}
			}
			output_ = append(output_, b.String())
			b.Reset()
		}
	}
	// store number of records in recipe table
	return output_
}

func (a Activities) GenerateTasks(pKey string) prepTaskS {
	// Merge and Populate prepTask and then sort.
	//  1. first load parrellelisable tasks identified by words or prep property "parallel" or device (=oven)
	//  2. sort
	//  3. add other tasks in order
	//
	var ptS prepTaskS // this type satisfies sort interface.
	processed := make(map[int]bool, prepctl.cnt)
	//
	// sort parallelisable prep tasks
	//
	for p := prepctl.start; p != nil; p = p.nextPrep {
		var add bool
		var pp = p.Prep
		if pp.UseDevice != nil {
			if strings.ToLower(pp.UseDevice.Type) == "oven" {
				add = true
			}
		}
		if pp.Parallel && !pp.Link || add {
			if p.prev != nil && p.prev.Prep != nil {
				if p.prev.Prep.Link {
					continue // exclude if part of linked activity
				}
			}
			processed[p.AId] = true
			pt := prepTaskRec{PKey: pKey, AId: p.AId, Type: 'P', time: pp.Time, Text: pp.text, Verbal: pp.verbal}
			ptS = append(ptS, pt)
		}
	}
	sort.Sort(ptS)
	//
	// generate Task Ids
	//
	var i int = 1 // start at one as works better with UpateItem ADD semantics.
	for j := 0; j < len(ptS); i++ {
		ptS[j].SortK = i
		j++
	}
	//
	// append remaining prep tasks - these are serial tasks so order unimportant
	//
	for p := prepctl.start; p != nil; p = p.nextPrep {
		if _, ok := processed[p.AId]; ok {
			continue
		}
		pt := prepTaskRec{PKey: pKey, SortK: i, AId: p.AId, Type: 'P', time: p.Prep.Time, Text: p.Prep.text, Verbal: p.Prep.verbal}
		ptS = append(ptS, pt)
		i++
	}
	//
	// append tasks
	//
	for p := taskctl.start; p != nil; p = p.nextTask {
		pt := prepTaskRec{PKey: pKey, SortK: i, AId: p.AId, Type: 'T', time: p.Task.Time, Text: p.Task.text, Verbal: p.Task.verbal}
		ptS = append(ptS, pt)
		i++
	}
	// now that we know the size of the list assign End-Of-List field. This approach replaces MaxId[] set stored in Recipe table
	// this mean each record knows how long the list is - helpful in a stateless Lambda app.
	eol := len(ptS)
	for i := range ptS {
		ptS[i].EOL = eol
	}
	// store number of records in recipe table
	return ptS
}

func (a Activities) PrintRecipe(rId string) (prepTaskS, string) {
	// *** NB> . this currently only handles prep tasks - which may not be relevant in printed recipe. So this func may not be useful.
	// Merge and Populate prepTask and then sort.
	//  1. first load parrellelisable tasks identified by words or prep property "parallel" or device (=oven)
	//  2. sort
	//  3. add other tasks in order
	//
	var ptS prepTaskS
	pid := 0                                     // index in prepOrder
	processed := make(map[int]bool, prepctl.cnt) // set of tasks
	//
	// sort parallelisable prep tasks
	//
	for p := prepctl.start; p != nil; p = p.nextPrep {
		var add bool
		var pp = p.Prep
		if pp.UseDevice != nil {
			if strings.ToLower(pp.UseDevice.Type) == "oven" {
				add = true
			}
		}
		if pp.Parallel && !pp.Link || add {
			if p.prev != nil && p.prev.Prep != nil {
				if p.prev.Prep.Link {
					continue // exclude if part of linked activity
				}
			}
			processed[p.AId] = true
			pt := prepTaskRec{time: pp.Time, Text: pp.text}
			ptS = append(ptS, pt)
		}
	}
	sort.Sort(ptS)
	//
	// append remaining prep tasks - these are serial tasks so order unimportant
	//
	for p := prepctl.start; p != nil; p = p.nextPrep {
		if _, ok := processed[p.AId]; ok {
			continue
		}
		var txt string
		var stime float32
		var count int
		if p.Prep.Link {
			for ; p.Prep.Link; p = p.nextPrep {
				//handle Link prep tasks
				txt += p.Prep.text + " and "
				stime += p.Prep.Time
				count++
			}
			txt += p.Prep.text
			stime += p.Prep.Time
			//
			pt := prepTaskRec{time: stime, Text: txt}
			ptS = append(ptS, pt)
		} else {
			pt := prepTaskRec{time: p.Prep.Time, Text: p.Prep.text}
			ptS = append(ptS, pt)
		}
		pid++
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("{ %q : [", jsonKey))
	for i, pt := range ptS {
		b.WriteString(fmt.Sprintf("%q", pt.Text))
		if i < len(ptS)-1 {
			b.WriteString(",")
		}
	}
	b.WriteString("] } ")
	return ptS, b.String()
} // PrintRecipe

//func readBase(..) (interface{} , error)
func (s *sessCtx) readBaseRecipeForContainers(ptS prepTaskS) (ContainerMap, error) {
	//
	// Table:  Activity
	//
	kcond := expression.KeyEqual(expression.Key("PKey"), expression.Value("A-"+s.reqBkId+"-"+s.reqRId))
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
		log.Print("error from Query:  " + err.Error())
		//return nil//, err
	}
	if int(*result.Count) == 0 {
		return nil, fmt.Errorf("No data found for reqRId %s in readBaseRecipeForTasks for Activity - ", s.reqRId)
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
	// Parse Activity and generate Containers
	//  If C-0-0 type container then one its a single-activity-container (SAC) ie. a single-ingredient-container (SIC)
	//  if not a member of C-0-0 then maybe shared amoung activities.
	//
	// Table:  Container
	//
	kcond = expression.KeyEqual(expression.Key("PKey"), expression.Value("C-"+s.reqBkId+"-"+s.reqRId))
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
			fmt.Println(err.Error())
			//return nil //, fmt.Errorf("%s", "Error in UnmarshalMap of container table: "+err.Error())
		}
		fmt.Println("** Adding container: ", itemc.Cid)
		ContainerM[itemc.Cid] = itemc
	}
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
		fmt.Println()
		//return nil //, fmt.Errorf("%s", "Error in Query of container table: "+err.Error())
	}
	if int(*result.Count) == 0 {
		fmt.Println("No container data..")
	}
	ContainerSAM := make(ContainerMap, int(*result.Count))
	for _, i := range result.Items {
		itemc = new(Container)
		err = dynamodbattribute.UnmarshalMap(i, itemc)
		if err != nil {
			fmt.Println(err.Error())
			//return nil //, fmt.Errorf("%s", "Error in UnmarshalMap of container table: "+err.Error())
		}
		fmt.Println("Add to ContainerSAM ", itemc.Cid)
		ContainerSAM[itemc.Cid] = itemc
		//ContainerM[itemc.Cid] = itemc
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
	resultS, err := s.dynamodbSvc.Scan(params)
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
	// parse Activities for containers.  Add any single-activity containers to ContainerM.
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
			// now compare contains defined in each activity with those registered for
			// the recipe and those that are single-activity-containers
			if len(p.AddToC) > 0 {
				// activity containers are held in []string
				for i := 0; i < len(p.AddToC); i++ {
					// ContainerM contains registered containers
					cId, ok := ContainerM[strings.TrimSpace(p.AddToC[i])]
					if !ok {
						// ContainerSAM contains single activity containers
						if cId, ok = ContainerSAM[strings.TrimSpace(p.AddToC[i])]; !ok {
							// is not a single ingredient container or not a registered container
							fmt.Printf("Error:   Container [%s] not found for %s %d\n", strings.TrimSpace(p.AddToC[i]), ap.Label, ap.AId)
							continue
						}
						// Single-Activity-Containers are not pre-configured by the user into the Container repo - to make life easier.
						// dynamically create a container with a new Cid, and add to ContainerM and update all references to it.
						cs := p.AddToC[i] // original non-activity-specific container name
						c := new(Container)
						c.Cid = p.AddToC[i] + "-" + strconv.Itoa(ap.AId)
						c.Contains = ap.Ingredient
						c.Measure = cId.Measure
						c.Label = cId.Label
						c.Type = cId.Type
						// register container by adding to map
						ContainerM[c.Cid] = c
						// update container id
						p.AddToC[i] = c.Cid
						// search for other references and change its name
						if l == prep {
							// update other reference before we get there.
							for i := 0; i < len(ap.Task.SourceC); i++ {
								if ap.Task.SourceC[i] == cs {
									ap.Task.SourceC[i] = c.Cid
									break
								}
							}
							for i := 0; i < len(ap.Task.UseC); i++ {
								if ap.Task.UseC[i] == cs {
									ap.Task.UseC[i] = c.Cid
									break
								}
							}
						}
						cId = c
					}
					fmt.Println("addToCp..append now")
					// activity to container edge
					p.AddToCp = append(p.AddToCp, cId)
					// container to activity edge
					associatedTask := taskT{Type: l, Activityp: ap}
					cId.Activity = append(cId.Activity, associatedTask)
				}
			}

			if len(p.UseC) > 0 {
				for i := 0; i < len(p.UseC); i++ {
					// ContainerM contains registered containers
					cId, ok := ContainerM[strings.TrimSpace(p.UseC[i])]
					if !ok {
						// ContainerSAM contains single activity containers
						if cId, ok = ContainerSAM[strings.TrimSpace(p.UseC[i])]; !ok {
							// is not a single ingredient container or not a registered container
							fmt.Printf("Error:   Container [%s] not found for %s %d\n", strings.TrimSpace(p.AddToC[i]), ap.Label, ap.AId)
							continue
						}
						// container referened in activity is a single-activity-container (SAP)
						// manually create container and add to ContainerM and update all references to it.
						cs := p.UseC[i] // original non-activity-specific container name
						c := new(Container)
						c.Cid = p.UseC[i] + "-" + strconv.Itoa(ap.AId)
						c.Contains = ap.Ingredient
						c.Measure = cId.Measure
						c.Label = cId.Label
						c.Type = cId.Type
						// register container by adding to map
						ContainerM[c.Cid] = c
						// update name of container in Activity to <name>-AId
						p.UseC[i] = c.Cid
						// search for other references and change its name
						if l == prep {
							// update task based reference before we get there.
							for i := 0; i < len(ap.Task.SourceC); i++ {
								if ap.Task.SourceC[i] == cs {
									ap.Task.SourceC[i] = c.Cid
									break
								}
							}
							for i := 0; i < len(ap.Task.UseC); i++ {
								if ap.Task.UseC[i] == cs {
									ap.Task.UseC[i] = c.Cid
									break
								}
							}
						}
						cId = c
					}
					fmt.Println("UseCp..append now")
					p.UseCp = append(p.UseCp, cId)
					//cId.Activityp = append(cId.Activityp, t)
					associatedTask := taskT{Type: l, Activityp: ap}
					cId.Activity = append(cId.Activity, associatedTask)
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
						cId.Activity = append(cId.Activity, associatedTask)
					}
				}
			}
			i++
		}
	}
	// check container is associated with an activity. if not delete from container map.
	for _, c := range ContainerM {
		if len(c.Activity) == 0 {
			delete(ContainerM, c.Cid)
		}
	}
	// assign first index into ptS for each container
	for _, v := range ContainerM {
		v.start = 99999
		// each container appears in a list prep tasks and tasks.
		for _, t := range v.Activity {
			// find first appearance in task list (typcially useC or addToC)
			for l, r := range ptS {
				l := l + 1
				if r.AId == t.Activityp.AId {
					if v.start > l {
						ContainerM[v.Cid].start = l
						break
					}
				}
			}
		}
	}
	// assign last index into ptS for each container.
	for _, v := range ContainerM {
		// each container appears in a list prep tasks and tasks.
		for _, t := range v.Activity {
			// find last appearance in task list (typcially useC or addToC)
			for i := len(ptS) - 1; i >= 0; i-- {
				// find last appearance (typically sourceC). Start at last ptS and work backwards
				if ptS[i].AId == t.Activityp.AId {
					if v.last < i+1 {
						ContainerM[v.Cid].last = i + 1
						break
					}
				}
			}
		}
	}
	for _, v := range ContainerM {
		fmt.Printf("Container %#v\n", v)
	}
	//
	// devices
	//
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

func (s *sessCtx) readBaseRecipeForTasks() (Activities, error) {
	//
	// Table:  Activity
	//
	kcond := expression.KeyEqual(expression.Key("PKey"), expression.Value("A-"+s.reqBkId+"-"+s.reqRId))
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
	input = input.SetTableName("Recipe").SetReturnConsumedCapacity("TOTAL").SetConsistentRead(false)
	//
	result, err := s.dynamodbSvc.Query(input)
	if err != nil {
		log.Print("error from Query:  " + err.Error())
		return nil, err
	}
	if int(*result.Count) == 0 {
		//fmt.Println("No activity data for reqRId " + reqRId_)
		return nil, fmt.Errorf("No data found for reqRId %s", s.reqRId)
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
	kcond = expression.KeyEqual(expression.Key("PKey"), expression.Value("C-"+s.reqBkId+"-"+s.reqRId))
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
			return ActivityS, err
		}
		ContainerM[itemc.Cid] = itemc
	}
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
	if int(*result.Count) == 0 {
		fmt.Println("No container data..")
	}
	for _, i := range result.Items {
		itemc = new(Container)
		err = dynamodbattribute.UnmarshalMap(i, itemc)
		if err != nil {
			return nil, fmt.Errorf("%s", "Error in UnmarshalMap of container table: "+err.Error())
		}
		ContainerM[itemc.Cid] = itemc
	}
	//
	// Table:  Unit
	//
	proj := expression.NamesList(expression.Name("slabel"), expression.Name("llabel"), expression.Name("desc"))
	expr, err = expression.NewBuilder().WithProjection(proj).Build()
	if err != nil {
		return ActivityS, err
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
		return ActivityS, err
	}
	if int(*result.Count) == 0 {
		return ActivityS, fmt.Errorf("No Unit data found")
	}
	unitM := make(map[string]*Unit, int(*result.Count))
	var unit *Unit
	for _, i := range resultS.Items {
		unit = new(Unit)
		err = dynamodbattribute.UnmarshalMap(i, unit)
		if err != nil {
			return ActivityS, fmt.Errorf("Error: in readBaseRecipeForTasks UnmarshalMap for Units - %s", err.Error())
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
					str = pt.Text
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
				for tclose, topen := 0, strings.IndexByte(str, '{'); topen != -1; {
					b.WriteString(str[tclose:topen])
					tclose += strings.IndexByte(str[tclose:], '}')
					switch strings.ToLower(str[topen+1 : tclose]) {
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
					topen = strings.IndexByte(str[tclose:], '{')
					if topen == -1 {
						b.WriteString(str[tclose:])
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

} //
