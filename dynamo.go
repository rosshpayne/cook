package main

import (
	_ "encoding/json"
	"fmt"
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
	Text   string `json="txt"`
	Verbal string `json="vbl"`
	EOL    int
}

func (ct ctRec) Alexa() dialog {
	return dialog{ct.Verbal, ct.Text, ct.EOL}
}

type mRecT struct {
	Id       int
	IngrdCat string
	RName    string
	RId      string
	BkName   string
	BkId     string
	Authors  string
	Quantity string
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

func (pt prepTaskRec) Alexa() dialog {
	return dialog{pt.Verbal, pt.Text, pt.EOL}
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

	return ctS[0], nil
}

func (s sessCtx) getContainerRecById() (alexaDialog, error) {

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

func (a Activities) IndexIngd(svc *dynamodb.DynamoDB, bkid string, bkname string, rname string, rid string, cat string, subcat string, authors string) error {
	//	   	err = aa.IndexIngd(s.dynamodbSvc,        s.reqBkId,   s.reqBkName, s . .reqRName,  s.reqRId, cat, authors)
	type indexRecT struct {
		PKey     string
		SortK    string
		PreQual  string
		PostQual string
		Quantity string
		BkName   string
		RName    string
		Authors  string
	}

	doNotIndex, err := loadNonIngredientsMap(svc) // map[string]bool
	if err != nil {
		panic(err)
	}
	var indexRecS []indexRecT

	saveIngdIndex := func() error {
		for _, v := range indexRecS {
			av, err := dynamodbattribute.MarshalMap(v)
			if err != nil {
				panic(fmt.Sprintf("failed in IndexIngd to marshal Record, %v", err))
			}
			_, err = svc.PutItem(&dynamodb.PutItemInput{
				TableName: aws.String("Ingredient"),
				Item:      av,
			})
			if err != nil {
				return fmt.Errorf("failed in IndexIngd to PutItem into Ingredient table - %v", err)
			}
			time.Sleep(50 * time.Millisecond)
		}
		indexRecS = nil // free memory. Probably redundant as its local to this func so once func exists memory would be freed anyway.
		return nil
	}
	//
	// populate indexT type, indexRecS
	//
	// var totalgrams int
	// for _, ap := range a {
	// 	if len(ap.Ingredient) > 0 {
	// 		if len(ap.Measure.Unit) > 0 {
	// 			switch ap.Measure.Unit {
	// 				case "g" : grams+= ap.Measure.Quantity
	// 				case "kg": grams+= ap.Measure.Quantity*1000
	// 			}
	// 		}
	// 	}
	// }
	fmt.Printf("doNotIndex %#v\n", doNotIndex)
	for _, ap := range a {

		if len(ap.Ingredient) > 0 {
			ap.Ingredient = strings.ToLower(ap.Ingredient)
			if !doNotIndex[ap.Ingredient] {
				// ingredient is indexable. Populate index record.
				for i, v := range []string{cat, subcat} {
					// ingrd-cat and ingrd-subcat
					for k := range []int{1, 2} {
						// ingrd-cat and qualifier-ingrd-cat and ingrd-subcat, qualifier-ingrd-subcat
						if len(v) == 0 && i == 1 {
							// subcat is not defined
							break
						}
						if ap.Ingredient == strings.ToLower(v) {
							// if ingredient name same as cat/subcat then index under cat/subcat name
							ap.Ingredient = ""
						}
						irec := indexRecT{}
						irec.PreQual = ap.QualiferIngrd
						irec.PostQual = ap.IngrdQualifer
						if len(ap.Measure.Size) > 0 {
							irec.Quantity = ap.Measure.Quantity + " " + ap.Measure.Size
						} else {
							irec.Quantity = ap.Measure.Quantity + ap.Measure.Unit
						}
						if len(v) == 0 && i == 0 {
							// take cat from last word in recipe title
							cat = rname[strings.LastIndex(rname, " ")+1:]
						}
						if k == 0 {
							irec.PKey = strings.ToLower(ap.Ingredient + " " + v)
						} else {
							irec.PKey = strings.ToLower(ap.QualiferIngrd + " " + ap.Ingredient + " " + v)
						}
						irec.SortK = bkid + "-" + rid
						irec.RName = rname
						irec.BkName = bkname
						irec.Authors = authors
						indexRecS = append(indexRecS, irec)
					}
				}
			}
		}
	}
	err = saveIngdIndex()
	return err
}

func (a Activities) saveTasks(s *sessCtx) (prepTaskS, error) {
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
			return []prepTaskRec{}, fmt.Errorf("failed to put Record to DynamoDB, %v", err)
		}
		time.Sleep(50 * time.Millisecond)
	}
	return ptS, nil
}

