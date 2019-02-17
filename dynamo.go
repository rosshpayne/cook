package main

import (
	_ "encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	_ "time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	_ "github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/dynamodb/expression"

	_ "github.com/aws/aws-lambda-go/lambdacontext"
)

//

// recipe lookup
type RnLkup struct {
	BkId   string `json:"PKey"`
	RId    int    `json:"SortK"`
	bkname string
}

type ctRec struct {
	Text   string `json:"txt"`
	Verbal string `json:"vbl"`
	EOL    int
}

func (ct ctRec) Alexa() dialog {
	return dialog{Verbal: ct.Verbal, Display: ct.Text, EOL: ct.EOL}
}

type mRecipeT struct {
	Id       int
	IngrdCat string
	RName    string
	RId      string
	BkName   string
	BkId     string
	Authors  string
	Quantity string
	Serves   string
}

type prepTaskRec struct {
	PKey   string  `json:"PKey"`  // R-[BkId]
	SortK  int     `json:"SortK"` // monotonically increasing - task at which user is upto in recipe
	AId    int     `json:"AId"`   // Activity Id
	Type   byte    `json:"Type"`
	time   float32 // all Linked preps sum time components into this field
	Text   string  `json:"Text"` // all Linked preps combined text into this field
	Verbal string  `json:"Verbal"`
	EOL    int     `json:"EOL"` // End-Of-List. Max Id assigned to each record
	// Recipe Part metadata
	PEOL int    `json:"PEOL"` // End-of-List-for-part
	PId  int    `json:"PId"`  // instruction id within a part
	Part string `json:"PT"`   // part index name
	Next int    `json:"nxt"`  // next SortK (recId)
	Prev int    `json:"prv"`  // previous SortK (recId) when in part mode as opposed to full recipe mode
	// not persisted
	taskp *PerformT
}

func (pt prepTaskRec) Alexa() dialog {
	return dialog{Verbal: pt.Verbal, Display: pt.Text, EOL: pt.EOL, PEOL: pt.PEOL, PID: pt.PId, PART: pt.Part}
}

type prepTaskS []*prepTaskRec

func (od prepTaskS) Len() int           { return len(od) }
func (od prepTaskS) Less(i, j int) bool { return od[i].time > od[j].time }
func (od prepTaskS) Swap(i, j int)      { od[i], od[j] = od[j], od[i] }

func (a ContainerMap) saveContainerUsage(s *sessCtx) error {
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
		ctd := ctRow{PKey: "C-" + s.pkey, SortK: float64(i + 1), Txt: v, Vbl: v, EOL: eol}
		av, err := dynamodbattribute.MarshalMap(ctd)
		if err != nil {
			return fmt.Errorf("%s: %s", "Error: failed to marshal Record in saveContainerUsage", err.Error())
		}
		_, err = s.dynamodbSvc.PutItem(&dynamodb.PutItemInput{
			TableName: aws.String("Recipe"),
			Item:      av,
		})
		if err != nil {
			return fmt.Errorf("%s: %s", "Error: failed to PutItem in saveContainerUsage", err.Error())
		}
		//time.Sleep(50 * time.Millisecond)
	}

	return nil
}

type containerT struct {
	Verbal string `json:"vbl"`
	Text   string `json:"txt"`
}

func (s *sessCtx) getContainers() []containerT {

	// fetch all container rows associated with a recipe
	// PKey = C-[BkId]-[RId]
	keyC := expression.KeyEqual(expression.Key("PKey"), expression.Value("C-"+s.pkey))
	expr, err := expression.NewBuilder().WithKeyCondition(keyC).Build()
	if err != nil {
		panic(err)
	}
	//
	input := &dynamodb.QueryInput{
		KeyConditionExpression:    expr.KeyCondition(),
		FilterExpression:          expr.Filter(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
	}
	input = input.SetTableName("Recipe").SetReturnConsumedCapacity("TOTAL").SetConsistentRead(false)
	//
	result, err := s.dynamodbSvc.Query(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case dynamodb.ErrCodeProvisionedThroughputExceededException:
				fmt.Println(dynamodb.ErrCodeProvisionedThroughputExceededException, aerr.Error())
			case dynamodb.ErrCodeResourceNotFoundException:
				fmt.Println(dynamodb.ErrCodeResourceNotFoundException, aerr.Error())
			// case dynamodb.ErrCodeRequestLimitExceeded:
			// 	fmt.Println(dynamodb.ErrCodeRequestLimitExceeded, aerr.Error())
			case dynamodb.ErrCodeInternalServerError:
				fmt.Println(dynamodb.ErrCodeInternalServerError, aerr.Error())
			default:
				fmt.Println(aerr.Error())
			}
			panic(aerr.Error())
		} else {
			// Print the error, cast err to awserr.Error to get the Code and
			// Message from an error.
			panic(err.Error())
		}
		panic(fmt.Errorf("%s: %s", "Error in GetItem of getContainerRecById", err.Error()))
	}

	recS := make([]containerT, int(*result.Count))
	err = dynamodbattribute.UnmarshalListOfMaps(result.Items, &recS)
	if err != nil {
		panic(fmt.Errorf("Error: %s [%s] err", "in UnmarshalListMaps of getContainers ", s.reqRName, err.Error()))
	}
	return recS
}

