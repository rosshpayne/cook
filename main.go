package main

import (
	_ "context"
	_ "encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"strconv"
	"strings"

	_ "github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
)

//TODO
// change float32 to float64 as this is what dynamoAttribute.Unmarshal uses

// Session Context - assigned from request and Session table.
//  also contains state information relevant to current session and not the next session.
type sessCtx struct {
	newSession bool
	//path      string // InputEvent.Path
	request string // pathItem[0]: request from user e.g. select, next, prev,..
	//param     string // InputEvent.Param
	state     stateStack
	lastState *stateRec // state attribute from state dynamo item - contains state history
	passErr   string
	//
	sessionId  string // sourced from request. Used as PKey to Sessions table
	reqOpenBk  string
	reqRName   string // requested recipe name - query param of recipe request
	reqBkName  string // requested book name - query param
	reqRId     string // Recipe Id - 0 means no recipe id has been assigned.  All RId's start at 1.
	reqBkId    string
	reqSearch  string // keyword search value
	reqVersion string // version id, starts at 0 which is blank??
	//reqSearch   string   // search value
	recId       [4]int   // record id for each object (ingredient, container, utensils, containers). No display will use verbal for all object listings.
	pkey        string   // primary key
	recipe      *RecipeT //  record from dynamo recipe query
	swapBkName  string
	swapBkId    string
	authorS     []string
	authors     string         // flattened authorS = comma separted list of authors
	serves      string         // recipe serves. Source recipe used in Ingredients search.
	index       []string       // user defined entries under which recipe is indexed. Sourced from recipe not ingredient.
	indexRecs   []indexRecipeT // processed index entries as saved to dynamo
	dbatchNum   string         // mulit-records sent to display in fixed batch sizes (6 say).
	reset       bool           // zeros []RecId in session table during changes to recipe, as []RecId is recipe dependent
	curReqType  int            // initialiseRequest, objectRequest(ingredient,task,container,utensil), instructionRequest(next,prev,goto,modify,repeat)
	questionId  int            // what question is the user responding to with a yes|no.
	dynamodbSvc *dynamodb.DynamoDB
	object      string //container,ingredient,instruction,utensil. Sourced from Sessions table or request
	//updateAdd      int        // dynamodb Update ADD. Operation dependent
	gotoRecId        int        // sourced from request
	objRecId         int        // current record id for object. Object is a ingredient,task,container,utensil.- displayed record id persisted to session after use.
	recIdNotExists   bool       // determines whether to create []RecId set attribute in Session  table
	noGetRecRequired bool       // a mutliple record request e.g. ingredients listing
	mChoice          []mRecipeT // multi-choice select. Recipe name and ingredient searches can result in mutliple records being returned. Results are saved.
	//
	selCtx selectCtxT // select context either recipe or other (i.e. object)
	selId  int        // value selected by user of index in itemList
	//selClear bool
	//
	showList  bool   // show what ever is in the current list (books, recipes)
	ingrdList string // output of activity.String() - ingredient listing
	// vPreMsg        string
	// dPreMsg        string
	displayHdr    string // passed to alexa display
	displaySubHdr string // passed to alexa display
	activityS     Activities
	dmsg          string // display msg
	vmsg          string // verbal msg
	ddata         string
	yesno         string
	//
	instructions []InstructionT // cached instructions for complete or part based recipe
	// Recipe Part data
	eol   int     // sourced from Sessions table
	peol  int     // End-of-List-for-part
	part  string  // part index name - if no value then no part is being used eventhough recipe may be have a part defined i.e nopart_ & a part
	parts []PartT // sourced from Recipe (R-)
	// cPart string  // current part being display (long name). All part means complete recipe will be listed.
	// next  int     // next SortK (recId)
	// prev  int     // previous SortK (recId) when in part mode as opposed to full recipe mode
	pid int // record id within a part 1..peol
	// //
	back bool // back button pressed on display
	//
	dispObjectMenu  bool
	dispIngredients bool
	dispContainers  bool
	dispPartMenu    bool
}

func (s *sessCtx) clearForSearch(lastState *stateRec) {
	s.reqBkId, s.reqBkName, s.reqRId, s.reqRName, s.reqVersion = "", "", "", "", ""
	s.recId = [4]int{0, 0, 0, 0}
	s.authors, s.authorS = "", nil
	s.serves = ""
	s.indexRecs = nil
	s.object = ""
	s.objRecId = 0
	s.mChoice = nil
	s.selCtx, s.selId = 0, 0
	s.ingrdList = ""
	s.eol, s.peol, s.part, s.parts, s.pid = 0, 0, "", nil, 0
	s.back = false
	s.dispObjectMenu, s.dispIngredients, s.dispContainers, s.dispPartMenu = false, false, false, false
	if len(lastState.OpenBk) > 0 {
		s.reqOpenBk = lastState.OpenBk
		id := strings.Split(lastState.OpenBk, "|")
		s.reqBkId, s.reqBkName = id[0], id[1]
		s.authors = id[2]
	}
}

