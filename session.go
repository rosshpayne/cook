package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/cook/global"

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
	OpenBk  BookT  // requested open book (nill if book closed or no open book requested) <bkname>|<author,author..>
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
	Ingredients IngredientT `json:"ingrd"` // contains complete ingredients listing
	//
	// search
	//
	Search     string
	RecipeList RecipeListT // recipes menu, recipe-part menu,
	//
	// select
	//
	SelCtx selectCtxT // select a recipe or other which is (ingred, task, container, utensil) in that order.
	SelId  int        // menu choice as supplied from Alexa APL - starts at 1 and goes to max selection choice. Hence selId = 0 for no choice
	//
	// Recipe Part related data
	//
	Parts   PartS  `json:"Parts"` // PartT.Idx - short name for Recipe Part
	Part    string `json:"Part"`
	CThread int    `json:"CThrd"` // current thread
	OThread int    `json:"OThrd"` // other active thread - only two threads currently catered for. seems unlikely there would be more.
	//InstId  int    `json:"Iid"`   // instruction index - copy of Id in InstructionData
	//
	//InstructionData InstructionS `json:"I"`
	InstructionData Threads `json:"I"`
	ShowObjMenu     bool
	MenuL           menuList
	//
	DispCtr *DispContainerT `json:"Dctr"`
	CtSize  int             `json:"Dim"`
	ScaleF  float64         `json:"SF"`
	//
	//Display apldisplayT `json:"AplD"` // Welcome display - contains registered
	Welcome *WelcomeT `json:"Welc"`
}

type pKey struct {
	PKey string `json:"PKey"`
}

type stateStack []stateRec

type stateItemT struct {
	PKey  string     `json:"PKey"`
	State stateStack `json:"state"`
}

func (s stateStack) pop() *stateRec {
	st := s[len(s)-1]
	return &st
}

func (ls *stateRec) activeRecipe() bool {
	if len(ls.InstructionData) > 0 || len(ls.RecipeList) > 0 || ls.ShowObjMenu || len(ls.Ingredients) > 0 {
		return true
	}
	return false
}