func (s sessCtx) updateSession() (int, error) {
	// state data that must be maintained across sessions
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
			fmt.Printf("recid in updateSession is: %s\n", recid_)
			updateC = expression.Add(expression.Name(recid_), expression.Value(s.updateAdd))
		}
	}

	updateC = updateC.Set(expression.Name("EOL"), expression.Value(s.eol)) //eol from get-RecId() associated with each Object

	if len(s.reqRName) > 0 {
		updateC = updateC.Set(expression.Name("Rname"), expression.Value(s.reqRName))
	} else {
		updateC = updateC.Set(expression.Name("Rname"), expression.Value(""))
	}
	if len(s.reqRId) > 0 {
		updateC = updateC.Set(expression.Name("RId"), expression.Value(s.reqRId))
	} else {
		updateC = updateC.Set(expression.Name("RId"), expression.Value(""))
	}
	// will clear Book entries provided execution paths bypasses mergeAndValidate func.
	if len(s.reqBkName) > 0 && s.reqBkName != "0" {
		updateC = updateC.Set(expression.Name("BKname"), expression.Value(s.reqBkName))
	} else if s.reqBkName != "0" {
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
	} else {
		updateC = updateC.Set(expression.Name("Oper"), expression.Value(""))
	}
	if len(s.object) > 0 {
		updateC = updateC.Set(expression.Name("Obj"), expression.Value(s.object)) // ingredient,task,container,utensil
	} else {
		updateC = updateC.Set(expression.Name("Obj"), expression.Value(""))
	}
	if s.questionId > 0 {
		updateC = updateC.Set(expression.Name("Qid"), expression.Value(s.questionId))
	} else {
		updateC = updateC.Set(expression.Name("Qid"), expression.Value(0))
	}
	if len(s.dbatchNum) > 0 {
		updateC = updateC.Set(expression.Name("DBat"), expression.Value(s.dbatchNum))
	}
	if len(s.mChoice) > 0 {
		updateC = updateC.Set(expression.Name("RnLst"), expression.Value(s.mChoice)) //recipename
	}
	if len(s.dmsg) > 0 {
		updateC = updateC.Set(expression.Name("Dmsg"), expression.Value(s.dmsg))
		updateC = updateC.Set(expression.Name("DData"), expression.Value(s.ddata))
	} else {
		updateC = updateC.Set(expression.Name("Dmsg"), expression.Value(""))
		updateC = updateC.Set(expression.Name("DData"), expression.Value(""))
	}
	if s.closeBook {
		updateC = updateC.Set(expression.Name("closeB"), expression.Value(true))
	} else {
		updateC = updateC.Set(expression.Name("closeB"), expression.Value(false))
	}
	if len(s.vmsg) > 0 {
		updateC = updateC.Set(expression.Name("Vmsg"), expression.Value(s.vmsg))
	} else {
		updateC = updateC.Set(expression.Name("Vmsg"), expression.Value(""))
	}
	updateC = updateC.Set(expression.Name("Select"), expression.Value(s.makeSelect)) // make a selection
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

func (s sessCtx) getTaskRecById() (alexaDialog, error) {

	var taskRec prepTaskRec
	pKey := "T-" + s.reqBkId + "-" + s.reqRId
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
	return taskRec, nil
}

