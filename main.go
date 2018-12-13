package main

import (
	"context"
	_ "encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	_ "github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	_ "github.com/aws/aws-lambda-go/lambdacontext"
)

//TODO
// change float32 to float64 as this is what dynamoAttribute.Unmarshal uses

// Session Context - assigned from request and Session table.
//  also contains state information relevant to current session and not the next session.
type sessCtx struct {
	operation      string // sourced from Sessions table or request
	sessionId      string // sourced from request. Used as PKey to Sessions table
	request        string // book or recipe request
	reqRName       string // requested recipe name - query param of recipe request
	reqBkName      string // requested book name - query param
	reqRId         string // Recipe Id - 0 means no recipe id has been assigned.  All RId's start at 1.
	reqBkId        string
	swapBkName     string
	swapBkId       string
	reset          bool // zeros []RecId in session table during changes to recipe, as []RecId is recipe dependent
	curreq         int  // bookrecipe_, object_(ingredient,task,container,utensil), listing_(next,prev,goto,modify,repeat)
	questionId     int  // what question is the user responding to with a yes|no.
	dynamodbSvc    *dynamodb.DynamoDB
	object         string //container,ingredient,instruction,utensil. Sourced from Sessions table or request
	updateAdd      int    // dynamodb Update ADD. Operation dependent
	gotoRecId      int    // sourced from request
	recId          int    // nextId for object list (ingredient,task,container,utensil) - sourced from Sessions table (updateSession()
	recIdNotExists bool   // determines whether to create []RecId set attribute in Session  table
	abort          bool   // early return to Alexa
	eol            int    // sourced from Sessions table
	msg            string // local (to this session only) state inf
	text           string // text sent to Alexa display
	verbal         string // text sent to Alexa voice
}

// Session table record
// only items from session context that need to be preserved between sessions are persisted.
type sessRecT struct {
	Obj     string // Object - to which operation (listing) apply
	BkId    string // Book Id
	BKname  string // Book name - saves a lookup under some circumstances
	Rname   string // Recipe name - saves a lookup under some circumstances
	SwpBkNm string
	SwpBkId string
	RId     string // Recipe Id
	Oper    string // Operation (next, prev, repeat, modify)
	Qid     int    // Question id
	RecId   []int  // current record in object list.
	EOL     int    // last RecId of current list. Used to determine when last record is reached or exceeded in the case of goto operation
}

const (
	// objects to which operations apply - s.object values
	ingredient_ string = "ingredient"
	task_       string = "task"
	container_  string = "container"
	utensil_    string = "utensil"
	recipe_     string = "recipe" // list recipe in book
)

const (
	// user request grouped into three types
	bookrecipe_ = iota
	object_
	listing_
)

type objectMapT map[string]int

var objectMap objectMapT

func init() {
	objectMap = make(objectMapT, 4)
	for i, v := range []string{ingredient_, task_, container_, utensil_, recipe_} {
		objectMap[v] = i
	}
}