func (s *sessCtx) getState() (*stateRec, error) {
	//
	fmt.Println("Enter getState() ..s.userId  ", s.userId)
	combineReqIds := s.alxReqId + "|" + s.invkReqId
	fmt.Println("Request IDs: ", combineReqIds)
	t := time.Now()
	t.Add(time.Hour * 52 * 1)
	updateC := expression.Set(expression.Name("Epoch"), expression.Value(t.Unix()))
	updateC = updateC.Set(expression.Name("RIds"), expression.Value(combineReqIds))
	notCond := expression.Not(expression.Equal(expression.Name("RIds"), expression.Value(combineReqIds)))
	expr, err := expression.NewBuilder().WithUpdate(updateC).WithCondition(notCond).Build()
	if err != nil {
		return nil, err
	}
	pkey := pKey{PKey: s.userId}
	av, err := dynamodbattribute.MarshalMap(&pkey)
	input := &dynamodb.UpdateItemInput{
		TableName:                 aws.String("Sessions"),
		Key:                       av, // accepts []map[]*attributeValues so must use marshal not expression
		UpdateExpression:          expr.Update(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		ConditionExpression:       expr.Condition(),
		ReturnValues:              aws.String("ALL_NEW"),
	}
	result, err := s.dynamodbSvc.UpdateItem(input)
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
	if len(result.Attributes) == 0 {
		fmt.Println("getState.... 0 item found")
		panic(fmt.Errorf("Panic: duplicate request detected [%s]", combineReqIds))
	}
	//
	type stateItemT struct {
		State stateStack `json:"state"`
	}

	stateItem := stateItemT{}
	err = dynamodbattribute.UnmarshalMap(result.Attributes, &stateItem)
	if err != nil {
		fmt.Println("error in UnmarshalMap")
		return nil, err
	}
	if len(stateItem.State) == 0 {
		//
		fmt.Println("no state data..")
		s.newSession = true
		return &stateRec{}, nil
	}
	lastState := stateItem.State.pop()
	s.state = stateItem.State
	//
	return lastState, nil
}

// func (s *sessCtx) getState() (*stateRec, error) {
// 	//
// 	// Table:  Sessions
// 	//
// 	type pKey struct {
// 		PKey string
// 	}
// 	pkey := pKey{s.userId}
// 	av, err := dynamodbattribute.MarshalMap(&pkey)
// 	if err != nil {
// 		return nil, err
// 	}
// 	input := &dynamodb.GetItemInput{
// 		Key:       av,
// 		TableName: aws.String("Sessions"),
// 	}
// 	result, err := s.dynamodbSvc.GetItem(input)
// 	if err != nil {
// 		if aerr, ok := err.(awserr.Error); ok {
// 			switch aerr.Code() {
// 			case dynamodb.ErrCodeProvisionedThroughputExceededException:
// 				fmt.Println(dynamodb.ErrCodeProvisionedThroughputExceededException, aerr.Error())
// 			case dynamodb.ErrCodeResourceNotFoundException:
// 				fmt.Println(dynamodb.ErrCodeResourceNotFoundException, aerr.Error())
// 			//case dynamodb.ErrCodeRequestLimitExceeded:
// 			//	fmt.Println(dynamodb.ErrCodeRequestLimitExceeded, aerr.Error())
// 			case dynamodb.ErrCodeInternalServerError:
// 				fmt.Println(dynamodb.ErrCodeInternalServerError, aerr.Error())
// 			default:
// 				fmt.Println(aerr.Error())
// 			}
// 		} else {
// 			// Print the error, cast err to awserr.Error to get the Code and
// 			// Message from an error.
// 			fmt.Println(err.Error())
// 		}
// 		return nil, err
// 	}
// 	if len(result.Item) == 0 {
// 		//
// 		fmt.Println("getState.... 0 rows found")
// 		s.newSession = true
// 		return &stateRec{}, nil
// 	}
// 	//
// 	type stateItemT struct {
// 		PKey   string     `json:"PKey"`
// 		State stateStack `json:"state"`
// 	}

// 	stateItem := stateItemT{}
// 	err = dynamodbattribute.UnmarshalMap(result.Item, &stateItem)
// 	if err != nil {
// 		return nil, err
// 	}
// 	lastState := stateItem.State.pop()
// 	s.state = stateItem.State
// 	//
// 	return lastState, nil
// }

func (s *sessCtx) setState(ls *stateRec) {
	//return staterow.state.pop(), nil
	// DONT SET display here. This must set just before pushState.
	//if ls.Display.Type != 0 {
	// 	s.display = &ls.Display
	// 	switch s.display.Type {
	// 	case WELCOME:
	// 		var w WelcomeT
	// 		s.displayData = w
	// 	}
	// }
	fmt.Printf("** in setState()")
	if len(ls.Request) > 0 {
		s.lastreq = ls.Request
	}
	if s.eol == 0 {
		s.eol = ls.EOL
	}
	s.cThread = ls.CThread
	s.oThread = ls.OThread
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
	if len(s.authors) == 0 && len(ls.Authors) > 0 {
		s.authors = ls.Authors
		s.authorS = strings.Split(s.authors, ",")
	}
	if ls.Qid > 0 {
		s.questionId = ls.Qid
	}
	if len(s.reqVersion) == 0 {
		s.reqVersion = ls.Ver
	}
	if s.ctSize == 0 && ls.CtSize > 0 {
		s.ctSize = ls.CtSize
	}
	s.showObjMenu = ls.ShowObjMenu
	//
	//  alexa launch
	//
	if s.request == "start" {
		if s.selId == 0 && ls.SelId > 0 {
			s.selId = ls.SelId
		}
		if ls.ShowObjMenu {
			s.showObjMenu = true
			s.displayData = objMenu
			s.selId = 0
		}
	}
	//
	if len(s.object) == 0 && len(ls.Obj) > 0 {
		s.object = ls.Obj
	}
	if s.selCtx == 0 && ls.SelCtx > 0 {
		s.selCtx = ls.SelCtx
	}
	if len(ls.Ingredients) > 0 {
		s.ingrdList = ls.Ingredients
	}
	if len(s.reqSearch) == 0 && len(ls.Search) > 0 {
		s.reqSearch = ls.Search
	}
	//
	// open book
	//
	if len(s.reqOpenBk) == 0 && len(ls.OpenBk) > 0 {
		s.reqOpenBk = ls.OpenBk
		id := strings.Split(string(ls.OpenBk), "|")
		s.reqBkId, s.reqBkName = id[0], id[1]
		s.authors = id[2]
		fmt.Println("set ls.OPenBk: ", ls.OpenBk)
		//s.displayData = ls.OpenBk
		// if s.request == "start" {
		// 	// redirect
		// 	s.request = "book"
		// }
	}
	// object rec Id
	//
	s.recId = ls.RecId
	if len(s.object) == 0 && s.questionId == 0 {
		s.object = ls.Obj
	}
	//
	// Recipe Part related data
	//
	if len(s.part) == 0 {
		s.part = ls.Part
	}

	if len(ls.InstructionData) > 0 {
		dd := ls.InstructionData
		// if s.peol == 0 && len(dd) > 0 && dd[ls.CThread].Id > 0 {
		// 	s.peol = dd[ls.CThread].Instructions[dd[ls.CThread].Id-1].PEOL //[ls.RecId[objectMap[ls.Obj]]].PEOL
		// }
		// if s.pid > 0 && len(dd) > 0 && dd[ls.CThread].Id > 0 {
		// 	s.pid = dd[ls.CThread].Instructions[dd[ls.CThread].Id-1].PID
		// }
		if ls.EOL > 0 && len(dd) > 0 && dd[ls.CThread].Id > 0 {
			s.eol = dd[ls.CThread].Instructions[dd[ls.CThread].Id-1].EOL
		}
		s.displayData = dd
	}
	if len(ls.Ingredients) > 0 {
		s.displayData = ls.Ingredients
		fmt.Println("** setState: s.displayData set to Ingredients")
	}
	// if len(ls.ContainerData) > 0 {
	// 	s.displayData = ls.ContainerData
	// }
	if len(ls.RecipeList) > 0 {
		s.recipeList = ls.RecipeList
		s.displayData = s.recipeList
	}
	//
	// if s.request == "search" {
	// 	switch ls.request {
	// 	case "start" :
	// 		s.displayData=
	// 	case
	// 	}
	// }
	fmt.Println("in SetState: selId = ", s.selId)
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
	//s.showObjMenu = ls.ShowObjMenu
	if s.showObjMenu { // && len(ls.Ingredients) == 0 && len(ls.RecipeList) == 0 {
		fmt.Println("in setSession: displaying object menu is set")
		s.showObjMenu = true
		s.displayData = objMenu
	}
	if len(ls.MenuL) > 0 {
		s.menuL = ls.MenuL
	}
	//s.dispCtr = &ls.DispCtr
	if ls.DispCtr == nil {
		fmt.Printf("getState: ls.DispCtr is nil\n")
	} else {
		fmt.Printf("getState: ls.DispCtr %#v\n", *(ls.DispCtr))
	}
	if ls.DispCtr != nil { //&& !s.showObjMenu {
		s.dispCtr = ls.DispCtr
		if s.displayData == nil {
			s.displayData = s.dispCtr
		}
	}
	if s.dispCtr == nil {
		fmt.Printf("getState: s.dispCtr is nil\n")
	} else {
		fmt.Printf("getState: s.dispCtr %#v\n", *(s.dispCtr))
	}
	if ls.ScaleF == 0 {
		global.SetScale(1.0)
	} else {
		global.SetScale(ls.ScaleF)
	}
	if s.scalef == 0 && global.GetScale() > 0.0 {
		s.scalef = global.GetScale()
	}
	//
	// initial request is always start but above logic checks for current state to sets displayData accordingly
	if s.displayData != nil {
		fmt.Println("** setState: s.displayData is set")
	}
	//
	if s.request != "select" {
		s.selId = ls.SelId
	}
	// if s.selId > 0 {
	// 	switch {
	// 	case ls.SelCtx == 0 && len(ls.Search) > 0 && ls.Request == "search":
	// 		s.selCtx = ctxRecipeMenu
	// 		fmt.Println("in SetState: selCtx = ctxRecipeMenu")
	// 	}
	// 	//s.displayData = objMenu
	// 	//s.dispObjectMenu = true

	// 	// 	case ls.Obj == "container":
	// 	// 		fmt.Println(" container so set selCTx, SelId")
	// 	// 		s.selCtx = ls.SelCtx
	// 	// 		s.selId = ls.SelId
	// 	// 		s.object = ls.Obj
	// 	// 	}
	//}
	if len(s.parts) == 0 && len(ls.Parts) > 0 {
		s.part = ls.Part
		s.parts = ls.Parts
		//s.displayData = ls.Parts
	}
	if ls.SelCtx == ctxPartMenu && len(ls.Part) == 0 {
		// active Parts menu but no selection maade so display part menu
		s.part = ls.Part
		s.parts = ls.Parts
		s.displayData = ls.Parts
	}
	//
	if (((s.request == "start" || s.request == "list") && s.selCtx == 0) || (s.request == "book" || s.request == "close" || s.request == "search")) && s.displayData == nil {
		var err error
		// if this is a genuine start with no previous state
		fmt.Printf("in setState:  welcome display %#v ", ls.Welcome)
		s.displayData = ls.Welcome
		// always check for books = don't rely on cached result even if it reasonably current
		s.bkids, err = s.getUserBooks()
		if err != nil {
			panic(err)
		}
	}
	//
}

// func (s *sessCtx) incrSelectCtx(ls *stateRec) {
// 	//
// 	//  moves "select context" (selCtx) forward based on last session state
// 	//
// 	fmt.Println("in incrSelectCtx: selId = ", s.selId)
// 	if s.selId > 0 {
// 		fmt.Println("Before selCtx = ", s.selCtx)
// 		fmt.Println("Before selId = ", s.selId)
// 		fmt.Println("Before object = ", s.object)
// 		switch {

// 		// case s.request == "select" && len(ls.Parts) > 0 && len(ls.Part) == 0:
// 		// 	s.selCtx = ctxPartMenu
// 		// 	fmt.Println("in SetState: selCtx = ctxPartMenu")
// 		// case s.request == "select" && s.selId == len(s.menuL):
// 		// 	s.selCtx = ctxObjectMenu // if Part Menu is required that is determined in the selId item.
// 		// 	fmt.Println("in SetState: selCtx = ctxPartMenu")
// 		// case ls.SelCtx == ctxObjectMenu && s.request == "scale" && len(ls.Ingredients) > 0:
// 		// 	s.selCtx = ctxObjectMenu
// 		// 	s.ingrdList = ls.Ingredients
// 		// 	fmt.Println("in setState: s.selCtx = ", s.selCtx)
// 		// case ls.SelCtx == ctxObjectMenu && len(ls.Ingredients) > 0:
// 		// 	s.selCtx = ctxObjectMenu
// 		// 	s.ingrdList = ls.Ingredients
// 		// 	fmt.Println("in setState: s.selCtx = ", s.selCtx)
// 		case ls.SelCtx == ctxRecipeMenu && len(s.reqRName) > 0:
// 			s.selCtx = ctxObjectMenu
// 			fmt.Println("in SetState: selCtx = ctxObjectMenu")
// 		}
// 		fmt.Println("After selCtx = ", s.selCtx)
// 		fmt.Println("After selId = ", s.selId)
// 		fmt.Println("After object = ", s.object)

// 	}

// }

func (s *sessCtx) pushState() (*stateRec, error) {
	// equivalent to a push operation for a stack (state data in this case)
	type pKey struct {
		PKey string
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
	sr.RId = s.reqRId       // Recipe Id
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
	if d, ok := s.displayData.(Threads); ok {
		sr.InstructionData = d
	}
	sr.CThread = s.cThread
	sr.OThread = s.oThread
	sr.ShowObjMenu = s.showObjMenu
	//sr.ObjMenu = s.ObjMenu
	if len(s.menuL) > 0 {
		sr.MenuL = s.menuL
	}
	if s.dispCtr == nil {
		fmt.Printf("pushState: s.dispCtr is nil\n")
	} else {
		fmt.Printf("pushState: s.dispCtr %#v\n", s.dispCtr)
	}
	if s.dispCtr != nil {
		//sr.DispCtr = *(s.dispCtr)
		sr.DispCtr = s.dispCtr
	}
	sr.ScaleF = global.GetScale()
	//
	// if s.display != nil {
	// 	sr.Display = *(s.display)
	// }
	if s.welcome != nil {
		sr.Welcome = s.welcome
	}
	//
	State := make(stateStack, 1)
	State[0] = sr
	// add to session context state
	s.state = append(s.state, sr)
	//
	t := time.Now()
	t.Add(time.Hour * 52 * 1)
	updateC = expression.Set(expression.Name("Epoch"), expression.Value(t.Unix()))
	//updateC = expression.Set(expression.Name("arqid"), expression.Value(s.alxReqId))
	//updateC = expression.Set(expression.Name("irqid"), expression.Value(s.invkReqId))
	if s.newSession {
		//.RecId = [4]int{}
		//s.state = State[:]
		updateC = updateC.Set(expression.Name("state"), expression.Value(State))
		s.newSession = false
	} else if len(s.state) > 12 {
		fmt.Println("> 12.. : ", State)
		updateC = updateC.Set(expression.Name("state"), expression.Value(s.state[len(s.state)-8:]))
	} else {
		updateC = updateC.Set(expression.Name("state"), expression.ListAppend(expression.Name("state"), expression.Value(State)))
	}
	expr, err := expression.NewBuilder().WithUpdate(updateC).Build()
	if err != nil {
		return nil, err
	}
	pkey := pKey{PKey: s.userId}
	av, err := dynamodbattribute.MarshalMap(&pkey)
	input := &dynamodb.UpdateItemInput{
		TableName:                 aws.String("Sessions"),
		Key:                       av, // accepts []map[]*attributeValues so must use marshal not expression
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
	fmt.Println("Exit pushState....")
	return &sr, nil
}

// type srecT struct {
// 	PKey string     `json:"PKey"`
// 	Sr   []stateRec `json:state"`
// }

func (s *sessCtx) updateState() error {

	type pKey struct {
		PKey string
	}
	var updateC expression.UpdateBuilder
	var atribute string
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
	if len(s.state) == 0 {
		// this case for new session. No UserId in session table so no state.
		panic(fmt.Errorf("s.state not set in UpdateState()"))
	}
	// for close book op only - shrink state slice down to 1
	// if len(s.CloseBkName) > 0 { . // errors with ValidationException: Invalid UpdateExpression: Two document paths overlap with each other; must remove or rewrite one of these paths; path one: [state], path two: [state, [0], MenuL]
	// 	State := s.state[:]
	// 	updateC = expression.Set(expression.Name("state"), expression.Value(State))
	// }
	if len(s.menuL) > 0 {
		atribute = fmt.Sprintf("state[%d].MenuL", len(s.state)-1)
		updateC = updateC.Set(expression.Name(atribute), expression.Value(s.menuL))
	}
	fmt.Println("About to update ScaleF..")
	// if scale changes then history must change to new value upto but not including last ingredients display or end of state list.

	// change state upto and including objMenu
	for scale, i := global.GetScale(), len(s.state)-1; i > 0; i-- {
		fmt.Println("About to update ScaleF..2   i ", i)

		atribute = fmt.Sprintf("state[%d].SF", i)
		updateC = updateC.Set(expression.Name(atribute), expression.Value(scale))
		fmt.Println(atribute, scale)
		//
		atribute = fmt.Sprintf("state[%d].Dim", i)
		updateC = updateC.Set(expression.Name(atribute), expression.Value(s.ctSize))
		//
		if s.dispCtr != nil {
			atribute = fmt.Sprintf("state[%d].Dctr", i)
			updateC = updateC.Set(expression.Name(atribute), expression.Value(s.dispCtr))
		}
		//
		atribute = fmt.Sprintf("state[%d].RecId", i)
		updateC = updateC.Set(expression.Name(atribute), expression.Value(s.recId))
		//
		atribute = fmt.Sprintf("state[%d].CThrd", i)
		updateC = updateC.Set(expression.Name(atribute), expression.Value(s.cThread))
		atribute = fmt.Sprintf("state[%d].OThrd", i)
		updateC = updateC.Set(expression.Name(atribute), expression.Value(s.oThread))
		//
		if len(s.part) > 0 {
			atribute = fmt.Sprintf("state[%d].Part", i)
			updateC = updateC.Set(expression.Name(atribute), expression.Value(s.part))
		}
		if s.state[i].ShowObjMenu {
			break
		}
	}
	//
	if s.openBkChange {
		atribute = fmt.Sprintf("state[%d].OpenBk", len(s.state)-1)
		updateC = updateC.Set(expression.Name(atribute), expression.Value(s.reqOpenBk))
		// when Bkids change
		atribute = fmt.Sprintf("state[%d].Welc", len(s.state)-1)
		updateC = updateC.Set(expression.Name(atribute), expression.Value(*(s.welcome)))
	}
	if len(s.state[len(s.state)-1].InstructionData) > 0 {
		atribute = fmt.Sprintf("state[%d].I[%d].id", len(s.state)-1, s.cThread)
		updateC = updateC.Set(expression.Name(atribute), expression.Value(s.recId[objectMap[task_]]))
	}
	if len(s.state[len(s.state)-1].Ingredients) > 0 && len(s.ingrdList) > 0 {
		atribute = fmt.Sprintf("state[%d].ingrd", len(s.state)-1)
		updateC = updateC.Set(expression.Name(atribute), expression.Value(s.ingrdList))
	}
	if s.request == "book" || s.request == "close" {
		// don't record book open/close requests. Contents of reqOpenBk tells us about this request.
		s.request = s.lastreq
	}
	atribute = fmt.Sprintf("state[%d].DT", len(s.state)-1)
	updateC = updateC.Set(expression.Name(atribute), expression.Value(time.Now().Format("Jan 2 15:04:05")))

	if len(s.CloseBkName) > 0 {
		atribute = fmt.Sprintf("state[%d].OpenBk", len(s.state)-1)
		updateC = updateC.Set(expression.Name(atribute), expression.Value(s.reqOpenBk))
	}
	// do not change Request value after it has been inserted.
	// atribute = fmt.Sprintf("state[%d].Request", len(s.state)-1)
	// updateC = updateC.Set(expression.Name(atribute), expression.Value(s.request))
	//
	expr, err := expression.NewBuilder().WithUpdate(updateC).Build()
	pkey := pKey{PKey: s.userId}
	av, err := dynamodbattribute.MarshalMap(&pkey)
	if err != nil {
		panic(err)
	}
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
				fmt.Println("error in UpdateItem: ")
				fmt.Println(aerr.Error())
			}
		} else {
			// Print the error, cast err to awserr.Error to get the Code and
			// Message from an error.
			fmt.Println("error in UpdateItem:2 ")
			fmt.Println(err.Error())
		}
		return err
	}
	//
	// following used in close book op - avoids extra dynamo access to fetch Welcome data
	//
	// if s.request == "start" {
	// 	x2 := srecT{}
	// 	err = dynamodbattribute.UnmarshalMap(result.Attributes, &x2)
	// 	if err != nil {
	// 		return fmt.Errorf("Error: %s %s", "in UnmarshalMap in updateState ", err.Error())
	// 	}
	// 	fmt.Printf("displayData = Welcome = %#v\n", *(x2.Sr[0].Welcome))
	// 	s.welcome = x2.Sr[0].Welcome
	// 	s.displayData = x2.Sr[0].Welcome

	// }
	fmt.Println("finish updateState in sessions.go")
	return nil
}