func (s *sessCtx) recipeRSearch() error {
	//
	// query on recipe name to get RecipeId and optionally book name and Id if not requested
	//
	type pKey struct {
		PKey  string
		SortK float64
	}
	type recT struct {
		RName  string `json:"RName"`
		Cat    string `json:"cat"`
		Subcat string `json:"subcat"`
	}
	rId, err := strconv.Atoi(s.reqRId)
	if err != nil {
		return fmt.Errorf("Error: in recipeRSearch converting reqId  [%s] to int - %s", s.reqRId, err.Error())
	}
	pkey := pKey{PKey: "R-" + s.reqBkId, SortK: float64(rId)}
	av, err := dynamodbattribute.MarshalMap(&pkey)
	if err != nil {
		return fmt.Errorf("%s: %s", "Error in MarshalMap of recipeIdLookup", err.Error())
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
		return fmt.Errorf("%s: %s", "Error in GetItem of recipeRSearch", err.Error())
	}
	if len(result.Item) == 0 {
		return fmt.Errorf("Error: %s [%s] %s [%s] - %s", "No recipe found in recipeRSearch for book Id", s.reqBkId, " and recipe Id ", s.reqRId, err.Error())
	}
	rec := recT{}
	err = dynamodbattribute.UnmarshalMap(result.Item, &rec)
	if err != nil {
		return fmt.Errorf("Error: in UnmarshalMaps of recipeRSearch [%s] err", s.reqRId, err.Error())
	}
	// populate session context fields
	s.reqRName = rec.RName
	s.cat = rec.Cat
	s.subcat = rec.Subcat
	err = s.bookNameLookup()
	if err != nil {
		s.reqBkName = ""
		return err
	}
	s.dmsg = s.reqRName + " in " + s.reqBkName + " by " + s.authors
	s.vmsg = "sFound " + s.reqRName + " in " + s.reqBkName + " by " + s.authors
	s.vmsg += `What would you like to list?. Say "list container" or "List Ingredient" or "List Prep tasks" or "start Cooking" or "cancel"`
	return nil
}

func (s *sessCtx) ingredientSearch() error {
	//
	// search for recipe by specifying ingredient and a category or sub-category.
	// data must exist in this table for each recipe. Data is populated as part of the base activity processig.
	//
	type dynoRecT struct {
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
		fmt.Printf("in ingredientSearch..in book [%s] for [%s]\n", s.reqBkId, s.reqIngrdCat)
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
		fmt.Println("allbooks...1")
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
	recS := make([]dynoRecT, int(*result.Count))
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
				rec := mRecT{Id: i + 1, IngrdCat: v.PKey, RName: v.RName, RId: sortk[1], BkName: v.BkName, BkId: sortk[0], Authors: v.Authors, Quantity: v.Quantity}
				s.mChoice = append(s.mChoice, rec)
			}
			s.reqRId, s.reqRName, s.reqBkId, s.reqBkName = "", "", "", ""
		}
		return nil
	}
	// result of active book returning 1 record and library search
	fmt.Printf("count of book... %d\n", int(*result.Count))
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
			rec := mRecT{Id: i + 1, RName: v.RName, RId: sortk[1], BkName: v.BkName, BkId: sortk[0], Authors: v.Authors, Quantity: v.Quantity}
			s.mChoice = append(s.mChoice, rec)
		}
		s.reqRId, s.reqRName, s.reqBkId, s.reqBkName = "", "", "", ""
	}
	return nil
}

func (s *sessCtx) recipeNameSearch() error {
	//
	// user "opens <book>". Alexa provides associated slot-type-id BkId value.
	//
	// used in recipe name
	type dynoRecT struct {
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
		return fmt.Errorf("No data found in recipeNameSearch, for rname [%s]", s.reqRName)
	}
	// define a slice of struct as Query expects to return 1 or more rows so the slice represents a row
	// and we ue unmarshallistofmaps to handle a batch like select
	recS := make([]dynoRecT, int(*result.Count))
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
			rec := mRecT{Id: i + 1, RName: s.reqRName, RId: s.reqRId, BkName: s.reqBkName, BkId: s.reqBkId, Authors: s.authors}
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
