package main

import (
	_ "encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	_ "github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/dynamodb/expression"

	_ "github.com/aws/aws-lambda-go/lambdacontext"
)

// recipe lookup
type RnLkup struct {
	BkId   string `json:"PKey"`
	RId    int    `json:"SortK"`
	bkname string
}

type ctRec struct {
	Txt string `json="txt"`
	Vbl string `json="vbl"`
	EOL int
}

// use this struct as key into map
type mkey struct {
	size string
	typE string
}
type clsort []mkey

func (cs clsort) Len() int           { return len(cs) }
func (cs clsort) Less(i, j int) bool { return cs[i].size < cs[j].size }
func (cs clsort) Swap(i, j int)      { cs[i], cs[j] = cs[j], cs[i] }

type prepTaskRec struct {
	PKey   string // R-[BkId]
	SortK  int    // monitically increaseing - task at which user is upto in recipe
	AId    int    // Activity Id
	Type   byte
	time   float32 // all Linked preps sum time components into this field
	Text   string  // all Linked preps combined text into this field
	Verbal string
	EOL    int // End-Of-List. Max Id assigned to each record
}

type prepTaskS []prepTaskRec

func (od prepTaskS) Len() int           { return len(od) }
func (od prepTaskS) Less(i, j int) bool { return od[i].time > od[j].time }
func (od prepTaskS) Swap(i, j int)      { od[i], od[j] = od[j], od[i] }

func (a ContainerMap) saveContainerUsage(s *sessCtx) (string, error) {
	type ctRow struct {
		PKey  string
		SortK float64
		EOL   int
		Txt   string `json:"txt"`
		Vbl   string `json:"vbl"`
	}
	ctS := a.generateContainerUsage(s.dynamodbSvc)
	//
	var rows int
	eol := len(ctS)
	for i, v := range ctS {
		rows++
		ctd := ctRow{PKey: "C-" + s.reqRId + "-" + s.reqBkId, SortK: float64(i + 1), Txt: v, Vbl: v, EOL: eol}
		av, err := dynamodbattribute.MarshalMap(ctd)
		if err != nil {
			return "", fmt.Errorf("%s: %s", "Error: failed to marshal Record in saveContainerUsage", err.Error())
		}
		_, err = s.dynamodbSvc.PutItem(&dynamodb.PutItemInput{
			TableName: aws.String("Recipe"),
			Item:      av,
		})
		if err != nil {
			return "", fmt.Errorf("%s: %s", "Error: failed to PutItem in saveContainerUsage", err.Error())
		}
		time.Sleep(50 * time.Millisecond)
	}
	// update maxid set attribute in Recipe table associated with container object
	//err := s.updateRecipe(objectMap[container_], rows) - don't use this approach replaced with EOL in session table
	// if err != nil {
	// 	return ctS[0], err
	// }
	// return first task
	return ctS[0], nil
}

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
		//fmt.Printf("\nContainers:  %d  \n\n", len(ContainerM))
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

func (s *sessCtx) getContainerRecById() (ctRec, error) {

	type pKey struct {
		PKey  string
		SortK float64
	}
	ctrec := ctRec{}
	pkey := pKey{PKey: "C-" + s.reqRId + "-" + s.reqBkId, SortK: float64(s.recId)}
	av, err := dynamodbattribute.MarshalMap(&pkey)
	if err != nil {
		return ctrec, fmt.Errorf("%s: %s", "Error in MarshalMap of getContainerRecById", err.Error())
	}
	input := &dynamodb.GetItemInput{
		Key:       av,
		TableName: aws.String("Recipe"),
	}
	result, err := s.dynamodbSvc.GetItem(input)
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
		return ctrec, fmt.Errorf("%s: %s", "Error in GetItem of getContainerRecById", err.Error())
	}
	if len(result.Item) == 0 {
		return ctrec, fmt.Errorf("%s", "No data Found in GetItem in getContainerRecById")
		// a, err := readBaseRecipeForContainers(s.dynamodbSvc, s.reqRId)
		// if err != nil {
		// 	return ctrec, fmt.Errorf("%s: %s", "Error in readBaseRecipeForContainers of getContainerRecById", err.Error())
		// }
		// return s.saveContainerUsage(a)
	}
	err = dynamodbattribute.UnmarshalMap(result.Item, &ctrec)
	if err != nil {
		return ctrec, fmt.Errorf("%s: %s", "Error in UnmarshalMap of getContainerRecById", err.Error())
	}
	return ctrec, nil
}