const (
	// objects to which future requests apply - s.object values
	ingredient_     string = "ingredient"
	task_           string = "task"
	container_      string = "container"
	utensil_        string = "utensil"
	recipe_         string = "recipe" // list recipe in book
	CompleteRecipe_ string = "Complete recipe"
)

type selectCtxT int

const (
	ctxRecipeMenu selectCtxT = 1
	ctxObjectMenu            = 2
	ctxPartMenu              = 3
)

// type objectT int

// const (
// 	// objects to which future requests will apply
// 	objIngredient objectT = iota
// 	objTask
// 	objContainer
// 	objUtensil
// 	objRecipe // list recipe in book
// )

const (
	// user request grouped into three types
	xx = iota // TODO check this
	initialiseRequest
	objectRequest
	instructionRequest
)

const (
	displayBatchSize_C int = 6
)

type objectMapT map[string]int

var objectMap objectMapT
var objectS []string

func init() {
	objectMap = make(objectMapT, 4)
	for i, v := range []string{ingredient_, task_, container_, utensil_, recipe_} {
		objectMap[v] = i
	}
	objectS = make([]string, 4)
	for i, v := range []string{ingredient_, utensil_, container_, task_} {
		objectS[i] = v
	}
}

type alexaDialog interface {
	Alexa() dialog
}
type dialog struct {
	Verbal  string
	Display string
	EOL     int
	PID     int // instruction ID within part
	PEOL    int
	PART    string
}