func (s *sessCtx) getContainerRecById() (alexaDialog, error) {
	type pKey struct {
		PKey  string
		SortK float64
	}
	ctrec := ctRec{}
	pkey := pKey{PKey: "C-" + s.pkey, SortK: float64(s.objRecId)}
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
	}
	err = dynamodbattribute.UnmarshalMap(result.Item, &ctrec)
	if err != nil {
		return ctrec, fmt.Errorf("%s: %s", "Error in UnmarshalMap of getContainerRecById", err.Error())
	}
	return ctrec, nil
}

type indexRecipeT struct {
	PKey     string
	SortK    string
	Quantity string
	BkName   string
	RName    string
	Authors  string
	Srv      string
}

func (s *sessCtx) generateAndSaveIndex(labelM map[string]*Activity, ingrdM map[string]*Activity) error {

	var indexRecS []indexRecipeT
	indexRow := make(map[string]bool)
	// any string() methods will be writing for the display
	writeCtx = uDisplay

	saveIngdIndex := func() error {
		for _, v := range indexRecS {
			av, err := dynamodbattribute.MarshalMap(v)
			if err != nil {
				panic(fmt.Sprintf("failed in IndexIngd to marshal Record, %v", err))
			}
			_, err = s.dynamodbSvc.PutItem(&dynamodb.PutItemInput{
				TableName: aws.String("Ingredient"),
				Item:      av,
			})
			if err != nil {
				return fmt.Errorf("failed in IndexIngd to PutItem into Ingredient table - %v", err)
			}
		}
		s.indexRecs = indexRecS // free memory. Probably redundant as its local to this func so once func exists memory would be freed anyway.
		return nil
	}

	indexBasicEntry := func(entry string) {
		//
		irec := indexRecipeT{SortK: s.reqBkId + "-" + s.reqRId, BkName: s.reqBkName, RName: s.reqRName, Authors: s.authors, Srv: s.recipe.Serves}
		irec.PKey = strings.TrimRight(strings.TrimLeft(strings.ToLower(entry), " "), " ")
		if !indexRow[entry] {
			// only append unique values..
			indexRow[entry] = true
			indexRecS = append(indexRecS, irec)
		}
	}

	makeIndexRecs := func(entry string, ap *Activity) {
		// for each index property value, add to index
		irec := indexRecipeT{SortK: s.reqBkId + "-" + s.reqRId, BkName: s.reqBkName, RName: s.reqRName, Authors: s.authors, Srv: s.recipe.Serves}
		irec.PKey = entry
		irec.Quantity = ap.String()
		if !indexRow[entry] {
			// only append unique values..
			indexRow[entry] = true
			indexRecS = append(indexRecS, irec)
		}
	}

	AddEntry := func(entry string) {
		// entry with hyphon is treated as one word
		entry = strings.Replace(entry, "-", " ", 1)
		// remove hyphon when saving as index entry though
		entry = strings.TrimRight(strings.TrimLeft(strings.ToLower(entry), " "), " ")
		// for each word in index entry find associated activity via its label or ingredient
		//  if not found then create a basic index entry (withou ingredient details)
		var indexed bool
		w := entry
		if a, ok := labelM[strings.ToLower(w)]; ok {
			makeIndexRecs(entry, a)
			indexed = true
		}
		if !indexed {
			if a, ok := ingrdM[strings.ToLower(w)]; ok {
				makeIndexRecs(entry, a)
				indexed = true
			}
		}
		// if not indexed, try searching using individual words in entry
		//  - but index using whole entry not the word.
		if !indexed {
			for _, w := range strings.Split(entry, " ") {
				if a, ok := labelM[strings.ToLower(w)]; ok {
					makeIndexRecs(entry, a)
					break
				}
				if a, ok := ingrdM[strings.ToLower(w)]; ok {
					makeIndexRecs(entry, a)
					break
				}
			}
		}
		if !indexRow[entry] {
			indexBasicEntry(entry)
		}
	}
	//
	// source index entries from recipe index attribute (saved to sessctx)
	//  check each word in entry against label and ingredient to add quantity data
	//  if not present just add recipe name and book data to index entry
	//
	//  entry -> "a b c" , potentially creates the following index entries.
	//   1      "a b c"
	//   2     "a" "b" "c"
	//	 3		"a b"
	//	 4		"a c"
	//	 5		"b c"
	for _, entry := range s.index {
		AddEntry(entry)
		e := strings.Split(entry, " ")
		switch len(e) {
		case 2:
			AddEntry(e[0])
			AddEntry(e[1])
		case 3:
			AddEntry(e[0])
			AddEntry(e[1])
			AddEntry(e[2])
			AddEntry(e[0] + " " + e[1])
			AddEntry(e[0] + " " + e[2])
			AddEntry(e[1] + " " + e[2])
		}
	}
	s.indexRecs = indexRecS
	err := s.generateSlotEntries()
	if err != nil {
		return fmt.Errorf("Error in generateAndSaveIndex at  generateSlotEntries - %s", err.Error())
	}
	err = saveIngdIndex()
	return err
}