// func (a Activities) TasksVerbal() string {
// 	var b strings.Builder
// 	b.WriteString(fmt.Sprintf("{ %q : [ ", jsonKey))
// 	for l, p := 0, taskctl.start; p != nil; l, p = l+1, p.nextTask {
// 		b.WriteString(fmt.Sprintf("%q", p.Task.verbal))
// 		if l < taskctl.cnt-1 {
// 			b.WriteString(".")
// 		}
// 	}
// 	b.WriteString("] } ")
// 	return b.String()
// }

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

// (s *sessCtx) saveTasks(a Activities) (prepTaskRec, error) {//TODO make method of Activities
func (a Activities) saveTasks(s *sessCtx) (prepTaskRec, error) {
	var rows int
	// only prep & task verbal and its text equivalent are saved.
	// Generate prep and tasks from Activities.
	ptS := a.GenerateTasks("T-" + s.reqBkId + "-" + s.reqRId)
	//
	// Fast bulk load is not a priority - trickle insert will suffice atleast for the moment.
	//
	for _, v := range ptS {
		rows++
		av, err := dynamodbattribute.MarshalMap(v)
		if err != nil {
			panic(fmt.Sprintf("failed to DynamoDB marshal Record, %v", err))
		}
		_, err = s.dynamodbSvc.PutItem(&dynamodb.PutItemInput{
			TableName: aws.String("Recipe"),
			Item:      av,
		})
		if err != nil {
			return prepTaskRec{}, fmt.Errorf("failed to put Record to DynamoDB, %v", err)
		}
		time.Sleep(50 * time.Millisecond)
	}
	return ptS[0], nil
}

func (s sessCtx) updateSession() (int, error) {
	// state data that must be maintained across sessions
	//
	//		Sid - session id
	//		BkId - Book id
	//		RId - RecipeId
	//		[]RecId - id of last record returned
	//		ATime - access time, updated each time
	//		Oper - operation - next,prev,goto,repeat,showall,modify
	//		Obj -  object - task, ingredient, utensil, container
	//		Qid - questionId
	//		EOF
	//
	type pKey struct {
		Sid string
	}
	var updateC expression.UpdateBuilder
	//book-recipe requests don't need a RecId set.
	if s.curreq != bookrecipe_ || s.reset { // reset on change of book or recipe
		// for the first object request in a session the RecId set will not exist - we need to SET. All other times we will ADD.
		//  we determine the first time using a len(recID) > 0 on the session query in the calling func.
		if s.reset || s.recIdNotExists {
			fmt.Println("SET RECID in sessions..for ", s.object)
			s.recIdNotExists = false
			// on insert build a prepopulated dynamodb set of int (internally float64 in dynamodb)
			switch len(s.object) {
			case 0:
				updateC = expression.Set(expression.Name("RecId"), expression.Value([]int{0, 0, 0, 0, 0}))
			default:
				switch s.object {
				case ingredient_:
					updateC = expression.Set(expression.Name("RecId"), expression.Value([]int{1, 0, 0, 0, 0}))
				case task_:
					updateC = expression.Set(expression.Name("RecId"), expression.Value([]int{0, 1, 0, 0, 0}))
				case container_:
					updateC = expression.Set(expression.Name("RecId"), expression.Value([]int{0, 0, 1, 0, 0}))
				case utensil_:
					updateC = expression.Set(expression.Name("RecId"), expression.Value([]int{0, 0, 0, 1, 0}))
				case recipe_:
					updateC = expression.Set(expression.Name("RecId"), expression.Value([]int{0, 0, 0, 0, 1}))
				default:
					updateC = expression.Set(expression.Name("RecId"), expression.Value([]int{0, 0, 0, 0, 0}))
				}
			}
		} else {
			// on update use ADD to increment an object related counter.
			recid_ := fmt.Sprintf("RecId[%d]", objectMap[s.object])
			updateC = expression.Add(expression.Name(recid_), expression.Value(s.updateAdd))
		}
	}

	updateC = updateC.Set(expression.Name("EOL"), expression.Value(s.eol)) //eol from get-RecId() associated with each Object

	if len(s.reqRId) > 0 {
		updateC = updateC.Set(expression.Name("RId"), expression.Value(s.reqRId))
	}
	// will clear Book entries provided execution paths bypasses mergeAndValidate func.
	if len(s.reqBkName) > 0 {
		updateC = updateC.Set(expression.Name("BKname"), expression.Value(s.reqBkName))
	} else {
		updateC = updateC.Set(expression.Name("BKname"), expression.Value(""))
	}
	if len(s.reqBkId) > 0 {
		updateC = updateC.Set(expression.Name("BkId"), expression.Value(s.reqBkId))
	} else {
		updateC = updateC.Set(expression.Name("BkId"), expression.Value(""))
	}
	if len(s.swapBkName) > 0 { //TODO - zeor Swp values when question 21 answered
		updateC = updateC.Set(expression.Name("SwpBkNm"), expression.Value(s.swapBkName))
		updateC = updateC.Set(expression.Name("SwpBkId"), expression.Value(s.swapBkId))
	}
	if len(s.operation) > 0 {
		updateC = updateC.Set(expression.Name("Oper"), expression.Value(s.operation)) // next,prev,repeat,modify,goto
	}
	if len(s.object) > 0 {
		updateC = updateC.Set(expression.Name("Obj"), expression.Value(s.object)) // ingredient,task,container,utensil
	}
	if len(s.reqRName) > 0 {
		updateC = updateC.Set(expression.Name("Rname"), expression.Value(s.reqRName))
	}
	if s.questionId > 0 {
		updateC = updateC.Set(expression.Name("Qid"), expression.Value(s.questionId))
	}
	updateC = updateC.Set(expression.Name("ATime"), expression.Value(time.Now().String()))

	expr, err := expression.NewBuilder().WithUpdate(updateC).Build()

	pkey := pKey{Sid: s.sessionId}
	av, err := dynamodbattribute.MarshalMap(&pkey)

	input := &dynamodb.UpdateItemInput{
		TableName:                 aws.String("Sessions"),
		Key:                       av, // accets []map[]*attributeValues so must use marshal not expression
		UpdateExpression:          expr.Update(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		ReturnValues:              aws.String("UPDATED_NEW"), //aws.String("ALL_NEW"),
	}
	result, err := s.dynamodbSvc.UpdateItem(input) // do an updateitem and return original id value so only one call.
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case dynamodb.ErrCodeProvisionedThroughputExceededException:
				fmt.Println(dynamodb.ErrCodeProvisionedThroughputExceededException, aerr.Error())
			case dynamodb.ErrCodeResourceNotFoundException:
				fmt.Println(dynamodb.ErrCodeResourceNotFoundException, aerr.Error())
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
		return 1, err
	}
	//
	// RecId has been updated so copy new value to session context
	//
	lastSess := sessRecT{}
	if len(result.Attributes) > 0 && s.curreq != bookrecipe_ {
		err = dynamodbattribute.UnmarshalMap(result.Attributes, &lastSess)
		if err != nil {
			return 1, err
		}
		fmt.Printf("\nin updateSession - lastSess: %#v ", lastSess)
		// NB: UPDATE_NEW in return values will return only updated elements in a slice/set
		//  In the case of SET all values are returned
		//	In the case of ADD only the changed element in the set is returned.
		switch len(lastSess.RecId) {
		case 1:
			return lastSess.RecId[0], nil
		default:
			return lastSess.RecId[objectMap[s.object]], nil
		}
	}
	return 1, nil
}