func (s *sessCtx) orchestrateRequest() error {
	//
	// ******************************** 	initialise state		****************************************
	//
	// fetch state from last session
	lastState, err := s.getState()
	if err != nil {
		return err
	}
	// set current state based on last session
	s.setState(lastState)
	//
	// ******************************** process responses to request  ****************************************
	//
	// yes/no response. May assign new book
	//
	if len(s.yesno) > 0 && lastState.Qid > 0 {
		if s.yesno == "yes" {
			var err error
			switch lastState.Qid {
			case 20:
				// close active book
				//  as req struct fields still have their zero value they will clear session state during pushState
				s.reset = true
				s.vmsg = fmt.Sprintf(`%s is closed. You can now search across all recipe books`, lastState.BkName)
				s.dmsg = fmt.Sprintf(`%s is closed. You can now search across all recipe books`, lastState.BkName)
				s.reqBkId, s.reqBkName, s.reqRName, s.reqRId, s.eol, s.reset = "", "", "", "", 0, true
				_, err = s.pushState()
			case 21:
				// swap book
				s.reqBkId, s.reqBkName, s.reset = lastState.SwpBkId, lastState.SwpBkNm, true
				s.vmsg = fmt.Sprintf(`Book [%s] is now open. You can now search or open a recipe within this book`, lastState.BkName)
				s.dmsg = fmt.Sprintf(`Book [%s] is now open. You can now search or open a recipe within this book`, lastState.BkName)
				_, err = s.pushState()
			default:
				// TODO: log error to error table
			}
			if err != nil {
				return fmt.Errorf("Error: in mergeAndValidateWithlastStateion of pushState() - %s", err.Error())
			}
		}
		if len(s.dmsg) == 0 {
			s.dmsg = lastState.Dmsg
		}
		//s.noGetRecRequired = true
		_, err = s.pushState()
		if err != nil {
			return err
		}
		return nil
	}
	//
	// respond to select from displayed items
	//
	if s.selId > 0 {
		// selId is the response from Alexa on the index (ordinal value) of the selected display item
		fmt.Println("SELCTX is : ", s.selCtx)
		switch s.selCtx {
		case ctxRecipeMenu:
			// select from: multiple recipes
			if s.selId > len(s.mChoice) || s.selId < 1 {
				s.passErr = "selection is not within range"
				return nil
			}
			p := s.mChoice[s.selId-1]
			s.reqRId, s.reqRName, s.reqBkId, s.reqBkName = p.RId, p.RName, p.BkId, p.BkName
			s.dmsg = fmt.Sprintf(`Now that you have selected [%s] recipe would you like to list ingredients, cooking instructions, utensils or containers or cancel`, s.reqRName)
			s.vmsg = fmt.Sprintf(`Now that you have selected {%s] recipe would you like to list ingredients, cooking instructions, utensils or containers or cancel`, s.reqRName)
			// chosen recipe, so set select context to object (ingredient, utensil, container, tas			s.selCtx = ctxObjectMenu
			//
			_, err := s.recipeRSearch()
			if err != nil {
				return err
			}
			s.reset = true
			_, err = s.pushState()
			if err != nil {
				return err
			}
			return nil

		case ctxObjectMenu:
			//	select from: list ingredient, list utensils, list containers, start cooking
			fmt.Println("selId: ", s.selId)
			if s.selId > len(objectS) {
				s.setDisplay(lastState)
				s.passErr = "selection is not within range"
				return nil
			}
			s.object = objectS[s.selId-1]
			fmt.Println("object: ", s.object)
			// clear mChoice which is not necessary in this state. Held in previous state.
			s.mChoice = nil
			// object chosen, nothing more to select for the time being
			//s.selCtx = 0
			switch s.object {

			//  "lets start cooking" selected
			case task_:
				//s.selClear = true //TODO: what if they back at this point we have cleared sel.
				fmt.Printf("s.parts  %#v [%s] \n", s.parts, s.part)
				if len(s.parts) > 0 && len(s.part) == 0 {
					s.dispPartMenu = true
					//
					// create recipe part menu
					//
					menu := make([]mRecipeT, len(s.parts)+1)
					menu[0] = mRecipeT{Part: CompleteRecipe_}
					for i, v := range s.parts {
						menu[i+1] = mRecipeT{Part: v.Title}
					}
					s.mChoice = menu
					_, err = s.pushState()
					if err != nil {
						return err
					}
					return nil
				} else {
					// go straight to instructions
					s.cacheInstructions(CompleteRecipe_)
					//
					_, err = s.pushState()
					if err != nil {
						return err
					}
					// now complete the request by morphing request to a "next" operation
					s.reset = true
					s.request = "select-next"
					s.curReqType = instructionRequest
					s.object = "task"
				}

			//  "list ingredients" selected
			case ingredient_:
				s.pkey = s.reqBkId + "-" + s.reqRId
				if s.reqVersion != "" {
					s.pkey += "-" + s.reqVersion
				}
				r, err := s.recipeRSearch()
				if err != nil {
					return err
				}
				// set unit formating mode
				writeCtx = uPrint
				as, err := s.loadActivities()
				if err != nil {
					return err
				}
				// generate ingredient listing
				s.ingrdList = as.String(r)
				s.dispIngredients = true
				//
				s.reset = true
				_, err = s.pushState()
				if err != nil {
					return err
				}

			case container_:
				s.dispContainers = true
				_, err = s.pushState()
				if err != nil {
					return err
				}
				return nil
			}

		// recipe part menu
		case ctxPartMenu:
			if s.selId > len(s.mChoice) || s.selId < 1 {
				s.setDisplay(lastState)
				s.passErr = "selection is not within range"
				return nil
			}
			p := s.mChoice[s.selId-1]
			fmt.Printf("selId  %d   mChoice   %#v\n", s.selId, p)
			s.reset = true
			s.cacheInstructions(p.Part)
			//
			_, err = s.pushState()
			if err != nil {
				return err
			}
			// now complete the request by morphing request to a next operation
			s.request = "select-next"
			s.curReqType = instructionRequest
			s.object = "task"
		}
	}
	//
	// note:
	// 1. ALL conditional paths return  and most update Sessions. Any pushStates do not change RecId as current state is initialiseRequest.
	//
	// 2. BookName will always co-exist with BookId in session table by the end of this section
	//	  similarly, RecipeName will always co-exist with RecipeId in the session table by the end of this section
	//
	if s.curReqType == initialiseRequest {
		//
		if s.request == "search" {
			// search only applies to recipes, ie. select context Recipe (ctxRecipeMenu).
			//s.selCtx = ctxRecipeMenu
			// we have fully populated session context from previous session e.g. BkName etc, now lets see what recipes we find
			// populates sessCtx.mChoice if search results in a list.
			fmt.Println("Search.....")
			fmt.Println("BookId: ", s.reqBkId)
			s.clearForSearch(lastState)
			fmt.Println("**** just cleared state . ***** now pushState")
			_, err = s.pushState()
			if err != nil {
				return err
			}
			//
			fmt.Println("..about to call keywordSearch")
			err := s.keywordSearch()
			if err != nil {
				return err
			}
			s.eol, s.reset, s.object = 0, true, ""
			if len(s.mChoice) == 0 || len(s.reqRId) > 0 {
				// single recipe found in search. Select context must now reflect object list. Persist value.
				//s.selCtx = ctxObjectMenu
				s.dispObjectMenu = true
			}
			_, err = s.pushState()
			if err != nil {
				return err
			}
			return nil
		}
		//TODO: is showList required?
		if s.showList {
			if len(s.mChoice) > 0 {
				for i, v := range s.mChoice {
					s.dmsg = s.dmsg + fmt.Sprintf("%d. Recipe [%s] in book [%s] by [%s] quantity %s\n", i+1, v.RName, v.BkName, v.Authors, v.Quantity)
					s.vmsg = s.dmsg + fmt.Sprintf("%d. Recipe [%s] in book [%s] by [%s] quantity %s\n", i+1, v.RName, v.BkName, v.Authors, v.Quantity)
				}
			}
			s.noGetRecRequired = true
			return nil
		}
		if s.request == "book/close" {
			if len(s.object) > 0 && s.eol != s.recId[objectMap[s.object]] {
				s.dmsg = fmt.Sprintf("You currently have recipe %s open. Do you still want to close the book?", lastState.RName)
				s.vmsg = fmt.Sprintf("You currently have recipe %s open. Do you still want to close the book?", lastState.RName)
				s.questionId = 20
			} else {
				switch len(s.reqBkId) {
				case 0:
					s.dmsg = `There is no book open to close.`
					s.vmsg = `There is no book open to close.`
				default:
					//
					s.dmsg = s.reqBkName + ` is now closed. Any searches will be across all books`
					s.vmsg = s.reqBkName + ` is now closed. Any searches will be across all books`
					s.reqBkId, s.reqBkName, s.reqRId, s.reqRName = "", "", "", ""
					s.reqOpenBk, s.authorS, s.authors = "", nil, ""
				}
			}
			s.eol = 0
			_, err := s.pushState() //TODO: do I need to do this..
			if err != nil {
				return err
			}
			return nil
		}
		if s.request == "book" {
			//
			// open book requested
			//
			if len(lastState.BkId) > 0 {
				// some book was open previously
				switch {
				case lastState.BkId == s.reqBkId:
					// open currenly opened book. reqBkId was sourced using bookNameLookup()
					if len(lastState.RName) == 0 {
						// no active recipe
						s.dmsg = `Book is already open. Please request a recipe from the book or say "list" and I will print the recipe names to the display.`
						s.vmsg = `Book is already open. Please request a recipe from the book or say "list" and I will print the recipe names to the display.`
						s.noGetRecRequired = true
					} else if s.eol != s.recId[objectMap[s.object]] {
						s.dmsg = `You are actively browing recipe ` + lastState.RName + ".Do you still want to open this book?"
						s.vmsg = `You are actively browsing recipe ` + lastState.RName + ".Do you still want to open this book?"
					}
				case lastState.BkId != s.reqBkId:
					if len(lastState.RName) == 0 {
						// no active recipe
						s.dmsg = "Opened " + s.reqBkName + " by " + s.authors + ". "
						s.vmsg = "Opened " + s.reqBkName + " by " + s.authors + ". "
						s.eol = 0
						s.noGetRecRequired = true
					} else if s.eol != s.recId[objectMap[s.object]] {
						s.dmsg = `You are actively browsing recipe ` + lastState.RName + " from book, " + lastState.BkName + ". Do you still want to open this book?"
						s.vmsg = `You are actively browsing recipe ` + lastState.RName + " from book, " + lastState.BkName + ". Do you still want to open this book?"
					}
				}
			} else {
				s.eol = 0
				s.dmsg = "Opened " + s.reqBkName + " by " + s.authors + ". "
				s.vmsg = "Opened " + s.reqBkName + " by " + s.authors + ". "
			}
			_, err = s.pushState()
			if err != nil {
				return err
			}
		}
		if s.request == "recipe" { // "open recipe" intent
			//
			// recipe requested.       Note bookName(Id) can be empty which will force Recipe query to search across all books
			//
			err := s.recipeNameSearch()
			if err != nil {
				return err
			}
			s.eol, s.reset = 0, true
			_, err = s.pushState()
			if err != nil {
				return err
			}
			return nil
		}
	}
	//
	// request must set recipe id before proceeding to list an object
	if s.curReqType == instructionRequest && len(lastState.RId) == 0 || s.curReqType == objectRequest && len(lastState.RId) == 0 {
		s.dmsg = `You have not specified a recipe yet. Please say "recipe" followed by it\'s name`
		s.vmsg = `You have not specified a recipe yet. Please say "recipe" followed by it\'s name`
		s.noGetRecRequired = true
		return nil
	}
	//  if listing (next,prev,repeat,goto - curReqType object, listing) without object (container,ingredient,task,utensil) -
	if s.curReqType == instructionRequest && len(s.object) == 0 {
		s.dmsg = `You need to say what you want to list. Please say either "ingredients","start cooking","containers" or "utensils". Not hard really..`
		s.vmsg = `You need to say what you want to list. Please say either "ingredients","start cooking","containers" or "utensils". Not hard really..`
		s.noGetRecRequired = true
		return nil
	}
	//  if listing and not finished and object request changes object. Accept and zero or repeat last RecId for requested object.
	if len(s.object) > 0 {
		//if !s.finishedListing(s.recId[objectMap[s.object]], objectMap[s.object]) && (s.object != s.object) {
		fmt.Println(s.object)
		if len(s.recId) > 0 {
			if s.eol != s.recId[objectMap[s.object]] && (s.object != s.object) {
				// show last listed entry otherwise list first entry
				switch s.recId[objectMap[s.object]] {
				case 0: // not listed before or been reset after previously completing list
					s.objRecId = 1 // show first entry
				default: // in the process of listing
					s.objRecId = s.recId[objectMap[s.object]] // repeat last shown entry
				}
			}
		}
	}
	// if object specified and different from last one
	if len(s.object) > 0 && len(s.recId) > 0 && (s.object != s.object) {
		// show last listed entry otherwise list first entry
		switch s.recId[objectMap[s.object]] {
		case 0: // not listed before or been reset after previously completing list
			s.objRecId = 1 // show first entry
		default: // in the process of listing
			s.objRecId = s.recId[objectMap[s.object]] // repeat last shown entry
		}
	}
	//  If listing and not finished and object request  has changed (task, ingredient, container, utensil) reset RecId
	// change in operation does not not need to be taken into account as this is part of the initialisation phase
	// copy object from last session
	if s.curReqType == instructionRequest {
		// object is same as last call
		s.object = s.object
	}
	switch s.request {
	case "goto":
		fmt.Printf("gotoRecId = %d  %d\n", s.gotoRecId, lastState.EOL)
		//TODO goto not implemented - maybe no use case
		// if s.gotoRecId > lastState.EOL { //EOL of current object (ingredients,tasks..) data
		// 	// do a repeat operation ie. display current record and define new message
		// 	s.dmsg = "request goes beyond last item in list. Please say again"
		// 	s.vmsg = "request goes beyond last item in list. Please say again"
		// 	s.objRecId= s.recId[objectMap[s.object]]
		// 	fmt.Printf("gotoRecId = %d  %d %d\n", s.gotoRecId, lastState.EOL, s.recId)
		// 	s.noGetRecRequired = true
		// 	return nil
		// } else {
		// 	// use updateAdd value to assign new recId during pushState
		// 	s.updateAdd = s.gotoRecId - s.recId[objectMap[s.object]]
		// }
	case "repeat":
		// return - no need to pushState as nothing has changed.  Select current recId.
		s.objRecId = s.recId[objectMap[s.object]]
		s.dmsg, s.vmsg, s.ddata = lastState.Dmsg, lastState.Vmsg, lastState.DData
		s.noGetRecRequired = true
		return nil
	case "prev":
		if len(s.object) == 0 {
			return fmt.Errorf("Error: no object defined for previous in orchestrateRequest")
		}
		s.objRecId = s.recId[objectMap[s.object]] - 1
		s.recId[objectMap[s.object]] -= 1
	case "next", "select-next":
		if len(s.recId) == 0 {
			s.recId = [4]int{}
		}
		//
		// for Recipe instructions (need to recode for other..)
		//
		if len(s.object) == 0 {
			return fmt.Errorf("Error: no object defined for next in orchestrateRequest")
		}
		// }
		// recId contains last processed instruction. So must add one to get current instruction.
		s.objRecId = s.recId[objectMap[s.object]] + 1
		s.recId[objectMap[s.object]] += 1
	}
	//
	return nil
}

