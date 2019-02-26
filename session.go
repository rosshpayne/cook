package main

import (
	"fmt"
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

// Session table record
// only items from session context that need to be preserved between sessions are persisted.
type stateRec struct {
	// InputRequest fields
	DT      string
	Path    string // inputEvent.Path
	Param   string
	Request string // PathItem[0]:
	ReqType int    // curReqType in session context - used to drive output format
	//
	Obj     string // Object - to which operation (listing) apply
	BkId    string // Book Id
	BkName  string // Book name - saves a lookup under some circumstances
	RName   string // Recipe name - saves a lookup under some circumstances
	OpenBk  string // requested open book (nill if book closed or no open book requested)
	Serves  string
	SwpBkNm string
	SwpBkId string
	RId     string // Recipe Id
	Qid     int    // Question id
	RecId   [4]int // current record in object list. (SortK in Recipe table)
	Ver     string
	EOL     int // last RecId of current list. Used to determine when last record is reached or exceeded in the case of goto operation
	Dmsg    string
	Vmsg    string
	DData   string
	Authors string // comma separated string of authors
	//
	Ingredients IngredientT // contains complete ingredients listing
	//
	// search
	//
	Search     string
	RecipeList RecipeListT // recipes menu, recipe-part menu,
	//
	// select
	//
	SelCtx selectCtxT // select a recipe or other which is (ingred, task, container, utensil)
	SelId  int        // select choice
	//
	// Recipe Part related data
	//
	Parts PartS  `json:"Parts"` // PartT.Idx - short name for Recipe Part
	Part  string `json:"Part"`
	//
	InstructionData InstructionS `json:"I"`
	ShowObjMenu     bool
	//ContainerData   ContainerS   `json:"C"` // no need to save this data. It is never sourced from state data.
	//
	// DispObjectMenu  bool
	// DispIngredients bool
	// DispContainers  bool
	// DispPartMenu    bool
}

type stateStack []stateRec

func (s stateStack) pop() *stateRec {
	st := s[len(s)-1]
	return &st
}

// func (s *sessCtx) setDisplay(ls *stateRec) {
// 	s.dispObjectMenu = ls.DispObjectMenu
// 	s.dispIngredients = ls.DispIngredients
// 	s.dispContainers = ls.DispContainers
// 	s.dispPartMenu = ls.DispPartMenu
// }

func (s *sessCtx) getState() (*stateRec, error) {
	//
	// Table:  Sessions
	//
	type pKey struct {
		Sid string
	}
	fmt.Println("entered getState..")
	pkey := pKey{s.sessionId}
	av, err := dynamodbattribute.MarshalMap(&pkey)
	input := &dynamodb.GetItemInput{
		Key:       av,
		TableName: aws.String("Sessions"),
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
		return nil, err
	}
	if len(result.Item) == 0 {
		//
		s.newSession = true
		return &stateRec{}, err
	}
	//
	type stateItemT struct {
		Sid   string     `json:"Sid"`
		State stateStack `json:"state"`
	}

	stateItem := stateItemT{}
	err = dynamodbattribute.UnmarshalMap(result.Item, &stateItem)
	if err != nil {
		return nil, err
	}
	lastState := stateItem.State.pop()
	s.state = stateItem.State
	return lastState, nil
}

func (s *sessCtx) setState(ls *stateRec) {
	//return staterow.state.pop(), nil
	if s.eol == 0 {
		s.eol = ls.EOL
	}
	if len(s.reqBkId) == 0 {
		s.reqBkId = ls.BkId
	}
	if len(s.reqBkName) == 0 {
		s.reqBkName = ls.BkName
	}
	if len(s.reqRName) == 0 {
		s.reqRName = ls.RName
	}
	if len(s.reqRId) == 0 {
		s.reqRId = ls.RId
	}
	if len(ls.Authors) > 0 {
		s.authors = ls.Authors
	}
	fmt.Printf("reqVersion: [%s]\n", s.reqVersion)
	fmt.Println("len(s.reqVersion) = ", len(s.reqVersion))
	if len(s.reqVersion) == 0 {
		s.reqVersion = ls.Ver
	}
	//
	// opened book
	//
	if len(s.reqOpenBk) == 0 && len(ls.OpenBk) > 0 {
		s.reqOpenBk = ls.OpenBk
		id := strings.Split(ls.OpenBk, "|")
		s.reqBkId, s.reqBkName = id[0], id[1]
		s.authors = id[2]
		fmt.Println("set BookId: ", s.reqBkId, " from ls.OpenBk")
	}
	// object rec Id
	//
	s.recId = ls.RecId
	if len(s.object) == 0 {
		s.object = ls.Obj
	}
	//
	// Recipe Part related data
	//
	if len(s.part) == 0 {
		s.part = ls.Part
	}
	if len(s.parts) == 0 {
		s.parts = ls.Parts
		s.displayData = ls.Parts
	}
	if len(ls.InstructionData) > 0 {
		dd := ls.InstructionData
		if s.peol == 0 && len(dd) > 0 {
			s.peol = dd[ls.RecId[objectMap[ls.Obj]]].PEOL
		}
		if s.pid > 0 && len(dd) > 0 {
			s.pid = dd[ls.RecId[objectMap[ls.Obj]]].PID
		}
		if ls.EOL > 0 && len(dd) > 0 {
			s.eol = dd[ls.RecId[objectMap[ls.Obj]]].EOL
		}
		s.displayData = dd
	}
	if len(ls.Ingredients) > 0 {
		s.displayData = ls.Ingredients
	}
	// if len(ls.ContainerData) > 0 {
	// 	s.displayData = ls.ContainerData
	// }
	if len(ls.RecipeList) > 0 {
		s.recipeList = ls.RecipeList
		s.displayData = s.recipeList
	}
	// if dd, ok := s.displayData.(InstructionS); ok {
	// 	lsd := ls.DisplayData.(InstructionS)
	// 	if len(dd) == 0 {
	// 		dd = lsd
	// 	}
	// }
	//
	// determine select context
	//
	if s.selId > 0 {
		switch {
		case ls.SelCtx == 0 && len(s.reqRName) == 0 && (ls.Request == "search" || ls.Request == "recipe"):
			s.selCtx = ctxRecipeMenu
			s.displayData = objMenu
			//s.dispObjectMenu = true
		case ls.SelCtx == 0 || (ls.SelCtx == ctxRecipeMenu && len(s.reqRName) > 0):
			s.selCtx = ctxObjectMenu
		case s.request == "select" && len(ls.Parts) > 0 && len(ls.Part) == 0:
			s.selCtx = ctxPartMenu
		}
	}
	//
	// gen primary key - used for most dyamo accesses
	//
	s.pkey = s.reqBkId + "-" + s.reqRId
	fmt.Println("reqVersion = ", s.reqVersion)
	if len(s.reqVersion) > 0 {
		if s.reqVersion != "0" {
			fmt.Println("..including version id")
			s.pkey += "-" + s.reqVersion
		} else {
			s.reqVersion = ""
		}
	}
	fmt.Println("PKEY = ", s.pkey)
	// determine if recIds need to be reset to 1
	if len(s.reqVersion) > 0 {
		if len(ls.Ver) == 0 {
			s.reset = true
		} else if len(ls.Ver) > 0 && s.reqVersion != ls.Ver {
			s.reset = true
		}
	}
	s.showObjMenu = ls.ShowObjMenu
	if s.showObjMenu && len(ls.Ingredients) == 0 {
		fmt.Println("in setSession: displaying object menu is set")
		s.displayData = objMenu
	}
	return
}

func (s *sessCtx) pushState() (*stateRec, error) {
	// equivalent to a push operation for a stack (state data in this case)
	type pKey struct {
		Sid string
	}

	var (
		sr      stateRec
		updateC expression.UpdateBuilder
	)
	fmt.Println("Entered pushState..")
	// copy statevfrom session context
	//sr.Path = s.path
	//sr.Param = s.param
	sr.DT = time.Now().Format("Jan 2 15:04:05")
	sr.RId = s.reqRId // Recipe Id
	fmt.Println("BkId = ", s.reqBkId)
	sr.BkId = s.reqBkId     // Book Id
	sr.BkName = s.reqBkName // Book name - saves a lookup under some circumstances
	sr.RName = s.reqRName   // Recipe name - saves a lookup under some circumstances
	sr.SwpBkNm = s.swapBkName
	sr.SwpBkId = s.swapBkId
	sr.Request = s.request // Request e.g.next, prev, repeat, modify)
	sr.ReqType = s.curReqType
	sr.Serves = s.serves
	sr.Qid = s.questionId // Question id	for k,v:=range objectMap {
	sr.Obj = s.object     // Object - to which operation (listing) apply
	sr.Ingredients = s.ingrdList
	sr.Ver = s.reqVersion
	sr.EOL = s.eol // last RecId of current list. Used to determine when last record is reached or exceeded in the case of goto operation
	sr.Dmsg = s.dmsg
	sr.Vmsg = s.vmsg
	sr.DData = s.ddata
	sr.OpenBk = s.reqOpenBk
	sr.Authors = s.authors
	//
	// Record id across objects
	//
	if s.reset {
		sr.RecId = [4]int{0, 0, 0, 0}
	} else {
		sr.RecId = s.recId
	}
	// search
	//
	sr.Search = s.reqSearch
	sr.RecipeList = s.recipeList
	//
	// select
	//
	sr.SelCtx = s.selCtx // select a recipe or other which is (ingred, task, container, utensil)
	sr.SelId = s.selId   //
	//
	// Recipe Part related data
	//
	sr.Parts = s.parts
	sr.Part = s.part
	// sr.Next = s.next
	// sr.Prev = s.prev
	// sr.PEOL = s.peol
	// sr.PId = s.pid
	// sr.EOL = s.eol
	// sr.DispObjectMenu = s.dispObjectMenu
	// sr.DispIngredients = s.dispIngredients
	// sr.DispContainers = s.dispContainers
	// sr.DispPartMenu = s.dispPartMenu
	if d, ok := s.displayData.(InstructionS); ok {
		sr.InstructionData = d
	}
	sr.ShowObjMenu = s.showObjMenu
	// if d, ok := s.displayData.(ContainerS); ok {
	// 	sr.InstructionData = d
	// }
	//
	State := make(stateStack, 1)
	State[0] = sr
	s.state = append(s.state, sr)
	//
	t := time.Now()
	t.Add(time.Hour * 24 * 1)
	updateC = expression.Set(expression.Name("Epoch"), expression.Value(t.Unix()))
	if s.newSession {
		sr.RecId = [4]int{}
		updateC = updateC.Set(expression.Name("state"), expression.Value(State))
		s.newSession = false
	} else if len(s.state) > 12 {
		updateC = updateC.Set(expression.Name("state"), expression.Value(s.state[len(s.state)-8:]))
	} else {
		updateC = updateC.Set(expression.Name("state"), expression.ListAppend(expression.Name("state"), expression.Value(State)))
	}
	expr, err := expression.NewBuilder().WithUpdate(updateC).Build()
	if err != nil {
		return nil, err
	}
	pkey := pKey{Sid: s.sessionId}
	av, err := dynamodbattribute.MarshalMap(&pkey)
	input := &dynamodb.UpdateItemInput{
		TableName:                 aws.String("Sessions"),
		Key:                       av, // accets []map[]*attributeValues so must use marshal not expression
		UpdateExpression:          expr.Update(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
	}
	_, err = s.dynamodbSvc.UpdateItem(input)
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
		return nil, err
	}
	return &sr, nil
}

func (s *sessCtx) updateState() error {

	type pKey struct {
		Sid string
	}
	var updateC expression.UpdateBuilder
	//
	// update RecId attribute of latest state item
	//
	fmt.Println("entered updateState..")
	t := time.Now()
	t.Add(time.Hour * 24 * 1)
	updateC = expression.Set(expression.Name("Epoch"), expression.Value(t.Unix()))
	//
	// for current state
	//
	atribute := fmt.Sprintf("state[%d].RecId", len(s.state)-1)
	updateC = updateC.Set(expression.Name(atribute), expression.Value(s.recId))
	atribute = fmt.Sprintf("state[%d].DT", len(s.state)-1)
	updateC = updateC.Set(expression.Name(atribute), expression.Value(time.Now().Format("Jan 2 15:04:05")))
	atribute = fmt.Sprintf("state[%d].Request", len(s.state)-1)
	updateC = updateC.Set(expression.Name(atribute), expression.Value(s.request))
	//
	// for previous states - upto object menu
	//
	// fmt.Println("len(s.state)  ", len(s.state))
	// if len(s.state)-2 > 0 {
	// 	atribute := fmt.Sprintf("state[%d].RecId", len(s.state)-2)
	// 	updateC = updateC.Set(expression.Name(atribute), expression.Value(s.recId))
	// }
	// //back to object choice menu - if recipe part's involved
	// fmt.Println(" back to object menu")
	// if len(s.state)-3 > 0 && (s.request == "select" || s.request == "search") && len(s.reqRName) > 0 {
	// 	atribute := fmt.Sprintf("state[%d].RecId", len(s.state)-3)
	// 	updateC = updateC.Set(expression.Name(atribute), expression.Value(s.recId))
	// }
	//
	expr, err := expression.NewBuilder().WithUpdate(updateC).Build()
	pkey := pKey{Sid: s.sessionId}
	av, err := dynamodbattribute.MarshalMap(&pkey)

	input := &dynamodb.UpdateItemInput{
		TableName:                 aws.String("Sessions"),
		Key:                       av, // accets []map[]*attributeValues so must use marshal not expression
		UpdateExpression:          expr.Update(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		ReturnValues:              aws.String("UPDATED_NEW"),
	}
	_, err = s.dynamodbSvc.UpdateItem(input)
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
	fmt.Println("updateState in sessions.go")
	return nil
}

func (s *sessCtx) popState() error {
	//
	// removes last entry in state attribute of session item.
	//  populates session context with state data from the new last entry
	//  (which was the penultimate entry before deletion)
	//
	type pKey struct {
		Sid string
	}
	var (
		sr    *stateRec
		State stateStack
	)

	fmt.Println("Entered popState()")
	var updateC expression.UpdateBuilder
	// get current state if not already sourced
	if len(s.state) == 0 {
		s.getState()
	}
	if len(s.state) > 1 {
		//
		// pop state and persist to dynamo
		//
		State = s.state[:len(s.state)-1]
		s.state = State[:]

		t := time.Now()
		t.Add(time.Hour * 24 * 1)
		updateC = expression.Set(expression.Name("Epoch"), expression.Value(t.Unix()))
		// rewrite all but last state entry - this is how we delete from a list in dynamo. Here the list represents the state stack.
		updateC = updateC.Set(expression.Name("state"), expression.Value(State))
		expr, err := expression.NewBuilder().WithUpdate(updateC).Build()
		//
		pkey := pKey{Sid: s.sessionId}
		av, err := dynamodbattribute.MarshalMap(&pkey)

		input := &dynamodb.UpdateItemInput{
			TableName:                 aws.String("Sessions"),
			Key:                       av, // accets []map[]*attributeValues so must use marshal not expression
			UpdateExpression:          expr.Update(),
			ExpressionAttributeNames:  expr.Names(),
			ExpressionAttributeValues: expr.Values(),
			ReturnValues:              aws.String("UPDATED_NEW"),
		}
		_, err = s.dynamodbSvc.UpdateItem(input)
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
		// pop last entry from session context state
		//s.state = s.state[:len(s.state)-1]
		sr = State.pop()
	} else {
		sr = s.state.pop() //[len(s.state)-1]
	}
	//

	// transfer state data to session context
	//s.path = sr.Path
	//s.param = sr.Param

	s.reqRId = sr.RId       // Recipe Id
	s.reqBkId = sr.BkId     // Book Id
	s.reqBkName = sr.BkName // Book name - saves a lookup under some circumstances
	s.reqRName = sr.RName   // Recipe name - saves a lookup under some circumstances
	s.swapBkName = sr.SwpBkNm
	s.swapBkId = sr.SwpBkId
	s.request = sr.Request // Request e.g.next, prev, repeat, modify)
	s.curReqType = sr.ReqType
	s.serves = sr.Serves
	s.questionId = sr.Qid // Question id	for k,v:=range objectMap {
	s.object = sr.Obj     // Object - to which operation (listing) apply
	s.recId = sr.RecId    //s.recId     // current record in object list. (SortK in Recipe table)
	s.reqVersion = sr.Ver
	s.eol = sr.EOL // last RecId of current list. Used to determine when last record is reached or exceeded in the case of goto operation
	s.dmsg = sr.Dmsg
	s.vmsg = sr.Vmsg
	s.ddata = sr.DData
	s.authors = sr.Authors
	//
	if len(sr.OpenBk) > 0 {
		bk := strings.Split(sr.OpenBk, "|")
		s.reqBkId, s.reqBkName, s.authors = bk[0], bk[1], bk[3]
		s.authorS = strings.Split(s.authors, ",")
	}
	//
	s.ingrdList = sr.Ingredients
	//
	// Record id across objects
	//
	s.recId = sr.RecId
	// search
	//
	s.reqSearch = sr.Search
	s.recipeList = sr.RecipeList
	//
	// select
	//
	s.selCtx = sr.SelCtx // select a recipe or other which is (ingred, task, container, utensil)
	s.selId = sr.SelId   //
	//
	// Recipe Part related data
	//
	s.parts = sr.Parts
	s.part = sr.Part
	//s.peol = sr.PEOL
	//s.pid = sr.PId
	//
	// Display Menu choices
	//
	// s.dispObjectMenu = sr.DispObjectMenu
	// s.dispIngredients = sr.DispIngredients
	// s.dispContainers = sr.DispContainers
	// s.dispPartMenu = sr.DispPartMenu
	//
	s.displayData = s.parts
	if len(sr.InstructionData) > 0 {
		s.displayData = sr.InstructionData
	}
	if len(sr.Ingredients) > 0 {
		s.displayData = sr.Ingredients
	}
	if len(sr.RecipeList) > 0 {
		s.displayData = sr.RecipeList
	}
	if sr.ShowObjMenu {
		s.displayData = objMenu
		//		s.showObjMenu = sr.ShowObjMenu
	}
	return nil
}