func (s *sessCtx) popState() error {
	//
	// removes top entry in state attribute of dynamo session item.
	//  populates session context with state data from the new top entry
	//  (which was the penultimate entry before deletion)
	//
	// NB: must exit with s.displayData assigned - as this will route to GenDisplay to produce response.
	//
	type pKey struct {
		PKey string
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
	fmt.Println(" state size: ", len(s.state))
	if len(s.state) > 1 {
		//
		// pop state and persist to dynamo
		//
		State = s.state[:len(s.state)-1]
		s.state = State[:]
		fmt.Println("about to save state size: ", len(s.state))
		//
		t := time.Now()
		t.Add(time.Hour * 24 * 1)
		updateC = expression.Set(expression.Name("Epoch"), expression.Value(t.Unix()))
		// rewrite all but last state entry - this is how we delete from a list in dynamo. Here the list represents the state stack.
		updateC = updateC.Set(expression.Name("state"), expression.Value(State))
		expr, err := expression.NewBuilder().WithUpdate(updateC).Build()
		//
		fmt.Println("using s.userid: ", s.userId)
		pkey := pKey{PKey: s.userId}
		av, err := dynamodbattribute.MarshalMap(&pkey)

		input := &dynamodb.UpdateItemInput{
			TableName:                 aws.String("Sessions"),
			Key:                       av, // accets []map[]*attributeValues so must use marshal not expression
			UpdateExpression:          expr.Update(),
			ExpressionAttributeNames:  expr.Names(),
			ExpressionAttributeValues: expr.Values(),
			ReturnValues:              aws.String("UPDATED_NEW"),
		}
		fmt.Println("Dyamo UpdateItem1: ")
		_, err = s.dynamodbSvc.UpdateItem(input)
		if err != nil {
			fmt.Println("Dyamo error in popstate 1: ", err.Error())
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
			fmt.Println("Dyamo error in popstate 2: ", err.Error())
			return err
		}
		// pop last entry from session context state
		//s.state = s.state[:len(s.state)-1]
		fmt.Println("Successfully updateItem")
		sr = State.pop()
	} else {
		sr = s.state.pop() //[len(s.state)-1]
	}
	//
	fmt.Println("after dynamo: ")
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
	//	s.questionId = sr.Qid // Question id	for k,v:=range objectMap {
	s.object = sr.Obj  // Object - to which operation (listing) apply
	s.recId = sr.RecId //s.recId     // current record in object list. (SortK in Recipe table)
	s.reqVersion = sr.Ver
	s.eol = sr.EOL // last RecId of current list. Used to determine when last record is reached or exceeded in the case of goto operation
	s.dmsg = sr.Dmsg
	s.vmsg = sr.Vmsg
	s.ddata = sr.DData
	s.authors = sr.Authors
	s.showObjMenu = sr.ShowObjMenu
	//
	if len(sr.OpenBk) > 0 {
		bk := strings.Split(string(sr.OpenBk), "|")
		fmt.Printf("popstate: open bk %#v\n", bk)
		s.reqBkId, s.reqBkName, s.authors = bk[0], bk[1], bk[2]
		s.authorS = strings.Split(s.authors, ",")
		s.reqOpenBk = sr.OpenBk
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
	//
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
	//
	s.displayData = s.parts
	if len(sr.InstructionData) > 0 {
		fmt.Println("displayData = InstructionData")
		s.displayData = sr.InstructionData
	}
	if s.ctSize == 0 && sr.CtSize > 0 {
		s.ctSize = sr.CtSize
	}
	if len(sr.Ingredients) > 0 {
		fmt.Println("displayData = Ingredients")
		s.displayData = sr.Ingredients
	}
	if len(sr.RecipeList) > 0 {
		fmt.Println("displayData = RecipeList")
		s.displayData = sr.RecipeList
	}
	if sr.ShowObjMenu {
		fmt.Println("displayData = showObjMenu")
		s.displayData = objMenu
		//		s.showObjMenu = sr.ShowObjMenu
	}
	// if sr.Request == "book" && len(sr.OpenBk) > 0 { // book request value not saved in session - as it is
	// 	s.displayData = s.reqOpenBk
	// }
	// if sr.Request == "book/close" && len(sr.OpenBk) > 0 {
	// 	s.displayData = s.reqOpenBk
	//
	if len(sr.MenuL) > 0 {
		s.menuL = sr.MenuL
	}
	//s.dispCtr = &sr.DispCtr
	s.dispCtr = sr.DispCtr
	if sr.ScaleF == 0 {
		global.SetScale(1.0)
	} else {
		global.SetScale(sr.ScaleF)
	}
	s.pkey = s.reqBkId + "-" + s.reqRId
	//
	// set displayData - important to do this as "back" will rely on popstate() to determine apl display to show
	//
	// do we use Request or Display to drive off - only one need be used, but will persis with Request for time being until
	// Display is fully implemented (if ever)
	//if sr.Request == "start" && sr.Display.Type != 0 && s.displayData == nil {
	// if sr.Request == "start" {
	// 	fmt.Println(" ** back now in start")
	// 	s.display = &sr.Display
	// 	fmt.Printf("s.display = %#v\n", s.display)
	// 	var w WelcomeT
	// 	s.displayData = w
	// }
	if sr.Request == "start" {
		fmt.Printf("displayData = Welcome = %#v\n", *(sr.Welcome))
		s.welcome = sr.Welcome // used in close book op
		s.displayData = sr.Welcome

	}
	// switch s.display.Type {
	// case WELCOME:
	// 	var w WelcomeT
	// 	s.displayData = w
	// }

	fmt.Printf("Popstate: parts %#v\n", s.parts)
	fmt.Println("Popstate: sr.showOBjMenu ", sr.ShowObjMenu)
	fmt.Printf("Popstate: sr.RecipeList %#v\n", sr.RecipeList)
	fmt.Printf("Popstate: s.reqBkId %s\n", s.reqBkId)
	fmt.Printf("Popstate: s.reqRId %s\n", s.reqRId)
	fmt.Printf("Popstate: s.request %s\n", s.request)

	fmt.Println("Exit popState()..")
	return nil
}