func (s *sessCtx) getRecById() error {
	var (
		at  alexaDialog
		err error
	)
	switch s.object {
	case task_:
		at, err = s.getTaskRecById()
	case container_:
		at, err = s.getContainerRecById()
		//case utensils: at, err := s.getNextUtensilRecById()
		//case ingredients: at, err := s.getNextIngrdRecById()
	}
	if err != nil {
		return err
	}
	rec := at.Alexa()
	//
	writeCtx = uSay
	s.vmsg = expandScalableTags(expandLiteralTags(rec.Verbal))
	writeCtx = uDisplay
	s.dmsg = expandScalableTags(expandLiteralTags(rec.Display))
	//
	// save state to dynamo
	//
	err = s.updateState()
	if err != nil {
		return err
	}
	return nil
}

type InputEvent struct {
	Path                  string            `json:"Path"`
	Param                 string            `json:"Param"`
	QueryStringParameters map[string]string `json:"-"`
	PathItem              []string          `json:"-"`
}

func (r *InputEvent) init() {
	r.QueryStringParameters = make(map[string]string)
	params := strings.Split(r.Param, "&")
	for _, v := range params {
		param := strings.Split(v, "=")
		r.QueryStringParameters[param[0]] = param[1]
	}
	r.PathItem = strings.Split(r.Path, "/")
}