func (d DevicesMap) saveDevices(s *sessCtx) error {
	var row int
	//
	type Pkey struct {
		PKey    string `json:"PKey"`
		SortK   int    `json:"SortK"`
		Device  string `json:"Device"`
		Comment string `json:"Comment"`
	}
	for k, v := range d {
		r := &Pkey{PKey: "D-" + s.pkey, SortK: row, Device: k, Comment: v}
		row++
		av, err := dynamodbattribute.MarshalMap(r)
		if err != nil {
			return fmt.Errorf("Error in saveDevices, MarshalMap - %s", err.Error())
		}
		_, err = s.dynamodbSvc.PutItem(&dynamodb.PutItemInput{
			TableName: aws.String("Recipe"),
			Item:      av,
		})
		if err != nil {
			return fmt.Errorf("Error in saveDevices, failed to put Record to DynamoDB - %s", err.Error())
		}
		//time.Sleep(50 * time.Millisecond)
	}
	return nil
}

func (a Activities) generateAndSaveTasks(s *sessCtx) (prepTaskS, error) {
	var rows int
	// only prep & task verbal and its text equivalent are saved.
	// Generate prep and tasks from Activities.
	ptS := a.GenerateTasks("T-"+s.pkey, s.recipe, s)
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
			return prepTaskS{}, fmt.Errorf("failed to put Record to DynamoDB, %v", err)
		}
		//time.Sleep(50 * time.Millisecond)
	}
	return ptS, nil
}