func (s *sessCtx) mergeAndValidateWithLastSession() error {
	// uses UpdateItem to:
	// 1.  create a new session-state record if record not present. Returns Cid=1
	// 2.  updates Cid attribute by 1 if record present. Returns incremented Cid.
	// TODO - add TTL so records are removed automatically.
	//
	// Table:  Sessions
	//
	// get session and lastOperation or insert new session record with currentOPeration saved as lastOPeration
	// If currentOp is either nextContainer or nextTask ADD 1 to id
	// if currentOp is either repeatContainer or repeatTask ADD 0 to id.
	// if currentOP is either prevContainer or prevTask ADD -1 to id - provided it not at 1 so no previous.
	// if currentOp is none of the above then don't ADD 0 to id
	// if context has flipped midstream ie. user issued cancel with intent to swap request e.g from containers to tasks.
	//   SET id to 1 ie. iether container or task id is now 1, so first item will be returned.
	// user issues "GOTO task <ingredient>|<number>" then SET id = <num> and lastOperation to listTask and return task <num>
	// user issues Modify <QTY>|<TIME>|<TEXT>|<INGREDIENT>

	// Operation has two parts.  Object: task|container  & Command: next|prev|goto (random access)
	// so given above opertions each execution will require two queries on Session table
	//   1.  Get lastOperation & compare with CurrentOperation
	//   2.  If the same than ADD 1|0|-1 or SET ?  id depending on Command
	//   3.  If different then its either flipped or one has finished and moved onto the next.
	//
	// **** Get last state of the session as we need to know its previous Object state if any.
	// ****   NB. updateSession() cannot return previous state only NEW values so we must run a separate query to get old values
	//            before they are updated.
	type pKey struct {
		Sid string
	}

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
		return err
	}
	if len(result.Item) == 0 {
		// *** no session data then ignore validating the session and insert it
		// session with what we've got in the session context
		if s.curreq != bookrecipe_ {
			s.msg = `You must specify a book and recipe from that book. To get started, please say "book", followed by the book name`
			s.abort = true
			return nil
		}
		// s.reqBkId, err = (*s).bookIdLookup()
		// s.msg = `Please say what recipe you would like from this book or I can list them if you like. Say "list" or recipe name.`
		// s.updateSession()
		// return nil
	}
	lastSess := sessRecT{}
	err = dynamodbattribute.UnmarshalMap(result.Item, &lastSess)
	if err != nil {
		return err
	}
	fmt.Printf("in merge.. - s.reqRName = %s\n", s.reqRName)
	fmt.Printf("in merge.. - s.reqBkName = %s\n", s.reqBkName)
	fmt.Printf("in merge.. -  lastSess.Rname = %s\n", lastSess.Rname)
	fmt.Printf("in merge.. -  lastSess.BkName = %s\n", lastSess.BKname)
	fmt.Printf("rows = %d\n", len(result.Item))
	fmt.Printf("rows = %#v\n", lastSess)
	//
	// **************** come this far only if previous session exists *********************
	//
	//
	// initialise session data from last session where missing from current request data

	if len(s.object) == 0 {
		s.object = lastSess.Obj
	}
	if s.eol == 0 {
		s.eol = lastSess.EOL
	}
	if len(s.reqBkId) == 0 {
		s.reqBkId = lastSess.BkId
	}
	if len(s.reqBkName) == 0 {
		s.reqBkName = lastSess.BKname
	}
	if len(s.reqRName) == 0 {
		s.reqRName = lastSess.Rname
	}
	if len(s.reqRId) == 0 {
		s.reqRId = lastSess.RId
	}
	//
	// note:
	// 1. ALL conditional paths return  and most update Sessions. Any updateSessions do not change RecId as current state is bookrecipe_.
	//
	// 2. BookName will always co-exist with BookId in session table by the end of this section
	//	  similarly, RecipeName will always co-exist with RecipeId in the session table by the end of this section
	//
	if s.curreq == bookrecipe_ {
		//
		if s.request == "book" {
			//
			// book requested
			//
			switch len(lastSess.BkId) {
			case 0:
				//
				s.eol = 0
				_, err := (*s).updateSession()
				if err != nil {
					return err
				}
				return nil
			default:
				// there is already an initialised book for this session
				if s.reqBkName != lastSess.BKname {
					// requested a different book
					switch len(lastSess.Rname) {
					case 0:
						// no active recipe. Just get book details
						s.msg = `Please state what recipe you would like from this book or I can list them if you like. Say "list" or recipe name.`
						// Book initialises. No recipe provided. Persist.
						s.eol = 0
						_, err := (*s).updateSession()
						if err != nil {
							return err
						}
						return nil
					default:
						// different book with an active recipe.
						//s.reqRName = lastSess.Rname
						if len(lastSess.Obj) > 0 && s.eol != lastSess.RecId[objectMap[lastSess.Obj]] {
							// currently listing
							s.msg = `You have specified a different book while having an active recipe from which you are currently listing. Do you want to swap to this book, "Yes" or "No" or "Cancel" for no?`
							s.questionId = 21
							// save book details to swap attributes in Session table
							s.swapBkName = s.reqBkName
							//
							//s.eol, s.reset = 0, true - depends on yes/no answer
							_, err := (*s).updateSession()
							if err != nil {
								return err
							}
							return nil
						} else {
							// finished listing or no object selected, swap to this book
							s.msg = `What recipe from this book might you be interested in. Please say a recipe name or I can list them to the display if you like if you "list".`
							// new book selected, zero recipe etc
							s.eol, s.reset, s.reqRId, s.reqRName = 0, true, "", ""
							_, err := (*s).updateSession()
							if err != nil {
								return err
							}
							return nil
						}
					}
				} else {
					// same initialised book requested
					switch len(lastSess.Rname) {
					case 0:
						// no active recipe
						s.msg = `You have specified this book already. Please request a recipe from the book or say "list" and I will print the recipe names to the display.`
						s.abort = true
						return nil
					}
				}
			}
		} else {
			//
			// recipe requested.       Note bookName(Id) can be empty which will force Recipe query to search across all books
			//
			// if len(lastSess.BKname) > 0 {
			// 	s.reqBkName = lastSess.BKname
			// 	s.reqBkId = lastSess.BkId // if bookname exists then so should its id, otherwise it would not be persisted. Not so for recipe.
			// }
			fmt.Printf("Here...1 - recipe: %s lastsess: %s", s.reqRName, lastSess.Rname)
			if len(lastSess.Rname) > 0 {
				// a recipe has already been requested
				if s.reqRName == lastSess.Rname {
					// and initialised. ignore request matches current open recipe
					s.msg = `This recipe is currently opened by you. You can list ingredients, cooking instructions, utensils or containers or cancel`
					return nil
				}
				// change recipe
				s.eol, s.reset = 0, true
				//
				_, err = (*s).updateSession()
				if err != nil {
					return err
				}
				return nil
			} else {
				// first recipe for session.
				// recipe lookup might reassign book so save current value
				s.eol, s.reset = 0, true
				_, err = (*s).updateSession()
				if err != nil {
					return err
				}
				return nil
			}
		}
	}
	//
	// request must set recipe id before proceeding to list an object
	if s.curreq == listing_ && len(lastSess.RId) == 0 || s.curreq == object_ && len(lastSess.RId) == 0 {
		s.msg = `You have not specified a recipe yet. Please say "recipe" followed by it\'s name`
		s.abort = true
		return nil
	}
	//  if listing (next,prev,repeat,goto - curreq object, listing) without object (container,ingredient,task,utensil) -
	if s.curreq == listing_ && len(lastSess.Obj) == 0 {
		s.msg = `You need to say what you want to list. Please say either "ingredients","start cooking","containers" or "utensils". Not hard really..`
		s.abort = true
		return nil
	}
	//  if listing and not finished and object request changes object. Accept and zero or repeat last RecId for requested object.
	if len(lastSess.Obj) > 0 {
		//if !s.finishedListing(lastSess.RecId[objectMap[lastSess.Obj]], objectMap[lastSess.Obj]) && (lastSess.Obj != s.object) {
		if s.eol != lastSess.RecId[objectMap[s.object]] && (lastSess.Obj != s.object) {
			// show last listed entry otherwise list first entry
			switch lastSess.RecId[objectMap[s.object]] {
			case 0: // not listed before or been reset after previously completing list
				s.updateAdd = 1 // show first entry
			default: // in the process of listing
				s.updateAdd = 0 // repeat last shown entry
			}
		}
	}
	// if object specified and different from last one
	if len(s.object) > 0 && len(lastSess.RecId) > 0 && (lastSess.Obj != s.object) {
		// show last listed entry otherwise list first entry
		switch lastSess.RecId[objectMap[s.object]] {
		case 0: // not listed before or been reset after previously completing list
			s.updateAdd = 1 // show first entry
		default: // in the process of listing
			s.updateAdd = 0 // repeat last shown entry
		}
	}
	//  If listing and not finished and object request  has changed (task, ingredient, container, utensil) reset RecId
	// change in operation does not not need to be taken into account as this is part of the initialisation phase
	// copy object from last session
	if s.curreq == listing_ {
		// object is same as last call
		s.object = lastSess.Obj
	}
	// evaluate edge cases for the current session "operation"
	//	execute updateSession only when necessary ie. when a real state change has occured.
	//	This will prevent "repeat" from being recorded in session table.
	//  Must assign a new value s.recId if not calling updateSession based on the request operation.
	//  Next task will be select the record from the object table based on the s.recId.
	switch s.operation {
	case "goto":
		if s.gotoRecId > lastSess.EOL { //EOL of current object (ingredients,tasks..) data
			// do a repeat operation ie. display current record and define new message
			s.msg = "request goes beyond last item in list. Please say again"
			s.recId = lastSess.RecId[objectMap[s.object]]
			return nil
		} else {
			// use updateAdd value to assign new recId during updateSession
			s.updateAdd = s.gotoRecId - lastSess.RecId[objectMap[s.object]]
		}
	case "repeat":
		// return - no need to updateSession as nothing has changed.  Select current recId.
		s.recId = lastSess.RecId[objectMap[s.object]]
		return nil
	case "prev":
		if lastSess.RecId[objectMap[s.object]] == 1 {
			s.msg = "at beginning.."
			s.recId = lastSess.RecId[objectMap[s.object]]
			return nil
		}
		s.updateAdd = -1
	case "next":
		if lastSess.EOL > 0 && len(lastSess.RecId) > 0 {
			if lastSess.RecId[objectMap[s.object]] == lastSess.EOL {
				s.recId = lastSess.EOL
				s.msg = "at end.."
				return nil
			}
		}
		s.updateAdd = 1
	}
	// check if we have Dynamodb Recid Set defined, this will be useful in updateSession
	if len(lastSess.RecId) == 0 {
		s.recIdNotExists = true
	}
	//
	// If we got this far we have a valid session context and want it persisted.
	// Save those bits of the session context that are important to maintain state between requests.
	// Act of updating the table will generate a new record id (ADD in updateItem) which we assign to the session context field recId.
	// We will use sessctx.recId to pick the next record for the response.
	//
	s.recId, err = (*s).updateSession()
	if err != nil {
		return err
	}
	return nil
}