func (s *sessCtx) updateSessionEOL() error {
	// state data that must be maintained across sessions
	//
	//		Sid - session id
	//		BkId - Book id
	//		RId - RecipeId
	//		[]RecId - id of last record returned
	//		ATime - access time, updated each time
	//		Oper - operation - next,prev,goto,repeat,showall,modify
	//		Obj -  object - task, ingredient, utensil, container
	//		Qid - questionId
	//		EOF
	//
	type pKey struct {
		Sid string
	}
	var updateC expression.UpdateBuilder
	updateC = expression.Set(expression.Name("EOL"), expression.Value(s.eol))
	updateC = updateC.Set(expression.Name("ATime"), expression.Value(time.Now().String()))
	expr, err := expression.NewBuilder().WithUpdate(updateC).Build()
	//
	pkey := pKey{Sid: s.sessionId}
	av, err := dynamodbattribute.MarshalMap(&pkey)

	input := &dynamodb.UpdateItemInput{
		TableName:                 aws.String("Sessions"),
		Key:                       av, // accepts []map[]*attributeValues not string so must use marshal rather than expression
		UpdateExpression:          expr.Update(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		ReturnValues:              aws.String("UPDATED_NEW"),
	}
	_, err = s.dynamodbSvc.UpdateItem(input) // do an updateitem and return original id value so only one call.
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case dynamodb.ErrCodeProvisionedThroughputExceededException:
				fmt.Println(dynamodb.ErrCodeProvisionedThroughputExceededException, aerr.Error())
			case dynamodb.ErrCodeResourceNotFoundException:
				fmt.Println(dynamodb.ErrCodeResourceNotFoundException, aerr.Error())
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

	return nil
}

func (s *sessCtx) getTaskRecById() (prepTaskRec, error) {

	var taskRec prepTaskRec
	pKey := "T-" + s.reqBkId + "-" + s.reqRId
	fmt.Println("in getTaskRecById ")
	fmt.Printf("pkey, s.recId  %s  %d", pKey, s.recId)
	keyC := expression.KeyEqual(expression.Key("PKey"), expression.Value(pKey)).And(expression.KeyEqual(expression.Key("SortK"), expression.Value(s.recId)))
	expr, err := expression.NewBuilder().WithKeyCondition(keyC).Build()
	if err != nil {
		panic(err)
	}
	//
	// Table: Tasks - get current task based on task Id
	//
	input := &dynamodb.QueryInput{
		KeyConditionExpression:    expr.KeyCondition(),
		FilterExpression:          expr.Filter(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		//ProjectionExpression:      expr.Projection(),
	}
	input = input.SetTableName("Recipe").SetReturnConsumedCapacity("TOTAL").SetConsistentRead(false)
	//
	result, err := s.dynamodbSvc.Query(input)
	if err != nil {
		//return prepTaskRec{}, fmt.Errorf("Error in Query of Tasks: " + err.Error())
		panic(err)
	}
	if int(*result.Count) == 0 { //TODO - put this code back so it makes sense
		// this is caused by a goto operation exceeding EOL
		return prepTaskRec{}, fmt.Errorf("Error: %s [%s] ", "Internal error: no tasks found for recipe ", s.reqRName)
	}
	if int(*result.Count) > 1 {
		return prepTaskRec{}, fmt.Errorf("Error: more than 1 task returned from getNextRecordById")
	}
	err = dynamodbattribute.UnmarshalMap(result.Items[0], &taskRec)
	if err != nil {
		return prepTaskRec{}, fmt.Errorf("Error: %s - %s", "in UnmarshalMap in getTaskRecById ", err.Error())
	}
	return taskRec, nil
}

// type RnLkup struct {
// 	BkId   string `json:"PKey"`
// 	RId    int    `json:"SortK"`
// 	bkname string
// }

func (s *sessCtx) recipeNameLookup() ([]RnLkup, error) {
	//
	// query on recipe name to get RecipeId and optionally book name and Id if not requested
	//
	var (
		result   *dynamodb.QueryOutput
		allBooks bool
	)
	if len(s.reqBkName) > 0 {
		// using Bookname and Id
		//
		kcond := expression.KeyEqual(expression.Key("PKey"), expression.Value("R-"+s.reqBkId))
		filter := expression.Equal(expression.Name("RName"), expression.Value(s.reqRName))
		expr, err := expression.NewBuilder().WithKeyCondition(kcond).WithFilter(filter).Build()
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
		//
		result, err = s.dynamodbSvc.Query(input)
		if err != nil {
			return nil, fmt.Errorf("Error: %s [%s] %s", "in Query of recipeNameLookup of Rname ", s.reqRName, err.Error())
		}
		if int(*result.Count) == 0 {
			allBooks = true
		}
	}
	if len(s.reqBkName) == 0 || allBooks {
		// query on GSI - across all books
		kcond := expression.KeyEqual(expression.Key("RName"), expression.Value(s.reqRName))
		expr, err := expression.NewBuilder().WithKeyCondition(kcond).Build()
		if err != nil {
			panic(err)
		}
		input := &dynamodb.QueryInput{
			KeyConditionExpression:    expr.KeyCondition(),
			IndexName:                 aws.String("RName-BkId"),
			FilterExpression:          expr.Filter(),
			ExpressionAttributeNames:  expr.Names(),
			ExpressionAttributeValues: expr.Values(),
			//              ProjectionExpression:      expr.Projection(),
		}
		input = input.SetTableName("Recipe").SetReturnConsumedCapacity("TOTAL").SetConsistentRead(false)
		//
		result, err = s.dynamodbSvc.Query(input)
		if err != nil {
			return nil, fmt.Errorf("Error: %s [%s] %s", "in Query of recipeNameLookup of Rname ", s.reqRName, err.Error())
		}
		if int(*result.Count) == 0 {
			switch allBooks {
			case true:
				return nil, fmt.Errorf(`Recipe [%s] not found in [%s] or library. Please notify support`, s.reqRName, s.reqBkName)
			case false:
				return nil, fmt.Errorf(`Recipe [%s] not found in library. Please notify support`, s.reqRName)
			}
		}
	}
	ridBkidName := make([]RnLkup, int(*result.Count))
	err := dynamodbattribute.UnmarshalListOfMaps(result.Items, &ridBkidName)
	if err != nil {
		return nil, fmt.Errorf("Error: %s [%s] err", "in UnmarshalListMaps of recipeNameLookup ", s.reqRName, err.Error())
	}
	//
	for i := 0; i < len(ridBkidName); i++ {
		ridBkidName[i].BkId = ridBkidName[i].BkId[2:]
		ridBkidName[i].bkname, err = s.bookNameLookup(ridBkidName[i].BkId) // trim "R-" prefix
		if err != nil {
			return nil, err
		}
	}
	if allBooks && int(*result.Count) > 0 {
		switch int(*result.Count) {
		case 1:
			s.msg = fmt.Sprintf("Alert: recipe not found in [%s], but was found in [%s]", s.reqBkName, ridBkidName[0].bkname)
		default:
			s.msg = fmt.Sprintf("Alert: recipe not found in [%s] but did occur in other books. See list", s.reqBkName)
		}
	} else {
		if int(*result.Count) > 1 {
			s.msg = fmt.Sprint("Alert: recipe found in multiple books. See list")
		}
	}
	// update session context request fields
	if len(s.reqBkName) == 0 {
		s.reqBkName = ridBkidName[0].bkname
		s.reqBkId = ridBkidName[0].BkId
	}
	s.reqRId = strconv.Itoa(ridBkidName[0].RId)

	return ridBkidName, nil // ridBkidName contains list of books containing the recipe. Useful in cases where there is more than one.
}

func (s sessCtx) bookNameLookup(bkId string) (string, error) {

	kcond := expression.KeyEqual(expression.Key("BkId"), expression.Value(bkId))
	expr, err := expression.NewBuilder().WithKeyCondition(kcond).Build()
	if err != nil {
		panic(err)
	}
	input := &dynamodb.QueryInput{
		KeyConditionExpression:    expr.KeyCondition(),
		IndexName:                 aws.String("BkId-BkName"),
		FilterExpression:          expr.Filter(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
	}
	input = input.SetTableName("Recipe").SetConsistentRead(false)
	// while BkId is unique we are using a GSI so must use Query (I presume)
	result, err := s.dynamodbSvc.Query(input)
	if err != nil {
		return "c", fmt.Errorf("Error: %s [%s] %s", "in Query in bookNameLookup of ", bkId, err.Error())
	}
	if int(*result.Count) == 0 {
		return "c", fmt.Errorf("No data found in bookNameLookup, for bookId [%s]", bkId)
	}
	if int(*result.Count) > 1 {
		return "c", fmt.Errorf("Internal error in bookNameLookup. %s [%s]", "More than one book found for bookId ", bkId)
	}
	// define a slice of struct as Query expects to return 1 or more rows so the slice represents a row
	// and we ue unmarshallistofmaps to handle a batch like select
	bookName := make([]struct{ PKey string }, int(*result.Count))
	err = dynamodbattribute.UnmarshalListOfMaps(result.Items, &bookName)
	if err != nil {
		return "c", fmt.Errorf("Error: %s [%s] %s", "in UnmarshalMaps in bookNameLookup ", s.reqRName, err.Error())
	}
	//
	return bookName[0].PKey[3:], nil // trim "BK-" prefix
}

func (s sessCtx) bookIdLookup() (string, error) {
	// retrieves book id given requeted book name
	kcond := expression.KeyEqual(expression.Key("PKey"), expression.Value("BK-"+s.reqBkName))
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
	input = input.SetTableName("Recipe").SetConsistentRead(false)
	// while BkId is unique we are using a GSI so must use Query (I presume)
	result, err := s.dynamodbSvc.Query(input)
	if err != nil {
		return "-", fmt.Errorf("Error: %s [%s] %s", "in Query in bookIdLookup of ", s.reqBkName, err.Error())
	}
	if int(*result.Count) == 0 {
		return "-", fmt.Errorf("No data found in bookIdLookup, for book name [%s]", s.reqBkName)
	}
	if int(*result.Count) > 1 {
		return "-", fmt.Errorf("More than one found in bookIdLookup [%s]", s.reqBkName)
	}
	// define a slice of struct as Query expects to return 1 or more rows so the slice represents a row
	// and we ue unmarshallistofmaps to handle a batch like select
	rec := make([]struct{ BkId string }, int(*result.Count))
	err = dynamodbattribute.UnmarshalListOfMaps(result.Items, &rec)
	if err != nil {
		return "-", fmt.Errorf("Error: %s [%s] %s", "in UnmarshalMaps in bookIdLookup ", s.reqBkName, err.Error())
	}
	return rec[0].BkId, nil
}
