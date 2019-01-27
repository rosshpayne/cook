package main

import (
	_ "encoding/json"
	"fmt"
	"os"
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
	return dialog{ct.Verbal, ct.Text, ct.EOL}
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
}

type indexRecipeT struct {
	PKey     string
	SortK    string
	Quantity string
	BkName   string
	RName    string
	Authors  string
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
	Part string `json:"PT"`   // part index name
	Next int    `json:"nxt"`  // next SortK (recId)
	Prev int    `json:"prv"`  // previous SortK (recId) when in part mode as opposed to full recipe mode
	// not persisted
	taskp *PerformT
}

func (pt prepTaskRec) Alexa() dialog {
	return dialog{pt.Verbal, pt.Text, pt.EOL}
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

func (s *sessCtx) getContainerRecById() (alexaDialog, error) {
	type pKey struct {
		PKey  string
		SortK float64
	}
	ctrec := ctRec{}
	pkey := pKey{PKey: "C-" + s.pkey, SortK: float64(s.recId)}
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

func loadNonIngredientsMap(svc *dynamodb.DynamoDB) (map[string]bool, error) {
	type recT struct {
		SortK string
	}
	//kcond := expression.KeyBeginsWith(expression.Key("PKey"), "NI-")
	kcond := expression.KeyEqual(expression.Key("PKey"), expression.Value("NI"))
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
	result, err := svc.Query(input)
	if err != nil {
		return nil, fmt.Errorf("Error: %s  %s", "in Query in loadNonIngredients  ", err.Error())
	}
	if int(*result.Count) == 0 {
		fmt.Println("No Data Returned in Query in loadNonIngredients  ")
	}
	ingdS := make([]recT, int(*result.Count))
	err = dynamodbattribute.UnmarshalListOfMaps(result.Items, &ingdS)
	if err != nil {
		return nil, fmt.Errorf("Error: %s", "in UnmarshalListMaps of loadNonIngredients  ", err.Error())
	}
	notIngrd := make(map[string]bool)
	for _, v := range ingdS {
		ingrd := v.SortK
		if _, ok := notIngrd[ingrd]; !ok {
			notIngrd[ingrd] = true
		}
	}
	return notIngrd, nil

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
		irec := indexRecipeT{SortK: s.reqBkId + "-" + s.reqRId, BkName: s.reqBkName, RName: s.reqRName, Authors: s.authors}
		irec.PKey = strings.TrimRight(strings.TrimLeft(strings.ToLower(entry), " "), " ")
		indexRecS = append(indexRecS, irec)
		indexRow[irec.PKey] = true
	}

	makeIndexRecs := func(entry string, ap *Activity) {
		// for each index property value, add to index
		irec := indexRecipeT{SortK: s.reqBkId + "-" + s.reqRId, BkName: s.reqBkName, RName: s.reqRName, Authors: s.authors}
		irec.PKey = entry
		irec.Quantity = ap.String()
		if !indexRow[irec.PKey] {
			// only append unique values..
			indexRow[irec.PKey] = true
			indexRecS = append(indexRecS, irec)
		}
	}

	AddEntry := func(entry string) {
		entry = strings.Replace(entry, "-", " ", 1)
		entry = strings.TrimRight(strings.TrimLeft(strings.ToLower(entry), " "), " ")
		// for each word in index entry find associated activity ingredient
		var indexed bool
		w := entry
		if a, ok := labelM[strings.ToLower(w)]; ok {
			delete(indexRow, entry)
			makeIndexRecs(entry, a)
			indexed = true
		}
		if !indexed {
			if a, ok := ingrdM[strings.ToLower(w)]; ok {
				delete(indexRow, entry)
				makeIndexRecs(entry, a)
				indexed = true
			}
		}
		if !indexed {
			for _, w := range strings.Split(entry, " ") {
				if a, ok := labelM[strings.ToLower(w)]; ok {
					delete(indexRow, entry)
					makeIndexRecs(entry, a)
					break
				}
				if a, ok := ingrdM[strings.ToLower(w)]; ok {
					delete(indexRow, entry)
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
	//    a b c =>
	//   1      "a b c"
	//   2     "a" "b" "c"		SplitN(,-1)
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
	//TODO: determine if running as Lambda or standalone executable
	// Before saving index to table generate slot entries
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
		return fmt.Errorf("Error: in recipeRSearch converting reqId  [%s] to int - %s", s.reqRId, err.Error())
	}
	pkey := pKey{PKey: "R-" + s.reqBkId, SortK: float64(rId)}
	fmt.Printf("updateREcipe PKEY %#v\n", pkey)
	av, err := dynamodbattribute.MarshalMap(&pkey)
	if err != nil {
		return fmt.Errorf("%s: %s", "Error in MarshalMap of recipeIdLookup", err.Error())
	}

	updateC := expression.Set(expression.Name("Start"), expression.Value(r.Start))
	updateC = updateC.Set(expression.Name("Part"), expression.Value(r.Part))
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

func (s *sessCtx) getTaskRecById() (alexaDialog, error) {

	var taskRec prepTaskRec
	pKey := "T-" + s.pkey
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
	// TODO - should be GetItem not Query as we are providing the primary key however a future feature to display 3 records instead of one would user query.
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
	fmt.Printf("taskRec ingetTaskRecById [%#v]\n ", taskRec)
	return taskRec, nil
}

var recipeParts []string

func (s *sessCtx) recipeRSearch() (*RecipeT, error) {
	//
	// query on recipe name to get RecipeId and optionally book name and Id if not requested
	//
	type pKey struct {
		PKey  string
		SortK float64
	}

	rId, err := strconv.Atoi(s.reqRId)
	if err != nil {
		return nil, fmt.Errorf("Error: in recipeRSearch converting reqId  [%s] to int - %s", s.reqRId, err.Error())
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
	s.index = rec.Index
	err = s.bookNameLookup()
	if err != nil {
		s.reqBkName = ""
		return nil, err
	}
	s.dmsg = s.reqRName + " in " + s.reqBkName + " by " + s.authors
	s.vmsg = "sFound " + s.reqRName + " in " + s.reqBkName + " by " + s.authors
	s.vmsg += `What would you like to list?. Say "list container" or "List Ingredient" or "List Prep tasks" or "start Cooking" or "cancel"`
	s.recipe = rec
	return rec, nil
}

func (s *sessCtx) ingredientSearch() error {
	//
	// search for recipe by specifying ingredient and a category or sub-category.
	// data must exist in this table for each recipe. Data is populated as part of the base activity processig.
	//
	type dynoRecipeT struct {
		PKey     string
		SortK    string `json:"SortK"`
		RName    string `json:"RName"`
		BkName   string `json:"BkName"`
		Authors  string `json:"Authors"`
		Quantity string
	}
	var (
		result   *dynamodb.QueryOutput
		allBooks bool
		err      error
	)
	if len(s.reqBkId) > 0 {
		// look for recipes in current book only - reqIngrdCat in lower case before searching
		kcond := expression.KeyEqual(expression.Key("PKey"), expression.Value(s.reqIngrdCat))
		kcond = kcond.And(expression.KeyBeginsWith(expression.Key("SortK"), s.reqBkId+"-"))
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
			return fmt.Errorf("Error: %s [%s] %s", "in Query in ingredientLookup of ", s.reqBkId, err.Error())
		}
		if int(*result.Count) == 0 {
			allBooks = true
		}
	}
	if len(s.reqBkId) == 0 || allBooks {
		// no active book or active book does not contain recipe type
		kcond := expression.KeyEqual(expression.Key("PKey"), expression.Value(s.reqIngrdCat))
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
			return fmt.Errorf("Error: %s [%s] %s", "in Query in ingredientSearch of ", s.reqBkId, err.Error())
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
	recS := make([]dynoRecipeT, int(*result.Count))
	err = dynamodbattribute.UnmarshalListOfMaps(result.Items, &recS)
	if err != nil {
		return fmt.Errorf("Error: %s [%s] err", "in UnmarshalListMaps of ingredientSearch ", s.reqRName, err.Error())
	}
	if allBooks {
		// case where active book did not contain recipe type so library searched.
		switch int(*result.Count) {
		case 0:
			s.vmsg = fmt.Sprintf("%s not found in [%s] and all other books. ", s.reqIngrdCat, s.reqBkName)
			s.dmsg = fmt.Sprintf("%s not found in [%s] and all other books. ", s.reqIngrdCat, s.reqBkName)
			//s.reqRId, s.reqRName, s.reqBkId, s.reqBkName = "", "", "", ""
		case 1:
			s.vmsg = fmt.Sprintf("%s not found in [%s], but was found in [%s]. ", s.reqIngrdCat, s.reqBkName, recS[0].BkName)
			s.dmsg = fmt.Sprintf("%s not found in [%s], but was found in [%s]. ", s.reqIngrdCat, s.reqBkName, recS[0].BkName)
			sortk := strings.Split(recS[0].SortK, "-")
			s.reqRId, s.reqRName, s.reqBkId, s.reqBkName = sortk[1], recS[0].RName, sortk[0], recS[0].BkName
		default:
			s.makeSelect = true
			s.vmsg = fmt.Sprintf(`No %s recipes found in [%s] but where found in severalother  books. Please see list and select one by saying "select" followed by its number`, s.reqIngrdCat, s.reqBkName)
			s.dmsg = fmt.Sprintf(`No %s recipes found in [%s], but were found in the following. Please select one. `, s.reqIngrdCat)
			for i, v := range recS {
				sortk := strings.Split(v.SortK, "-")
				s.ddata += strconv.Itoa(i+1) + ": " + v.BkName + " by " + v.Authors + ". Quantity: " + v.Quantity + "\n "
				rec := mRecipeT{Id: i + 1, IngrdCat: v.PKey, RName: v.RName, RId: sortk[1], BkName: v.BkName, BkId: sortk[0], Authors: v.Authors, Quantity: v.Quantity}
				s.mChoice = append(s.mChoice, rec)
			}
			s.reqRId, s.reqRName, s.reqBkId, s.reqBkName = "", "", "", ""
		}
		return nil
	}
	// result of active book returning 1 record and library search
	switch int(*result.Count) {
	case 0:
		s.vmsg = fmt.Sprintf("No %s found in any recipe book. ", s.reqIngrdCat)
		s.dmsg = fmt.Sprintf("No %s found in any recipe book. ", s.reqIngrdCat)
	case 1:
		s.vmsg = "The following recipe, " + recS[0].RName + " in book " + recS[0].BkName + ` by authors ` + recS[0].Authors + ` contains the ingredient. You can list other ingredients or containers, utensils used in the recipe or list the prep tasks or you can start cooking`
		s.dmsg = "The following recipe, " + recS[0].RName + " in book " + recS[0].BkName + ` by authors ` + recS[0].Authors + ` contains the ingredient. You can list other ingredients or containers, utensils used in the recipe or list the prep tasks or you can start cooking`
		sortk := strings.Split(recS[0].SortK, "-")
		s.reqRId, s.reqRName, s.reqBkId, s.reqBkName = sortk[1], recS[0].RName, sortk[0], recS[0].BkName
	default:
		s.makeSelect = true
		s.vmsg = fmt.Sprintf("Multiple %s recipes found. See display", s.reqIngrdCat)
		s.dmsg = fmt.Sprintf(`%s recipes. Please select one by saying "select [number-in-list]"\n`, s.reqIngrdCat)
		for i, v := range recS {
			sortk := strings.Split(v.SortK, "-")
			s.ddata += strconv.Itoa(i+1) + ": " + v.BkName + " by " + v.Authors + ". Quantity: " + v.Quantity + "\n"
			rec := mRecipeT{Id: i + 1, RName: v.RName, RId: sortk[1], BkName: v.BkName, BkId: sortk[0], Authors: v.Authors, Quantity: v.Quantity}
			s.mChoice = append(s.mChoice, rec)
		}
		s.reqRId, s.reqRName, s.reqBkId, s.reqBkName = "", "", "", ""
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
	type dynoRecipeT struct {
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
	recS := make([]dynoRecipeT, int(*result.Count))
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
		err = s.bookNameLookup()
		//
		s.dmsg = s.reqRName + " in " + s.reqBkName + " by " + s.authors
		s.vmsg = "yFound " + s.reqRName + " in " + s.reqBkName + " by " + s.authors
		s.vmsg += `What would you like to list?. Say "list container" or "List Ingredient" or "List Prep tasks" or "start Cooking" or "cancel"`
	default:
		// more than one recipe-book found
		s.makeSelect = true
		s.dmsg = `Recipe appears in more than one book. Please make a selection from the list below. Say "select number\n" `
		s.vmsg = `the recipe appears in more than one book. I will recite the first 6. Please say "next" to hear each one and "select" to choose or "cancel" to exit\n" `
		for i := 0; i < len(recS); i++ {
			s.reqBkId = recS[i].PKey[2:] // trim prefix "R-"
			s.reqRId = strconv.Itoa(recS[i].SortK)
			s.reqRName = recS[i].RName
			err = s.bookNameLookup()
			s.ddata += strconv.Itoa(i+1) + ". " + s.reqRName + " in " + s.reqBkName + " by " + s.authors + "\n"
			rec := mRecipeT{Id: i + 1, RName: s.reqRName, RId: s.reqRId, BkName: s.reqBkName, BkId: s.reqBkId, Authors: s.authors}
			s.mChoice = append(s.mChoice, rec)
		}
		// zero session context because mutli records means no-one record is active until user selects one.
		s.reqBkId, s.reqRName, s.reqBkName, s.reqRId, s.authors = "", "", "", "", ""
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
	var authors string
	for i, v := range rec[0].Authors {
		switch i {
		case 0:
			authors = v[strings.LastIndex(v, " ")+1:]
		case 1:
			authors += ", " + v[strings.LastIndex(v, " ")+1:]
			break
		}
	}
	s.authors = authors
	s.authorS = rec[0].Authors
	s.reqBkName = rec[0].PKey[3:] // trim "BK-" prefix
	return nil
}