func (s *sessCtx) updateRecipe(r *RecipeT) error {
	//
	type pKey struct {
		PKey  string
		SortK float64
	}
	//var updateC expression.UpdateBuilder

	rId, err := strconv.Atoi(s.reqRId)
	if err != nil {
		return fmt.Errorf("Error: in updateRecipe converting reqRId  [%s] to int - %s", s.reqRId, err.Error())
	}
	pkey := pKey{PKey: "R-" + s.reqBkId, SortK: float64(rId)}
	fmt.Printf("updateREcipe PKEY %#v\n", pkey)
	av, err := dynamodbattribute.MarshalMap(&pkey)
	if err != nil {
		return fmt.Errorf("%s: %s", "Error in MarshalMap of recipeIdLookup", err.Error())
	}
	// update Part attribute
	updateC := expression.Set(expression.Name("Part"), expression.Value(r.Part))
	expr, err := expression.NewBuilder().WithUpdate(updateC).Build()
	if err != nil {
		//return prepTaskRec{}, fmt.Errorf("Error in Query of Tasks: " + err.Error())
		panic(err)
	}
	input := &dynamodb.UpdateItemInput{
		TableName:                 aws.String("Recipe"),
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

func (s *sessCtx) getTaskRecById() (alexaDialog, error) {

	var taskRec prepTaskRec
	pKey := "T-" + s.pkey
	keyC := expression.KeyEqual(expression.Key("PKey"), expression.Value(pKey)).And(expression.KeyEqual(expression.Key("SortK"), expression.Value(s.objRecId)))
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
	// TODO - should be GetItem not Query as we are providing the primary key however a future feature to display 3 records instead of one would user query.
	result, err := s.dynamodbSvc.Query(input)
	if err != nil {
		//return prepTaskRec{}, fmt.Errorf("Error in Query of Tasks: " + err.Error())
		return prepTaskRec{}, err
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
	fmt.Printf("taskRec in . getTaskRecById [%#v]\n ", taskRec)
	//
	// save to Session Context
	//
	s.eol = taskRec.EOL
	if len(taskRec.Part) > 0 {
		s.peol = taskRec.PEOL
		s.part = taskRec.Part
		s.next = taskRec.Next
		s.prev = taskRec.Prev
		s.pid = taskRec.PId
	}
	return taskRec, nil
}

// func (s *sessCtx) getTaskRecById() (alexaDialog, error) {

// 	var (
// 		taskRec prepTaskRec
// 	)
// 	pKey := "T-" + s.pkey
// 	keyC := expression.KeyEqual(expression.Key("PKey"), expression.Value(pKey)).And(expression.KeyEqual(expression.Key("SortK"), expression.Value(s.objRecId)))
// 	// startId = s.objRecId - 1
// 	// endId = s.objRecId + 2
// 	// keyC := expression.KeyEqual(expression.Key("PKey"), expression.Value(pKey)).And(expression.Between(expression.Name("SortK"), expression.Value(startId), expression.Value(endId)))
// 	expr, err := expression.NewBuilder().WithKeyCondition(keyC).Build()
// 	if err != nil {
// 		panic(err)
// 	}
// 	//
// 	// Table: Tasks - get current task based on task Id
// 	//
// 	input := &dynamodb.QueryInput{
// 		KeyConditionExpression:    expr.KeyCondition(),
// 		FilterExpression:          expr.Filter(),
// 		ExpressionAttributeNames:  expr.Names(),
// 		ExpressionAttributeValues: expr.Values(),
// 		//ProjectionExpression:      expr.Projection(),
// 	}
// 	input = input.SetTableName("Recipe").SetReturnConsumedCapacity("TOTAL").SetConsistentRead(false)
// 	//
// 	// TODO - should be GetItem not Query as we are providing the primary key however a future feature to display 3 records instead of one would user query.
// 	result, err := s.dynamodbSvc.Query(input)
// 	if err != nil {
// 		//return prepTaskRec{}, fmt.Errorf("Error in Query of Tasks: " + err.Error())
// 		panic(err)
// 	}
// 	if int(*result.Count) == 0 { //TODO - put this code back so it makes sense
// 		// this is caused by a goto operation exceeding EOL
// 		return prepTaskRec{}, fmt.Errorf("Error: %s [%s] ", "Internal error: no tasks found for recipe ", s.reqRName)
// 	}
// 	if int(*result.Count) > 1 {
// 		return prepTaskRec{}, fmt.Errorf("Error: more than 1 task returned from getNextRecordById")
// 	}
// 	err = dynamodbattribute.UnmarshalMap(result.Items, &taskRec)
// 	if err != nil {
// 		return nil, fmt.Errorf("Error: %s - %s", "in UnmarshalMap in getTaskRecById ", err.Error())
// 	}
// 	//
// 	// save to Session Context
// 	//
// 	s.eol = taskRec.EOL
// 	if len(taskRec.Part) > 0 {
// 		s.peol = taskRec.PEOL
// 		s.part = taskRec.Part
// 		s.next = taskRec.Next
// 		s.prev = taskRec.Prev
// 		s.pid = taskRec.PId
// 	}
// 	return taskRec, nil
// }

var recipeParts []string

func (s *sessCtx) recipeRSearch() (*RecipeT, error) {
	//
	// query on recipe name to get RecipeId and  book name
	//
	type pKey struct {
		PKey  string
		SortK float64
	}
	rId, err := strconv.Atoi(s.reqRId)
	if err != nil {
		return nil, fmt.Errorf("Error: in recipeRSearch converting reqRId  [%s] to int - %s", s.reqRId, err.Error())
	}
	pkey := pKey{PKey: "R-" + s.reqBkId, SortK: float64(rId)}
	av, err := dynamodbattribute.MarshalMap(&pkey)
	if err != nil {
		return nil, fmt.Errorf("%s: %s", "Error in MarshalMap of recipeIdLookup", err.Error())
	}
	input := &dynamodb.GetItemInput{
		Key:       av,
		TableName: aws.String("Recipe"),
	}
	input = input.SetTableName("Recipe").SetReturnConsumedCapacity("TOTAL").SetConsistentRead(false)
	//
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
		return nil, fmt.Errorf("%s: %s", "Error in GetItem of recipeRSearch", err.Error())
	}
	if len(result.Item) == 0 {
		return nil, fmt.Errorf("No Recipe record found for R-%s-%s", s.reqBkId, s.reqRId)
	}
	rec := &RecipeT{}
	err = dynamodbattribute.UnmarshalMap(result.Item, rec)
	if err != nil {
		return nil, fmt.Errorf("Error: in UnmarshalMaps of recipeRSearch [%s] err", s.reqRId, err.Error())
	}
	// populate session context fields
	s.reqRName = rec.RName
	s.index = rec.Index //TODO: is this required?
	err = s.bookNameLookup()
	if err != nil {
		s.reqBkName = ""
		return nil, err
	}
	s.dmsg = s.reqRName + " in " + s.reqBkName + " by " + s.authors
	s.vmsg = "sFound " + s.reqRName + " in " + s.reqBkName + " by " + s.authors
	s.vmsg += `What would you like to list?. Say "list containers" or "List Ingredients" or "start Cooking" or "cancel"`
	s.recipe = rec
	s.parts = rec.Part
	return rec, nil
}

func (s *sessCtx) keywordSearch() error {
	//
	// search for recipe by specifying ingredient and a category or sub-category.
	// data must exist in this table for each recipe. Data is populated as part of the base activity processig.
	//
	type searchRecT struct {
		PKey     string
		SortK    string `json:"SortK"`
		RName    string `json:"RName"`
		BkName   string `json:"BkName"`
		Authors  string `json:"Authors"`
		Quantity string `json:"Quantity"`
		Serves   string `json:"Srv"`
	}
	var (
		result   *dynamodb.QueryOutput
		allBooks bool
		err      error
	)
	// zero mChoice list
	s.mChoice = nil
	//
	if len(s.reqBkId) > 0 {
		// look for recipes in current book only
		kcond := expression.KeyEqual(expression.Key("PKey"), expression.Value(s.reqSearch))
		kcond = kcond.And(expression.KeyBeginsWith(expression.Key("SortK"), s.reqBkId+"-"))
		expr, err := expression.NewBuilder().WithKeyCondition(kcond).Build()
		if err != nil {
			return fmt.Errorf("Error: %s [%s] %s", "in NewBuilder in keywordSearch of ", s.reqSearch, err.Error())
		}
		input := &dynamodb.QueryInput{
			KeyConditionExpression:    expr.KeyCondition(),
			FilterExpression:          expr.Filter(),
			ExpressionAttributeNames:  expr.Names(),
			ExpressionAttributeValues: expr.Values(),
		}
		input = input.SetTableName("Ingredient").SetConsistentRead(false)
		//
		result, err = s.dynamodbSvc.Query(input)
		if err != nil {
			return fmt.Errorf("Error: %s [%s] %s", "in Query in keywordSearch of ", s.reqSearch, err.Error())
		}
		if int(*result.Count) == 0 {
			allBooks = true
		}
	}
	if len(s.reqBkId) == 0 || allBooks {
		// no active book or active book does not contain recipe type
		kcond := expression.KeyEqual(expression.Key("PKey"), expression.Value(s.reqSearch))
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
		input = input.SetTableName("Ingredient").SetConsistentRead(false)
		//
		result, err = s.dynamodbSvc.Query(input)
		if err != nil {
			return fmt.Errorf("Error: %s [%s] %s", "in Query in keywordSearch of ", s.reqBkId, err.Error())
		}
		if int(*result.Count) == 0 {
			switch allBooks {
			case true:
				return fmt.Errorf(`Recipe [%s] not found in [%s] or library. Please notify support`, s.reqRName, s.reqBkName)
			case false:
				return fmt.Errorf(`Recipe [%s] not found in library. Please notify support`, s.reqRName)
			}
		}
	}
	recS := make([]searchRecT, int(*result.Count))
	err = dynamodbattribute.UnmarshalListOfMaps(result.Items, &recS)
	if err != nil {
		return fmt.Errorf("Error: %s [%s] err", "in UnmarshalListMaps of keywordSearch ", s.reqRName, err.Error())
	}
	if allBooks {
		// case where active book did not contain recipe type so library searched.
		switch int(*result.Count) {
		case 0:
			s.vmsg = fmt.Sprintf("%s not found in [%s] and all other books. ", s.reqSearch, s.reqBkName)
			s.dmsg = fmt.Sprintf("%s not found in [%s] and all other books. ", s.reqSearch, s.reqBkName)
			//s.reqRId, s.reqRName, s.reqBkId, s.reqBkName = "", "", "", ""
		case 1:
			s.vmsg = fmt.Sprintf("%s not found in [%s], but was found in [%s]. Do you want to swap to this book?", s.reqSearch, s.reqBkName, recS[0].BkName)
			s.dmsg = fmt.Sprintf("%s not found in [%s], but was found in [%s]. Do you want to swap to this book?", s.reqSearch, s.reqBkName, recS[0].BkName)
			sortk := strings.Split(recS[0].SortK, "-")
			s.reqRId, s.reqRName, s.reqBkId, s.reqBkName, s.serves = sortk[1], recS[0].RName, sortk[0], recS[0].BkName, recS[0].Serves
		default:
			//s.makeSelect = true
			s.vmsg = fmt.Sprintf(`No %s recipes found in [%s] but where found in other books. Please see the display`, s.reqSearch, s.reqBkName)
			s.dmsg = fmt.Sprintf(`No %s recipes found in [%s], but were found in the following. Please select one. `, s.reqSearch, s.reqBkName)
			for i, v := range recS {
				sortk := strings.Split(v.SortK, "-")
				s.ddata += strconv.Itoa(i+1) + ": " + v.BkName + " by " + v.Authors + ". Quantity: " + v.Quantity + "\n "
				rec := mRecipeT{Id: i + 1, IngrdCat: v.PKey, RName: v.RName, RId: sortk[1], BkName: v.BkName, BkId: sortk[0], Authors: v.Authors, Quantity: v.Quantity, Serves: v.Serves}
				s.mChoice = append(s.mChoice, rec)
			}
		}
		return nil
	}
	//
	// result of seach within open book
	//
	switch int(*result.Count) {
	case 0:
		s.vmsg = fmt.Sprintf("No %s was found in any book. ", s.reqSearch)
		s.dmsg = fmt.Sprintf("No %s was found in any book. ", s.reqSearch)
	case 1:
		s.vmsg = "The following recipe, " + recS[0].RName + " in book " + recS[0].BkName + ` by authors ` + recS[0].Authors + ` contains the ingredient. You can list other ingredients or containers, utensils used in the recipe or list the prep tasks or you can start cooking`
		s.dmsg = "The following recipe, " + recS[0].RName + " in book " + recS[0].BkName + ` by authors ` + recS[0].Authors + ` contains the ingredient. You can list other ingredients or containers, utensils used in the recipe or list the prep tasks or you can start cooking`
		sortk := strings.Split(recS[0].SortK, "-")
		s.reqRId, s.reqRName, s.reqBkId, s.reqBkName, s.serves = sortk[1], recS[0].RName, sortk[0], recS[0].BkName, recS[0].Serves
		// set session ctx to display object menu (ingredient,containers,utensils) list
	default:
		//s.makeSelect = true
		s.vmsg = fmt.Sprintf("Multiple %s recipes found. See display", s.reqSearch)
		s.dmsg = fmt.Sprintf(`%s recipes. Please select one by saying "select [number-in-list]"\n`, s.reqSearch)
		for i, v := range recS {
			sortk := strings.Split(v.SortK, "-")
			s.ddata += strconv.Itoa(i+1) + ": " + v.BkName + " by " + v.Authors + ". Quantity: " + v.Quantity + "\n"
			rec := mRecipeT{Id: i + 1, RName: v.RName, RId: sortk[1], BkName: v.BkName, BkId: sortk[0], Authors: v.Authors, Quantity: v.Quantity, Serves: v.Serves}
			s.mChoice = append(s.mChoice, rec)
		}
	}
	return nil
}

func (s *sessCtx) generateSlotEntries() error {
	//
	type Index struct {
		PKey string `json:"PKey"`
	}
	type SrchKeyS Index
	var str string
	proj := expression.NamesList(expression.Name("PKey"))
	expr, err := expression.NewBuilder().WithProjection(proj).Build()
	if err != nil {
		return fmt.Errorf("%s", "Error in expression build in generateSlotEntries - "+err.Error())
	}
	// Build the query input parameters
	params := &dynamodb.ScanInput{
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		ProjectionExpression:      expr.Projection(),
		TableName:                 aws.String("Ingredient"),
	}
	result, err := s.dynamodbSvc.Scan(params)
	if err != nil {
		return fmt.Errorf("%s", "Error in scan of generateSlotEntries - "+err.Error())
	}
	srchKeys := make(map[string]bool)
	skey := make([]Index, int(*result.Count))
	err = dynamodbattribute.UnmarshalListOfMaps(result.Items, &skey)
	if err != nil {
		return fmt.Errorf("%s", "Error in UnmarshalMap of unit table: "+err.Error())
	}
	//
	f, err := os.OpenFile("slot.entries", os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		panic(err)
	}
	//
	write := func(PKey string) error {
		if !srchKeys[PKey] {
			srchKeys[PKey] = true
			if PKey[len(PKey)-1] != 's' {
				str = PKey + ",," + PKey + "s" + "\n"
			} else {
				str = PKey + ",," + "\n"
			}
			_, err := f.Write([]byte(str))
			if err != nil {
				return fmt.Errorf("Write error in generateSlotEntries - %s", err.Error())
			}
		}
		return nil
	}
	//
	for _, v := range skey {
		if len(v.PKey) > 2 && v.PKey[:2] != "C-" {
			write(v.PKey)
		}
	}
	skey = nil
	//
	//  merge s.indexRecs.PKey entries to skey and printout new slot entries to file slot.entries
	//
	fmt.Println()
	for _, v := range s.indexRecs {
		err = write(v.PKey)
		if err != nil {
			return err
		}
	}
	if err := f.Close(); err != nil {
		panic(err)
	}
	skey = nil
	return nil
}

func (s *sessCtx) recipeNameSearch() error {
	//
	// user "opens <book>". Alexa provides associated slot-type-id BkId value.
	//
	// used in recipe name
	type RecipeT_ struct {
		PKey  string
		SortK int
		RName string
	}
	var (
		expr expression.Expression
		err  error
	)

	kcond := expression.KeyEqual(expression.Key("RName"), expression.Value(s.reqRName))
	if len(s.reqBkId) > 0 {
		filter := expression.Equal(expression.Name("PKey"), expression.Value("R-"+s.reqBkId))
		expr, err = expression.NewBuilder().WithKeyCondition(kcond).WithFilter(filter).Build()
	} else {
		expr, err = expression.NewBuilder().WithKeyCondition(kcond).Build()
	}
	if err != nil {
		panic(err)
	}
	input := &dynamodb.QueryInput{
		KeyConditionExpression:    expr.KeyCondition(),
		IndexName:                 aws.String("RName-Key"),
		FilterExpression:          expr.Filter(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
	}
	input = input.SetTableName("Recipe").SetConsistentRead(false)
	// while BkId is unique we are using a GSI so must use Query (I presume)
	result, err := s.dynamodbSvc.Query(input)
	if err != nil {
		return fmt.Errorf("Error: %s [%s] - %s", "in Query in recipeNameSearch of ", s.reqRName, err.Error())
	}
	if int(*result.Count) == 0 {
		return fmt.Errorf("No recipe found in recipeNameSearch, for rname [%s]", s.reqRName)
	}
	// define a slice of struct as Query expects to return 1 or more rows so the slice represents a row
	// and we ue unmarshallistofmaps to handle a batch like select
	recS := make([]RecipeT_, int(*result.Count))
	err = dynamodbattribute.UnmarshalListOfMaps(result.Items, &recS)
	if err != nil {
		return fmt.Errorf("Error: %s [%s] %s", "in UnmarshalMaps in recipeNameSearch ", s.reqRName, err.Error())
	}
	switch len(recS) {
	case 1:
		// single recipe-book found
		s.reqBkId = recS[0].PKey[2:] // trim prefix "R-"
		s.reqRId = strconv.Itoa(recS[0].SortK)
		s.reqRName = recS[0].RName
		//
		// populate Session Context Part data for a recipe
		//
		_, err = s.recipeRSearch()
		if err != nil {
			return fmt.Errorf("Error: %s [%s] %s", "in recipeNameSearch of recipeRSearch ", s.reqRName, err.Error())
		}
		//
		// populate session context Book related data
		//
		err = s.bookNameLookup()
		if err != nil {
			return fmt.Errorf("Error: %s [%s] %s", "in UnmarrecipeNameSearch of shalMaps bookNameLookup ", s.reqRName, err.Error())
		}
		//
		s.dmsg = s.reqRName + " in " + s.reqBkName + " by " + s.authors
		s.vmsg = "yFound " + s.reqRName + " in " + s.reqBkName + " by " + s.authors
		s.vmsg += `What would you like to list?. Say "list container" or "List Ingredient" or "List Prep tasks" or "start Cooking" or "cancel"`
	default:
		// more than one recipe-book found
		//s.makeSelect = true
		s.dmsg = `Recipe appears in more than one book. Please make a selection from the list below. Say "select number\n" `
		s.vmsg = `the recipe appears in more than one book. I will recite the first 6. Please say "next" to hear each one and "select" to choose or "cancel" to exit\n" `
		for i := 0; i < len(recS); i++ {
			s.reqBkId = recS[i].PKey[2:] // trim prefix "R-"
			s.reqRId = strconv.Itoa(recS[i].SortK)
			s.reqRName = recS[i].RName
			err = s.bookNameLookup()
			s.ddata += strconv.Itoa(i+1) + ". " + s.reqRName + " in " + s.reqBkName + " by " + s.authors + "\n"
			rec := mRecipeT{Id: i + 1, RName: s.reqRName, RId: s.reqRId, BkName: s.reqBkName, BkId: s.reqBkId, Authors: s.authors, Serves: s.serves}
			s.mChoice = append(s.mChoice, rec)
		}
		// clear session context because mutli records means no-one record is active until user selects one.
		s.reqBkId, s.reqRName, s.reqBkName, s.reqRId, s.authors, s.serves = "", "", "", "", "", ""
	}
	//
	return nil
}

func (s *sessCtx) bookNameLookup() error {
	//
	// user "opens <book>". Alexa provides associated slot-type-id BkId value.
	//
	type recT struct {
		PKey    string
		Authors []string `json:"Authors"`
	}

	flatten := func(w []string) string {
		var a string
		for i, v := range w {
			switch i {
			case 0:
				a = v[strings.LastIndex(v, " ")+1:]
			case 1, 2, 3:
				a += ", " + v[strings.LastIndex(v, " ")+1:]
			default:
				break
			}
		}
		return a
	}

	kcond := expression.KeyEqual(expression.Key("BkId"), expression.Value(s.reqBkId)) // must internally converts bookid string to int
	proj := expression.NamesList(expression.Name("Authors"), expression.Name("PKey"))
	expr, err := expression.NewBuilder().WithKeyCondition(kcond).WithProjection(proj).Build()
	if err != nil {
		return fmt.Errorf("Error: %s [%s] - %s", "in NewBuilder in bookNameLookup, bookId ", s.reqBkId, err.Error())
	}
	input := &dynamodb.QueryInput{
		KeyConditionExpression:    expr.KeyCondition(),
		IndexName:                 aws.String("BkId-BkName"),
		FilterExpression:          expr.Filter(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		ProjectionExpression:      expr.Projection(),
	}
	input = input.SetTableName("Recipe").SetConsistentRead(false)
	// while BkId is unique we are using a GSI so must use Query (I presume)
	result, err := s.dynamodbSvc.Query(input)
	if err != nil {
		return fmt.Errorf("Error: %s [%s] %s", "in Query in bookNameLookup of ", s.reqBkId, err.Error())
	}
	if int(*result.Count) == 0 {
		return fmt.Errorf("No data found in bookNameLookup, for bookId [%s]", s.reqBkId)
	}
	if int(*result.Count) > 1 {
		return fmt.Errorf("Internal error in bookNameLookup. %s [%s]", "More than one book found for bookId ", s.reqBkId)
	}
	// define a slice of struct as Query expects to return 1 or more rows so the slice represents a row
	// and we ue unmarshallistofmaps to handle a batch like select

	rec := make([]recT, int(*result.Count))
	err = dynamodbattribute.UnmarshalListOfMaps(result.Items, &rec)
	if err != nil {
		return fmt.Errorf("Error: %s [%s] %s", "in UnmarshalMaps in bookNameLookup ", s.reqRName, err.Error())
	}
	s.authors = flatten(rec[0].Authors)
	s.authorS = rec[0].Authors
	s.reqBkName = rec[0].PKey[3:] // trim "BK-" prefix
	fmt.Println("in bookNameLookup: Opened book ", s.reqBkName)
	return nil
}