func (s *sessCtx) getRecById() ([]string, error) {
	var ret []string
	switch s.object {
	case task_:
		rec, err := s.getTaskRecById()
		if err != nil {
			return nil, err
		}
		ret = []string{rec.Text, rec.Verbal}
		// if EOL has changed because of object change then update session context with new EOL
		//	use EOL on next session to determine if RecId is at EOL and print end-of-list message
		// fmt.Println("getRecById rec.EOL, s.eol", rec.EOL, s.eol)
		if s.eol != rec.EOL {
			s.eol = rec.EOL
			fmt.Println(" **** about to updateSEssionEOLL: getRecById rec.EOL, s.eol ", rec.EOL, s.eol)
			s.updateSessionEOL()
		}
	case container_:
		rec, err := s.getContainerRecById()
		if err != nil {
			return nil, err
		}
		ret = []string{rec.Txt, rec.Vbl}
		if s.eol != rec.EOL {
			s.eol = rec.EOL
			fmt.Println(" **** about to updateSEssionEOLL: getRecById rec.EOL, s.eol ", rec.EOL, s.eol)
			s.updateSessionEOL()
		}
		//		case utensils: rec, err := s.getNextUtensilRecById()
		//		case ingredients: rec, err := s.getNextIngrdRecById()
	}

	return ret, nil
}