type DisplayItem struct {
	Id        string
	Title     string
	SubTitle1 string
	SubTitle2 string
	Text      string
}
type RespEvent struct {
	Position string        `json:"Position"` // recId|EOL|PEOL|PName
	BackBtn  bool          `json:"Back"`
	Type     string        `json:"Type"`
	Header   string        `json:"Header"`
	SubHdr   string        `json:"SubHdr"`
	Text     string        `json:"Text"`
	Verbal   string        `json:"Verbal"`
	Error    string        `json:"Error"`
	List     []DisplayItem `json:"List"` // recipe data: id|Title1|subTitle1|SubTitle2|Text
	ListA    []DisplayItem `json:"ListA"`
	ListB    []DisplayItem `json:"ListB"`
	ListC    []DisplayItem `json:"ListC"`
}

//
func handler(request InputEvent) (RespEvent, error) {

	var (
		pathItem []string
		err      error
	)

	(&request).init()

	dynamodbService := func() *dynamodb.DynamoDB {
		sess, err := session.NewSession(&aws.Config{
			Region: aws.String("us-east-1"),
		})
		if err != nil {
			log.Panic(err)
		}
		return dynamodb.New(sess, aws.NewConfig())
	}

	pathItem = request.PathItem

	//var body string
	// create a new session context and merge with last session data if present.
	sessctx := &sessCtx{
		sessionId:   request.QueryStringParameters["sid"],
		dynamodbSvc: dynamodbService(),
		request:     pathItem[0],
		//path:        request.Path,
		//param:       request.Param,
	}
	//
	// Three request types:
	//   * initialiseRequest - all requests associated with listing recipe related data
	//   * objectRequest - determines the object to which next instructionRequest will be applied either task(instruction) or container,
	//                     useful for verbal interaction which by definition allows random requests.
	//                     unecessary type when user follows displayed interaction
	//   * instructionRequest - requests associated with displaying an instruction record.
	//
	switch sessctx.request {
	case "purge":
		sessctx.reqBkId = request.QueryStringParameters["bkid"]
		sessctx.reqRId = request.QueryStringParameters["rid"]
		sessctx.reqVersion = request.QueryStringParameters["ver"]
		//
		sessctx.pkey = sessctx.reqBkId + "-" + sessctx.reqRId
		if sessctx.reqVersion != "" {
			sessctx.pkey += "-" + sessctx.reqVersion
		}
		// fetch recipe name and book name
		_, err = sessctx.recipeRSearch()
		if err != nil {
			break
		}
		// read base recipe data and generate tasks, container and device usage and save to dynamodb.
		err = sessctx.purgeRecipe()
		if err != nil {
			break
		}
		sessctx.noGetRecRequired, sessctx.reset = true, true
	//
	case "load", "print":
		//
		//   these requests are used for the standalone executable
		//   print :ingredients" also accessible in the Alexa interaction via screen interaction
		//
		var r *RecipeT
		sessctx.reqBkId = request.QueryStringParameters["bkid"]
		sessctx.reqRId = request.QueryStringParameters["rid"]
		sessctx.reqVersion = request.QueryStringParameters["ver"]
		//
		sessctx.pkey = sessctx.reqBkId + "-" + sessctx.reqRId
		if sessctx.reqVersion != "" {
			sessctx.pkey += "-" + sessctx.reqVersion
		}
		// fetch recipe name and book name
		r, err = sessctx.recipeRSearch()
		if err != nil {
			break
		}
		// read base recipe data and generate tasks, container and device usage and save to dynamodb.
		switch sessctx.request {
		case "load":
			pIngrdScale = 1.0
			err = sessctx.loadBaseRecipe()
			if err != nil {
				break
			}
		case "print":
			// set unit format mode
			writeCtx = uPrint
			as, err := sessctx.loadActivities()
			if err != nil {
				fmt.Printf("error: %s", err.Error())
			}
			fmt.Println(as.String(r))
		}
		sessctx.noGetRecRequired, sessctx.reset = true, true
	//
	case "genSlotValues":
		err = sessctx.generateSlotEntries()
		if err != nil {
			break
		}
		sessctx.noGetRecRequired, sessctx.reset = true, true
	case "book", "recipe", "select", "search", "list", "yesno", "version", "back":
		sessctx.curReqType = initialiseRequest
		switch sessctx.request {
		case "book": // user reponse "open book" "close book"
			// book id and name  populated in this section
			if len(pathItem) > 1 && pathItem[1] == "close" {
				sessctx.request = "book/close"
			} else { // open
				sessctx.reqBkId = request.QueryStringParameters["bkid"]
				err = sessctx.bookNameLookup()
				sessctx.reqOpenBk = sessctx.reqBkId + "|" + sessctx.reqBkName + "|" + sessctx.authors
			}
		case "version":
			sessctx.reqVersion = request.QueryStringParameters["ver"]
		case "list":
			sessctx.showList = true
		case "search":
			sq, err_ := url.QueryUnescape(request.QueryStringParameters["srch"])
			err = err_
			sessctx.reqSearch = strings.ToLower(sq)
		case "recipe": // must be Recipe Name not Ingredient-cat
			// Alexa request: query parameter format either BkId-RId or Recipe name as spoken ie. Alexa's slot-type name
			// decided that BkId-RId is a bad idea as it can conflict with dynamodb so Slot-type can only have full recipe names.
			var rcp string
			rcp, err = url.QueryUnescape(request.QueryStringParameters["rcp"])
			if err != nil {
				panic(err)
			}
			sessctx.reqRName = rcp
		case "yesno":
			i := request.QueryStringParameters["yn"] // "1" yes, "0" no
			sessctx.yesno = "no"
			if i == "1" {
				sessctx.yesno = "yes"
			}
		case "select":
			var i int
			i, err = strconv.Atoi(request.QueryStringParameters["sId"])
			if err != nil {
				err = fmt.Errorf("%s: %s", "Error in converting int of select request \n\n", err.Error())
			} else {
				sessctx.selId = i
			}
		case "back":
			// used back button on display device
			sessctx.back = true
			err = sessctx.popState()
			//
		}
	//
	case container_, task_:
		//  "object" request is required only for VUI, as all requests are be random by nature.
		//  GUI requests, on the other hand, are controlled by this app making the following request redundant.
		sessctx.object = sessctx.request
		sessctx.curReqType = objectRequest
		sessctx.request = "next"
		fmt.Printf("s.parts  %#v\n", sessctx.parts)
		if sessctx.request == task_ {
			if len(sessctx.parts) > 0 && len(sessctx.part) == 0 {
				sessctx.dispPartMenu = true
				sessctx.noGetRecRequired = true
			}
			//s.getInstructions()
		}
	case "next", "prev", "goto", "repeat", "modify":
		sessctx.curReqType = instructionRequest
		switch sessctx.request {
		case "goto":
			var i int
			i, err = strconv.Atoi(request.QueryStringParameters["goId"])
			if err != nil {
				err = fmt.Errorf("%s: %s", "Error in converting int of goto request ", err.Error())
			} else {
				sessctx.gotoRecId = i
			}
		}
	}
	if err != nil {
		return RespEvent{Text: sessctx.vmsg, Verbal: sessctx.dmsg + sessctx.ddata, Error: err.Error()}, nil
	}
	//
	// validate the request and populate session context with appropriate metadata associated with request
	//
	if !sessctx.back {
		err = sessctx.orchestrateRequest()
		if err != nil {
			return RespEvent{Text: sessctx.vmsg, Verbal: sessctx.dmsg + sessctx.ddata, Error: err.Error()}, nil
		}
	}
	//
	// package the response data into RespEvent (an APL aware "display" structure) and return
	//
	var (
		subh string
		hdr  string
	)
	if sessctx.curReqType == initialiseRequest || sessctx.noGetRecRequired {

		switch {

		case sessctx.dispPartMenu == true:
			if len(sessctx.passErr) > 0 {
				hdr = sessctx.passErr
			} else {
				hdr = sessctx.reqRName + ` recipe is divided into parts.`
				//subh = `Select first option to follow complete recipe`
			}
			mchoice := make([]DisplayItem, len(sessctx.mChoice))
			for i, v := range sessctx.mChoice {
				id := strconv.Itoa(i + 1)
				mchoice[i] = DisplayItem{Id: id, Title: v.Part}
			}
			s := sessctx
			return RespEvent{Type: "Select", Header: hdr, SubHdr: subh, Text: s.vmsg, Verbal: s.dmsg, List: mchoice}, nil

		case sessctx.dispIngredients == true:
			//case sessctx.request == "select" && sessctx.object == ingredient_:

			var ingrdlst []DisplayItem
			for _, v := range strings.Split(sessctx.ingrdList, "\n") {
				item := DisplayItem{Title: v}
				ingrdlst = append(ingrdlst, item)
			}
			s := sessctx
			return RespEvent{Type: "Ingredient", Header: s.reqRName, SubHdr: "Ingredients", List: ingrdlst}, nil

		case sessctx.dispContainers == true:
			//case sessctx.request == "select" && sessctx.object == container_:

			var mchoice []DisplayItem
			for _, v := range sessctx.getContainers() {
				item := DisplayItem{Title: v.Verbal}
				mchoice = append(mchoice, item)
			}
			s := sessctx
			return RespEvent{Type: "Ingredient", Header: s.reqRName, SubHdr: "Containers", List: mchoice}, nil

		case sessctx.request == "search" && len(sessctx.mChoice) > 0:
			// display recipes
			var mchoice []DisplayItem
			for _, v := range sessctx.mChoice {
				var item DisplayItem
				id := strconv.Itoa(v.Id)
				if len(v.Serves) > 0 {
					item = DisplayItem{Id: id, Title: v.RName, SubTitle1: "Book: " + v.BkName, SubTitle2: "Serves:  " + v.Serves, Text: v.Quantity}
				} else {
					var subTitle2 string
					if a := strings.Split(v.Authors, ","); len(a) > 1 {
						subTitle2 = "Authors: " + v.Authors
					} else {
						subTitle2 = "Author: " + v.Authors
					}
					item = DisplayItem{Id: id, Title: v.RName, SubTitle1: "Book: " + v.BkName, SubTitle2: subTitle2, Text: v.Quantity}
				}
				mchoice = append(mchoice, item)
			}
			s := sessctx
			return RespEvent{Type: "Search", Header: "Search results for: " + s.reqSearch, Text: s.vmsg, Verbal: s.dmsg, List: mchoice}, nil

		case sessctx.dispObjectMenu:
			//case (sessctx.request == "select" || sessctx.request == "search") && len(sessctx.reqRName) > 0:

			if len(sessctx.passErr) > 0 {
				hdr = sessctx.passErr
			} else {
				hdr = sessctx.reqRName
			}
			s := sessctx
			mchoice := make([]DisplayItem, 4)
			//for i, v := range []string{ingredient_, utensil_, container_, task_} {
			for i, v := range []string{"List ingredients", "List utensils", "List containers", `Let's start cooking..`} {
				id := strconv.Itoa(i + 1)
				mchoice[i] = DisplayItem{Id: id, Title: v}
			}
			return RespEvent{Type: "Select", Header: hdr, Text: s.vmsg, Verbal: s.dmsg, List: mchoice}, nil

		default:
			//
			// all other requests simply return data from last session - no display requirements.
			//
			return RespEvent{Text: sessctx.vmsg, Verbal: sessctx.dmsg + sessctx.ddata}, nil
		}
	}
	//
	//  Fetch next instruction record. Currently only recipe instructions are supported.
	//
	err = sessctx.getRecById()
	if err != nil {
		return RespEvent{Text: sessctx.vmsg, Verbal: sessctx.dmsg, Error: err.Error()}, nil
	}
	//
	// respond with next record from task (instruction). May support verbal listing of container, utensils at some stage (current displayed listing only)
	//
	s := sessctx
	if len(s.passErr) > 0 {
		hdr = s.passErr
	} else {
		hdr = " Cooking Instructions  -  " + s.reqRName
		subh = strconv.Itoa(s.objRecId) + " of " + strconv.Itoa(s.eol)
		if len(s.part) > 0 {
			subh += "    Part: " + s.part + "  -  " + strconv.Itoa(s.pid) + " of " + strconv.Itoa(s.peol)
		}
	}
	//split instructions across three lists
	//
	var listA []DisplayItem
	for k, i, ir := 0, s.objRecId-3, s.instructions; k < 3; k++ {
		if i >= 0 {
			item := DisplayItem{Title: ir[i].Text}
			listA = append(listA, item)
		}
		i++
	}
	if len(listA) == 0 {
		listA = []DisplayItem{DisplayItem{Title: " "}}
	}
	listB := make([]DisplayItem, 1)
	listB[0] = DisplayItem{Title: s.instructions[s.objRecId].Text}
	listC := make([]DisplayItem, len(s.instructions)-s.objRecId)
	for k, i, ir := 0, s.objRecId+1, s.instructions; i < len(ir); i++ {
		listC[k] = DisplayItem{Title: ir[i].Text}
		k++
	}
	type_ := "Tripple"
	if len(s.instructions[s.objRecId].Text) > 120 {
		type_ = "Tripple2" // larger text bounding box
	}
	return RespEvent{Type: type_, Header: hdr, SubHdr: subh, Text: sessctx.vmsg, Verbal: sessctx.dmsg, ListA: listA, ListB: listB, ListC: listC}, nil
}

func main() {
	//lambda.Start(handler)
	p1 := InputEvent{Path: os.Args[1], Param: "sid=asdf-asdf-asdf-asdf-asdf-987654&bkid=" + os.Args[2] + "&rid=" + os.Args[3]}
	//p1 := InputEvent{Path: os.Args[1], Param: "sid=asdf-asdf-asdf-asdf-asdf-987654&rcp=Take-home Chocolate Cake"}
	//var i float64 = 1.0
	// p1 := InputEvent{Path: os.Args[1], Param: "sid=asdf-asdf-asdf-asdf-asdf-987654&bkid=" + "&srch=" + os.Args[2]}
	// //
	pIngrdScale = 1.0
	writeCtx = uDisplay
	p, _ := handler(p1)
	if len(p.Error) > 0 {
		fmt.Printf("%#v\n", p.Error)
	} else {
		fmt.Printf("output:   %s\n", p.Text)
		fmt.Printf("output:   %s\n", p.Verbal)
	}
}