//
//  handler executed via API Gateway via ALexa Lambda
//
func handler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {

	var pathItem []string
	var err error

	dynamodbService := func() *dynamodb.DynamoDB {
		sess, err := session.NewSession(&aws.Config{
			Region: aws.String("us-east-1"),
		})
		if err != nil {
			fmt.Println("Error creating session:")
			log.Panic(err)
		}
		return dynamodb.New(sess, aws.NewConfig())
	}

	fmt.Printf("Resource:  %s\n", request.Resource)
	if len(request.Path) > 0 {
		pathItem = strings.Split(request.Path[1:], "/")
		for i, v := range pathItem {
			fmt.Printf("pathItem: %d   %s\n", i, v)
		}
	}

	log.Printf("\nHTTPMethod: %s", request.HTTPMethod)
	log.Printf("\nBody: %s", request.Body)
	for k, v := range request.Headers {
		log.Printf("Header:  %s  %v", k, v)
	}

	for k, v := range request.QueryStringParameters {
		log.Printf("QueryString:  %s  %v", k, v)
	}
	for k, v := range request.PathParameters {
		log.Printf("PathParameters:  %s  %s", k, v)
	}
	for k, v := range request.StageVariables {
		log.Printf("StageVariable:  %s  %s", k, v)
	}

	var (
		items []string
		body  string
	)
	// create a new session context and merge with last session data if present.
	sessctx := &sessCtx{
		sessionId:   request.QueryStringParameters["sid"],
		dynamodbSvc: dynamodbService(),
	}
	switch pathItem[0] {
	case "load":
		sessctx.reqBkId = "20"
		sessctx.object = "container"
		sessctx.reqRId = "1"
		a, err := readBaseRecipeForContainers(sessctx.dynamodbSvc, sessctx.reqRId)
		if err != nil {
			panic(err)
		}
		_, err = a.saveContainerUsage(sessctx)
		if err != nil {
			panic(err)
		}
		sessctx.reqBkId = "20"
		sessctx.object = "task"
		sessctx.reqRId = "1"
		aa, err := readBaseRecipeForTasks(sessctx.dynamodbSvc, sessctx.reqRId)
		if err != nil {
			panic(err)
		}
		_, err = aa.saveTasks(sessctx)
		if err != nil {
			panic(err)
		}
		os.Exit(0)
	// objec
	case "clear":
		_, err = sessctx.updateSession()
		body = fmt.Sprintf("{ %q : [ %q, %q , %q] }", "response", "", "", "book entry cleared")
		return events.APIGatewayProxyResponse{
			StatusCode: 200,
			Body:       body,
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
		}, err
	case "book", "recipe":
		sessctx.curreq = bookrecipe_
		switch pathItem[0] {
		case "book":
			// book request data fully populated in this section
			sessctx.request = "book"
			sessctx.reqBkId = request.QueryStringParameters["bkid"]
			sessctx.reqBkName, err = sessctx.bookIdLookup()
		case "recipe":
			// both recipe and book request data fully populated in this section
			p := strings.Split(request.QueryStringParameters["bkrid"], "-") // [bkid]-[rid]
			sessctx.reqBkId = p[0]
			sessctx.reqBkName, err = sessctx.bookIdLookup()
			if err == nil {
				sessctx.reqRId = p[1]
				sessctx.reqRName, err = (*sessctx).recipeIdLookup()
			}
			// TODO provide a query to use BookID to list available recipes. May be need to include list of recipes as an OBject to list for the user.
			// err = sessctx.saveRecipe()	//TODO this method woul be used when Recipes are being created not as part of the user request.
			//	err = sessctx.mergeRecipeMetaData()
		}
	case container_, task_: //ingredient_, utensil_
		sessctx.object = pathItem[0]
		sessctx.curreq = object_
		sessctx.operation = "next"
	// operation
	case "next", "prev", "goto", "repeat", "modify":
		sessctx.operation = pathItem[0]
		sessctx.curreq = listing_
		// sessctx.object will depend on value from last session, which must exist for an operation.
		switch pathItem[0] {
		case "goto":
			var i int
			i, err = strconv.Atoi(request.QueryStringParameters["goId"])
			sessctx.gotoRecId = i
		}
		// case "yes":  check questions asked from last session (session table)
		// case "no":
	}
	if err != nil {
		err := fmt.Errorf("%s: %s", "Error in converting int of goto operation ", err.Error())
		return events.APIGatewayProxyResponse{
			StatusCode: 500,
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
		}, err
	}
	// compare with last session and update operation (eg task to container) if necessary.
	err = sessctx.mergeAndValidateWithLastSession()
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: 500,
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
		}, err
	}

	if sessctx.curreq == bookrecipe_ || sessctx.abort {
		body = fmt.Sprintf("{ %q : [ %q, %q , %q] }", "response", sessctx.reqBkId, sessctx.reqRId, sessctx.msg)
		return events.APIGatewayProxyResponse{
			StatusCode: 200,
			Body:       body,
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
		}, nil
	}
	//
	// session ctx validated and fully populated - now fetch required record
	//
	items, err = sessctx.getRecById() // returns [text, verbal] response
	if err != nil {
		err := fmt.Errorf("%s: %s", "Error from getRecById: ", err.Error())
		return events.APIGatewayProxyResponse{
			StatusCode: 500,
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
		}, err
	}
	body = fmt.Sprintf("{ %q : [ %q, %q, %q ] }", "response", items[0], items[1], sessctx.msg)
	//
	// check URL and action it
	//
	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Body:       body,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
	}, nil

} //  handler

func main() {
	lambda.Start(handler)
}
