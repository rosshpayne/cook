package main

import (
	"context"
	_ "encoding/json"
	"fmt"
	"log"
	"net/url"
	_ "os"
	"strconv"
	"strings"

	"github.com/rosshpayne/cook/global"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-lambda-go/lambdacontext"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
)

//TODO
// change float32 to float64 as this is what dynamoAttribute.Unmarshal uses

// Session Context - assigned from request and Session table.
//  also contains state information relevant to current session and not the next session.
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
	//
	Part string
}

type mRecipeM map[mRecipeT]bool

type sessCtx struct {
	//	err        error
	alxReqId  string
	invkReqId string
	//
	newSession bool
	//path      string // InputEvent.Path
	request string // pathItem[0] or redirected value
	lastreq string // previous request (not current one)
	origreq string // original pathItem[0] before redirect if used
	//param     string // InputEvent.Param
	state     stateStack // read from dynamo by getState, commited to dynamo by saveState
	saveState bool
	//
	lastState *stateRec // state attribute from state dynamo item - contains state history
	//
	userId      string   // sourced from request. Used as PKey to Sessions table
	bkids       []string // registered books to user
	reqRName    string   // requested recipe name - query param of recipe request
	reqBkName   string   // requested book name - query param
	CloseBkName string
	reqRId      string // Recipe Id - 0 means no recipe id has been assigned.  All RId's start at 1.
	reqBkId     string
	reqVersion  string   // version id, starts at 0 which is blank??
	reqSearch   string   // search value
	recId       [4]int   // record id for each object (ingredient, container, utensils, containers). No display will use verbal for all object listings.
	pkey        string   // primary key
	recipe      *RecipeT //  record from dynamo recipe query
	rsearch     bool     // searchR
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
	gotoRecId int // sourced from request
	//objRecId       int  // current record id for object. Object is a ingredient,task,container,utensil.- displayed record id persisted to session after use.
	recIdNotExists bool // determines whether to create []RecId set attribute in Session  table
	//noGetRecRequired bool        // a mutliple record request e.g. ingredients listing
	recipeList RecipeListT // multi-choice select. Recipe name and ingredient searches can result in mutliple records being returned. Results are saved.
	recipeMap  mRecipeM
	//
	selCtx selectCtxT // select context either recipe or other (i.e. object)k,
	selId  int        // value selected by user of index in itemList
	//selClear bool
	//
	showList  bool        // show what ever is in the current list (books, recipes)
	ingrdList IngredientT // output of activity.String() - ingredient listing
	// vPreMsg        string
	// dPreMsg        string
	displayHdr    string // passed to alexa display
	displaySubHdr string // passed to alexa display
	activityS     Activities
	dmsg          string // display msg
	derr          string // display error
	vmsg          string // verbal msg
	ddata         string
	yesno         string
	//
	displayData Displayer
	//instructions []InstructionT // cached instructions for complete or part based recipe/
	// Recipe Part data
	eol   int    // sourced from Sessions table
	peol  int    // End-of-List-for-part
	part  string // part index/long name depending on context - if no value then no part is being used eventhough recipe may be have a part defined i.e nopart_ & a part
	parts PartS  // sourced from Recipe (R-) - contains part, division and thread values
	// cPart string  // current part being display (long name). All part means complete recipe will be listed.
	// next  int     // next SortK (recId)
	// prev  int     // previous SortK (recId) when in part mode as opposed to full recipe mode
	pid int // record id within a part 1..peol
	// //
	back bool // back button pressed on display
	//
	showObjMenu bool
	//
	cThread int // current thread
	oThread int // other active thread
	//instrId     // instruction id
	//
	menuL   menuList
	dispCtr *DispContainerT
	//dimension int // size of user container in scaling mode
	scalef  float64
	reScale bool // true in scaling mode
	//
	//display *apldisplayT
	welcome *WelcomeT
	//
	email string
	//
	reqOpenBk    BookT // BkId|BkName|authors
	openBkChange bool
	//	tryOpenBk    bool
	action string // list actions: ingredients, containers, instructions
	ctSize int    // resize a container
}

const scaleThreshold float64 = 0.9

const (
	// objects to which future requests apply - s.object values
	ingredient_     string = "ingredient"
	task_           string = "task"
	container_      string = "container"
	CtrMsr_         string = "sizeContainer"
	recipe_         string = "recipe" // list recipe in book
	CompleteRecipe_ string = "Complete recipe"
	part_           string = "part"
)

type selectCtxT int

const (
	ctxRecipeMenu selectCtxT = iota + 1
	ctxObjectMenu
	ctxPartMenu
)

const (
	// user request grouped into three types
	initialiseRequest = iota + 1
	objectRequest
	instructionRequest
)

type objectMapT map[string]int

var objectMap objectMapT

var objectS []string

func (s *sessCtx) closeBook() error {
	//
	// close books sends user back to first screen/dialog - a reset in essense
	//
	fmt.Println("closeBook()")
	//s.openBkChange = true
	s.CloseBkName = s.reqBkName
	//
	err := s.restart()
	if err != nil {
		return err
	}

	return nil
}

func (s *sessCtx) restart() error {

	fmt.Println("restart...") //
	curRequest := s.request
	// remove state history, back to start request
	//
	if len(s.state) > 1 {
		s.state = s.state[0:2]
		s.popState() // will set request to "start" assigned from stateRec[0].Request.
	}
	//
	//  now lets clear some state attributes in the remaining state
	//
	if curRequest == "close" {
		s.reqBkId, s.reqBkName, s.reqRId, s.reqRName = "", "", "", ""
		s.reqOpenBk, s.authorS, s.authors = "", nil, ""
		s.eol, s.peol = 0, 0
	}
	s.back = true // condition used in display()
	//
	s.updateState()
	//
	s.welcome = s.state[0].Welcome
	s.displayData = s.welcome

	return nil
}

func (s *sessCtx) openBook() error {

	//s.tryOpenBk = false
	s.openBkChange = true
	err := s.bookNameLookup()
	if err != nil {
		return err
	}
	s.reqOpenBk = BookT(s.reqBkId + "|" + s.reqBkName + "|" + s.authors)
	s.eol, s.peol = 0, 0
	s.reqRName, s.reqRId = "", ""
	//s.displayData = s.reqOpenBk
	//s.newSession = true
	fmt.Println("openBook() - ", s.reqOpenBk)
	s.cThread, s.oThread = 0, 0
	s.updateState()
	return nil
}

func (s *sessCtx) clearForSearch(lastState *stateRec) {
	s.recipeList = nil
	s.reqRName, s.reqRId, s.reqVersion = "", "", ""
	s.recId = [4]int{0, 0, 0, 0}
	s.authors, s.authorS = "", nil
	s.serves = ""
	s.menuL = nil
	s.indexRecs = nil
	s.object = ""
	//s.objRecId = 0
	s.recipeList = nil
	s.selCtx, s.selId = 0, 0
	s.ingrdList = ""
	s.eol, s.peol, s.part, s.parts, s.pid = 0, 0, "", nil, 0
	s.back = false
	// if no open book unset Book details. For open book case, these are populated during setState()
	if len(lastState.OpenBk) == 0 {
		s.reqBkId, s.reqBkName = "", ""
	}
	s.cThread, s.oThread = 0, 0
}

// func selectCtxRequired(r string) bool {
// 	switch r {
// 	case "restart", "book", "close", "search", "start", "resize", "scale", "list":
// 		return false
// 	default:
// 		return true
// 	}
// }
func init() {
	objectMap = make(objectMapT, 4)
	for i, v := range []string{ingredient_, task_, container_, CtrMsr_, recipe_} {
		objectMap[v] = i
	}
	objectS = make([]string, 4)
	for i, v := range []string{ingredient_, container_, CtrMsr_, task_} {
		objectS[i] = v
	}
}

// type alexaDialog interface {
// 	Alexa() dialog
// }
// type dialog struct {
// 	Verbal  string
// 	Display string
// 	EOL     int
// 	PID     int // instruction ID within part
// 	PEOL    int
// 	PART    string
// }

func (s *sessCtx) orchestrateRequest() error {
	//
	// ******************************** 	initialise state		****************************************
	//
	// fetch state from last session
	//
	lastState, err := s.getState()
	if err != nil {
		return err
	}
	// set current state based on last session
	s.setSessionState(lastState)

	// if selectCtxRequired(s.request) {
	// 	s.incrSelectCtx(lastState)
	// }
	//
	// ******************************** process responses to request  ****************************************
	//
	if s.request == "restart" {
		err := s.restart()
		if err != nil {
			return err
		}
		return nil
	}
	//
	// alexa launch request
	//
	if s.request == "start" {
		s.request = lastState.Request
		fmt.Println("in start....................")
		fmt.Println("s.request, origreq  = ", s.request, s.origreq)
		fmt.Println("s.email = ", s.email)
		if s.dispCtr == nil {
			fmt.Println("at start: s.dispCtr is NIL")
		} else {
			fmt.Printf(" at start: s.dispCtr %#v\n", s.dispCtr)
		}
		switch wx := s.displayData.(type) {
		case Threads:
			fmt.Println("request start: displayData: Threads")
			return nil

		case *WelcomeT:
			fmt.Println("displayData: *WelcomeT")
			// no previous session - check if userId is registered
			// check for row U-[userId] in ingredients table. This item contains books registered to userId
			//  if no data then user is not registered and therefore ineligble to use app
			if len(s.email) > 0 {
				// resp from startwithEmail. (see below) for the case of no registerd books.
				var w WelcomeT
				w.msg = fmt.Sprintf("Hi, you have no books registered. \n\nPlease go to www.eburypress.co.uk and register your book purchase, using the email;\n\n%s ", s.email)
				s.displayData = &w
				return nil
			}
			if wx != nil {
				return nil
			}
			switch len(s.bkids) {
			case 0:
				fmt.Println("No books found...")
				// user has no registered books. Get index.js to supply email and start again using startWithEmail
				var w WelcomeT
				w.msg = "no books found. There was an error with getting permissions to your email. Please say yes."
				w.request = "email"
				s.displayData = &w
				return nil
			default:
				var w WelcomeT
				s.displayData = &w
				return nil
			}
			// default:
			// should let it fall through as setState may not have assigned displayData but relies on other state data
			//  to determine what should be displayed - so let it fall through and not return at this point
			// 	if s.displayData == nil {
			// 		panic("error: displayData is nil - setState should have assigned it a value but didnot")
			// 	}
			// 	return nil
		case ObjMenu:
			// displayData assigned in setState()
			fmt.Println("displayData: ObjMenu")
			return nil
		case PartS:
			if s.selId == 0 { // no selection yet made so display parts menu
				return nil
			}
		case IngredientT:
			// reset request - Ingredient can be restarted (read data from cache)
			// rather than start from beginning ie. requited to orchestrate the data
			s.request = s.origreq
		}
	}

	if s.request == "list" {
		//
		fmt.Println(`"process  "list"`)
		if len(lastState.RecipeList) > 0 || len(s.state) == 1 {
			fmt.Println("Cannot list from this context - must choose a recipe first")
			s.derr = `*** Alert: Cannot list from this context -  choose a recipe first. Say "find recipe" or "restart" or "open book"`
			// displayData set in setState()
			return nil
		}
		//
		// rollup to objMenu
		//
		for showObjMenu := s.showObjMenu; !showObjMenu; showObjMenu = s.showObjMenu {
			s.popState() // will set request to "start" assigned from stateRec[0].Request.
			if s.request == "start" {
				err = fmt.Errorf("Internal Error:  list request failed to popState() to objMenu")
				break
			}
		}
		if err != nil {
			return err
		}
		s.displayData = nil
		// redirect list request to select
		s.request = "select"
		s.selCtx = ctxObjectMenu // 2
		fmt.Println("s.action: ", s.action)
		switch s.action {
		case "ingredients", "ingredient":
			s.selId = 1
		case "containers", "container":
			s.selId = 2
		case "size":
			if s.displayData == nil {
				s.displayData = s.dispCtr
			}
			switch len(s.menuL) {
			case 4:
				s.selId = 3
			default:
				fmt.Println("*** Resizing the container is not available for this recipe")
				s.derr = `*** Alert: Resizing the container is not available for this recipe`
			}
		case "instructions", "instruction", "start cooking", "steps", "step":
			s.selId = len(s.menuL)
			// if part menu is available then find appropriate selId based on s.part and set selCtx to part menu
			// 0 index is CompleteRecipe_
			if len(s.parts) > 0 {
				//s.selCtx = ctxPartMenu
				s.selCtx = ctxObjectMenu
				// find item in Part Menu for s.part
				//s.selId = 0
				s.selId = len(s.menuL)
				if len(s.part) > 0 {
					s.selCtx = ctxPartMenu
					if s.part == CompleteRecipe_ {
						s.selId = 1
					} else {
						s.selId = 1
						for i, prtName := 1, s.parts[0].Title; prtName != s.part && i < len(s.parts); i++ {
							prtName = s.parts[i].Title
							s.selId = i + 1
						}
						s.selId++ // select id starts at 1
					}
				}
			}
		case "parts":
			if len(s.part) > 0 {
				s.part = ""
			}
			if len(s.parts) > 0 {
				s.selId = len(s.menuL)
			} else {
				// no parts
				s.derr = `*** Alert:  Recipe is not divided into parts. Say "list instructions" to start cooking`
				s.request = s.lastreq
				s.selCtx = lastState.SelCtx
				s.selId = lastState.SelId
			}
		default:
			fmt.Println("Invalid action ", s.action)
			s.derr = `Invalid list action. Valid actions are "ingredient","container","instructions", "parts"`
		}
	}

	if s.request == "resize" { // user enters "size [integer]" only from container size screen
		//
		// user sepecified size of container e.g "size 23"
		if s.object != "sizeContainer" {
			fmt.Println("Cannot change container size from this context - must choose a recipe first")
			s.derr = `*** Alert: Cannot change container size from this context -  Say "list size" and then "size [integer]" to size your container`
			// user lastState selCtx & selId to complete processing
		} else {
			c := s.dispCtr
			cdim, err := strconv.Atoi(c.Dimension)
			if err != nil {
				panic(err.Error())
			}
			fmt.Println("Dimension: cdim = ", cdim)
			fmt.Println("user entered dimension = ", s.ctSize)
			if float64(s.ctSize)/float64(cdim) < 0.6 {
				return fmt.Errorf("Resize cannot be less than 60% of original container size")
			}
			s.displayData = s.dispCtr
			s.reset = true
			s.showObjMenu = false
			s.curReqType = 0
			// NB. updateState is executed from dispCtr.GenDisplay()
			return nil
		}
	}
	//
	// check if scale requested and state is listing ingredients, otherwise ignore
	//
	if s.request == "scale" {
		fmt.Println(" save scale - updateState() ")
		s.reScale = true
		//
		//  define current state
		//
		// s.request = lastState.Request
		// fmt.Println("lastState.Request (now s.request) = ", lastState.Request) . /// should ALL be set in setState()
		// s.selId = lastState.SelId
		// fmt.Println("lastState.SelId (now s.request) = ", lastState.SelId)
		// s.selCtx = lastState.SelCtx
		// fmt.Println("lastState.selCtx (now s.selCtx) = ", lastState.SelCtx)
		// s.object = lastState.Obj
		//
		// error if state/screen does not permit to change scale
		//
		switch s.displayData.(type) {
		case Threads:
			s.derr = ` *** Alert :  you cannot scale a recipe while following instructions. Say "go back" or "restart" and scale from there.`
			return nil
		default:
			if s.object == "sizeContainer" {
				s.derr = ` *** Alert :  you should scale a recipe using the size of your container. Say "size [integer]" or "restart" or a "list" option.`
				s.displayData = s.dispCtr
				return nil
			}
			global.SetScale(s.scalef)
			if s.showObjMenu {
				s.displayData = objMenu
				s.updateState()
				return nil
			}
			s.request = s.lastreq
			//return nil - needs to use selId & selCtx to regen instructions
		}
	}
	//
	// yes/no response. May assign new book
	//
	fmt.Println("yesno: ", s.yesno)
	fmt.Println("selCtx: ", s.selCtx)
	fmt.Println("selId: ", s.selId)
	fmt.Println("showObjMenu: ", s.showObjMenu)
	fmt.Println("request = ", s.request)
	fmt.Println("Qid: ", lastState.Qid)

	if lastState.Qid > 0 && (len(s.yesno) > 0 || (s.selId == 1 || s.selId == 2)) {

		var err error
		switch lastState.Qid {
		case 20:
			// close active book
			//  as req struct fields still have their zero value they will clear session state during pushState
			if s.selId == 1 {
				s.reset = true
				s.vmsg = fmt.Sprintf(`%s is closed. You can now search across all recipe books`, lastState.BkName)
				s.dmsg = fmt.Sprintf(`%s is closed. You can now search across all recipe books`, lastState.BkName)
				s.reqBkId, s.reqBkName, s.reqRName, s.reqRId, s.eol, s.reset = "", "", "", "", 0, true
			} else {
				s.popState()
			}
		case 21:
			if s.selId == 1 {
				// open book
				fmt.Println(">> Yes to Open Book")
				s.vmsg = fmt.Sprintf(`[%s] is now open. You can now search or open a recipe within this book`, lastState.BkName)
				s.dmsg = fmt.Sprintf(`[%s] is now open. You can now search or open a recipe within this book`, lastState.BkName)
				err = s.openBook()
				if err != nil {
					return err
				}
			} else {
				s.popState()
			}
		case 22:
			// swap to this book
			if s.selId == 1 {
				s.reqBkId, s.reqBkName, s.reset = lastState.SwpBkId, lastState.SwpBkNm, true
				s.vmsg = fmt.Sprintf(`Book [%s] is now open. You can now search or open a recipe within this book`, lastState.BkName)
				s.dmsg = fmt.Sprintf(`Book [%s] is now open. You can now search or open a recipe within this book`, lastState.BkName)
				err = s.openBook()
				if err != nil {
					return err
				}
			} else {
				s.popState()
			}
		default:
			// TODO: log error to error table
		}
		// if s.selId == 2 || s.yesno == "no" {

		// }
		if len(s.dmsg) == 0 {
			s.dmsg = lastState.Dmsg
		}
		//
		s.ingrdList, s.object, s.selCtx, s.showObjMenu = "", "", 0, false
		s.pushState()
		return nil
	}
	//
	// open book
	//
	if s.request == "book" {
		//
		// open book requested
		//
		// if _, ok := s.displayData.(BookT); ok { // this requires a BookT screen. Decided not to use it and use header text instead
		// 	// assigned during setState() - no more to do so return
		// 	fmt.Println("Request Book: displayData already assigned return")
		// 	return nil
		// }
		//
		// use existing screen (displayData) and update header with the following messages
		//
		//s.tryOpenBk = true
		fmt.Println("orachestrate for request book...")
		//
		if len(lastState.BkId) > 0 {
			fmt.Println("Book Processing...BkId, reqBkId  ", lastState.BkId, s.reqBkId)
			fmt.Println("Book Processing...Request , request ", lastState.Request, s.request)

			// a book is currently been accessed
			switch {
			case lastState.Request != "start":
				s.dmsg = `You must be at the start to open a book. Please say "restart" and then open your book from there. Otherwise continue.`
				s.vmsg = `You must be at the start to open a book. Please say "restart" and then open your book from there. Note this will cancel what you are doing.`

			case lastState.BkId == s.reqBkId:
				// open currenly opened book. reqBkId was sourced using bookNameLookup()
				if lastState.activeRecipe() && len(lastState.OpenBk) == 0 {
					s.dmsg = "You are actively browsing a recipe from this book. Do you still want to open " + s.reqBkName + "?"
					s.vmsg = "You are actively browsing a recipe from this book. Do you still want to open " + s.reqBkName + "?"
					s.questionId = 21
				}
				if len(lastState.OpenBk) > 0 {
					s.dmsg = `*** Please close ` + lastState.BkName + ` before opening another book, by saying "close book", otherwise just continue.`
					s.vmsg = s.dmsg[4:]
					if lastState.Request != "start" {
						s.vmsg = s.vmsg + " Closing will also cancel what you are doing at the moment."
					}
				}

			case lastState.BkId != s.reqBkId:
				if len(lastState.RName) == 0 {
					// no active recipe
					s.dmsg = "Opened " + s.reqBkName + " by " + s.authors + ". "
					s.vmsg = "Opened " + s.reqBkName + " by " + s.authors + ". "
					err = s.openBook()
					if err != nil {
						return err
					}
				} else if lastState.activeRecipe() {
					s.dmsg = "You are actively browsing a recipe from book, " + lastState.BkName + ". Do you still want to open " + s.reqBkName + "?"
					s.vmsg = "You are actively browsing a recipe from book, " + lastState.BkName + ". Do you still want to open " + s.reqBkName + "?"
					s.questionId = 21
				}
			}
		} else {
			// no book is currently been accessed
			err = s.openBook()
			if err != nil {
				return err
			}
			s.dmsg = "Opened " + s.reqBkName + " by " + s.authors + ". "
			s.vmsg = "Opened " + s.reqBkName + " by " + s.authors + ". "
		}
		fmt.Println("Book msg: ", s.dmsg)
		//s.ingrdList, s.recipeList, s.object, s.showObjMenu = "", nil, "", false
		//return nil
	}
	//
	// close book
	//
	if s.request == "close" {
		if len(s.object) > 0 && s.eol != s.recId[objectMap[s.object]] {
			s.dmsg = fmt.Sprintf("You currently have recipe %s open. Do you still want to close the book?", lastState.RName)
			s.vmsg = fmt.Sprintf("You currently have recipe %s open. Do you still want to close the book?", lastState.RName)
			fmt.Println("in close: ", s.dmsg)
			s.questionId = 20
		} else {
			switch len(s.reqOpenBk) {
			case 0:
				s.dmsg = `There is no books open to close.`
				s.vmsg = `There is no books open to close.`
				fmt.Println("in close: ", s.dmsg)
			default:
				//
				s.dmsg = s.reqBkName + ` is now closed. All searches will be across all books`
				s.vmsg = s.reqBkName + ` is now closed. All searches will be across all books`
				err := s.closeBook()
				if err != nil {
					return err
				}
				fmt.Println("in close: ", s.dmsg)
			}
		}
		return nil
	}
	//
	//
	//

	if s.request == "search" {

		search := func(srch string) error {
			if srch[len(srch)-1] == 's' {
				srch = srch[:len(srch)-1]
			} else {
				srch = srch + "s"
			}
			fmt.Println(" search for: ", srch)
			err = s.keywordSearch(srch)
			if err != nil {
				return err
			}
			return nil
		}

		s.clearForSearch(lastState)
		fmt.Println("Search.....", s.reqSearch)
		fmt.Println("Before BookId, RecipeId ", s.reqBkId, s.reqRId)
		//
		s.recipeList = nil
		//
		srch := s.reqSearch
		for _, v := range []string{" of ", " with ", " the ", " has ", " and ", " recipe ", " recipes "} {
			srch = strings.Replace(" "+srch, v, " ", -1)
		}
		srch = strings.Replace(strings.TrimSpace(srch), "  ", " ", -1)
		//
		err := s.keywordSearch(srch)
		if err != nil {
			return err
		}
		//
		// reverse words - even if found previously
		// .e.g. user enteres "rhubarb tarragon" then also search for "tarragon rhubarb"
		// as some recipes may use the reverse.
		//
		f := strings.Fields(srch)
		switch len(f) {
		case 2:
			err = s.keywordSearch(f[1] + " " + f[0])
		case 3:
			err = s.keywordSearch(f[2] + " " + f[1] + " " + f[0])
			if err != nil {
				return err
			}
			err = s.keywordSearch(f[1] + " " + f[0] + " " + f[2])
			if err != nil {
				return err
			}
		}
		//
		if len(s.recipeList) == 0 {
			search(srch)
		}
		//
		s.eol, s.reset, s.object = 0, true, ""
		s.showObjMenu = false
		fmt.Println("after search: BookId, RecipeId ", s.reqBkId, s.reqRId)
		s.pkey = s.reqBkId + "-" + s.reqRId
		switch len(s.recipeList) {

		case 0:
			// nothing found
			if len(s.reqOpenBk) > 0 {
				s.derr = `*** No recipe found in ` + s.reqBkName + ` matching "` + s.reqSearch + `". `
			} else {
				s.derr = `*** No recipe found in any of your books matching "` + s.reqSearch + `". `
			}
			words := strings.Split(s.reqSearch, " ")
			if len(words) == 1 {
				s.derr += "Try multiple keywords e.g search orange chocolate tart. "
			} else {
				s.derr += "Change the order of the keywords or try alternative keywords. "
			}
			s.derr += "Otherwise continue."
			return nil

		case 1:
			// one recipe found
			fmt.Println("redirect to ctxRecipeMenu for processing")
			s.request = "select"
			s.selCtx = ctxRecipeMenu
			s.selId = 1
			s.authors = s.recipeList[0].Authors
			s.reqBkName = s.recipeList[0].BkName
			s.reqRId = s.recipeList[0].RId
			s.reqBkId = s.recipeList[0].BkId
			s.reqRName = s.recipeList[0].RName

		default:
			// many recipes found
			s.curReqType = 0
			s.displayData = s.recipeList
			s.selCtx = ctxRecipeMenu
			s.selId = 0
			fmt.Printf("recipe List found [%#v]\n", s.recipeList)
			if s.origreq != "start" {
				s.pushState()
			}
			return nil
		}
	}
	//
	if s.request == "resume" {
		if s.cThread == 0 || s.object != "task" || s.reqRId == "0" {
			s.derr = "There is nothing to resume"
			return nil
		}
		if t, ok := s.displayData.(Threads); ok {
			if s.oThread == -1 {
				s.derr = "All is completed. There is nothing to resume"
				return nil
			}
			if t[s.oThread].Id == len(t[s.oThread].Instructions) {
				s.derr = "All is completed. There is nothing to resume"
				return nil
			}
			x := s.cThread
			s.cThread = s.oThread
			s.oThread = x
			t[s.cThread].Id++
			s.recId[objectMap[s.object]] = t[s.cThread].Id
		} else {
			s.derr = "There is nothing to resume"
		}
		return nil
	}
	if s.curReqType == instructionRequest {
		// object is same as last call
		s.object = s.object
	}
	//
	//  cooking instructions navigation commands
	//
	switch s.request {
	case "goto":
		fmt.Printf("gotoRecId = %d  %d\n", s.gotoRecId, lastState.EOL)
		//s.objRecId = s.gotoRecId
		s.recId[objectMap[s.object]] = s.gotoRecId
		return nil
	case "repeat":
		// return - no need to pushState as nothing has changed.  Select current recId.
		//s.objRecId = s.recId[objectMap[s.object]]
		s.dmsg, s.vmsg, s.ddata = lastState.Dmsg, lastState.Vmsg, lastState.DData
		return nil
	case "prev":
		if len(s.object) == 0 {
			return fmt.Errorf("Error: no object defined for previous in orchestrateRequest")
		}
		//s.objRecId = s.recId[objectMap[s.object]] - 1
		s.recId[objectMap[s.object]] -= 1
		return nil
	case "next", "select-next":
		if len(s.recId) == 0 {
			s.recId = [4]int{}
		}
		//
		if len(s.object) == 0 {
			return fmt.Errorf("Error: no object defined for next in orchestrateRequest")
		}
		// recId contains last processed instruction. So must add one to get current instruction.
		//	s.objRecId = s.recId[objectMap[s.object]] + 1
		s.recId[objectMap[s.object]] += 1
		return nil
	}
	//
	// respond to select from displayed items
	//
	if s.request == "select" || s.request == "start" { //&& s.selId > 0 { selId==0 when partmenu is being displayed from start
		// selId is the response from Alexa on the index (ordinal value) of the selected display item
		fmt.Println("SELCTX is : ", s.selCtx)

		switch s.selCtx {
		case ctxRecipeMenu:
			// select from: multiple recipes
			if !s.reScale {
				// select id only checked for genuine selection which doesn't happen if request is scale.
				if s.selId > len(s.recipeList) || s.selId < 1 {
					s.derr = "selection is not within range"
					return nil
				}
				s.rsearch = false
				p := s.recipeList[s.selId-1]
				s.reqRId, s.reqRName, s.reqBkId, s.reqBkName = p.RId, p.RName, p.BkId, p.BkName
				// s.dmsg = fmt.Sprintf(`Now that you have selected [%s] recipe would you like to list ingredients, cooking instructions, utensils or containers or cancel`, s.reqRName)
				// s.vmsg = fmt.Sprintf(`Now that you have selected {%s] recipe would you like to list ingredients, cooking instructions, utensils or containers or cancel`, s.reqRName)
				// chosen recipe, so set select context to object (ingredient, utensil, container, tas			s.selCtx = ctxObjectMenu
				//
				s.parts = nil
				_, err := s.recipeRSearch()
				if err != nil {
					return err
				}
			}
			s.displayData = objMenu
			s.selId = 0
			s.showObjMenu = true
			s.recipeList = nil
			s.selCtx = ctxObjectMenu

			s.reset = true
			if !s.reScale {
				fmt.Println("ctxRecipeMenu: about to pushState")
				s.pushState()
			} else {
				s.updateState()
			}

			return nil

		case ctxObjectMenu:
			s.pkey = s.reqBkId + "-" + s.reqRId
			//	select from: list ingredient, list utensils, list containers, start cooking
			fmt.Println("** selId: ", s.selId)
			// if s.reScale {
			// 	// current operation is a rescale - which only applies to ingredients so set Ingredient
			// 	s.object = ingredient_
			// } else {
			if !s.reScale {
				// check select id is within menu range. For reScale there is no menu listing in its current state.
				if s.selId > len(s.menuL) {
					//s.setDisplay(lastState)
					s.derr = "selection is not within range"
					return nil
				}
				s.object = objectS[s.menuL[s.selId-1]]
			}
			fmt.Println("SElected: ", s.object)
			// clear recipeList which is not necessary in this state. Held in previous state.
			s.recipeList = nil
			// object chosen, nothing more to select for the time being
			//s.selCtx = 0
			switch s.object {

			//  "lets start cooking" selected
			case task_:
				//s.dispCtr = nil - must not nullify as instructions does an updateState that will nullify all dispCtr
				//s.selClear = true //TODO: what if they back at this point we have cleared sel.
				fmt.Printf("s.parts  %#v [%s] \n", s.parts, s.part)
				s.showObjMenu = false
				if len(s.parts) > 0 && len(s.part) == 0 { //&& (s.origreq == "select" || s.origreq == "list")) { //|| (s.origreq == "list" && s.action == "parts") {
					//s.dispPartMenu = true
					s.displayData = s.parts
					s.selCtx = ctxPartMenu
					s.selId = 0
					s.curReqType = 0
					//
					if s.request == "select" {
						if s.reScale {
							s.updateState()
						} else {
							s.pushState()
						}
					}
					return nil
				} else {
					// go straight to instruction
					s.displayData, err = s.loadInstructions()
					if err != nil {
						return err
					}
					//
					if s.request == "select" {
						s.pushState()
					}
					// now complete the request by morphing request to a "next" operation
					s.reset = true
					s.request = "select-next"
					s.curReqType = instructionRequest
					s.object = "task"
				}

			//  "list ingredients" selected
			case ingredient_:
				fmt.Println("Here in ingredient_.. about to read activity table and generated ingredient string")
				s.dispCtr = nil
				switch s.request {
				case "select":
					if s.reqVersion != "" {
						s.pkey += "-" + s.reqVersion
					}
					//if recipe name is not known get it
					var r *RecipeT
					r = s.recipe
					if len(s.reqRName) == 0 || s.recipe == nil {
						r, err = s.recipeRSearch()
						if err != nil {
							return err
						}
						fmt.Printf("receipeRSearch returned: %#v", r)
					}
					// set unit formating mode
					global.Set_WriteCtx(global.UPrint)
					// load recipe data, part metadata, containers etc
					as, err := s.loadActivities()
					if err != nil {
						return err
					}
					fmt.Println("ingredient_: num Activities = ", len(as))
					// generate ingredient listing
					global.Set_WriteCtx(global.UIngredient)
					s.ingrdList = IngredientT(as.String(r))
					s.displayData = s.ingrdList
					s.showObjMenu = false

					if s.reScale {
						s.updateState()
					} else {
						s.pushState()
					}

				case "start", "list":
					s.ingrdList = IngredientT(s.ingrdList)
				}
				//
				s.reset = true
				s.curReqType = 0
				//
				return nil

			case container_:
				fmt.Println("Here in container.. about to loadBaseContainers")
				//s.dispContainers = true
				s.dispCtr = nil
				global.Set_WriteCtx(global.UPrint)
				s.displayData, err = s.loadBaseContainers()
				if err != nil {
					return err
				}
				s.showObjMenu = false
				s.curReqType = 0
				if s.reScale {
					s.updateState()
				} else if s.request == "select" {
					s.pushState()
				}
				return nil

			case CtrMsr_:
				//do not set s.showObjMenu = false, as dispCtr.GenDisplay() performs updateState() rather than pushState()
				fmt.Printf("CtrMsr - %#v\n", *(s.dispCtr))
				s.showObjMenu = false
				s.displayData = s.dispCtr
				return nil
			}

		// recipe part menu
		case ctxPartMenu:
			if s.selId == 0 { // no selection made so display menu
				s.displayData = s.parts
				return nil
			}
			//s.dispCtr = nil TODO check...
			if s.selId > len(s.parts)+1 || s.selId < 1 {
				//s.setDisplay(lastState)
				s.derr = "selection is not within range"
				return nil
			}
			curPart := s.part
			//s.part = ""
			s.object = task_
			switch s.selId {
			case 1:
				s.part = CompleteRecipe_
			default:
				s.part = s.parts[s.selId-2].Title
				if s.parts[s.selId-2].Type_ == "Div" {
					s.cThread, s.oThread = 0, 0
				}
			}
			// zero index into instructions if part changed
			if s.part != curPart {
				s.recId[objectMap[s.object]] = 0
			}
			fmt.Printf("^^^ selId  %d   parts   %#v\n", s.selId, s.part)
			s.reset = true
			//s.recId = [4]int{0, 0, 0, 0}
			s.showObjMenu = false
			s.displayData, err = s.loadInstructions()
			if err != nil {
				return err
			}
			if s.request == "select" {
				if s.part != lastState.Part {
					s.updateState() // save part name to state upto objMenu
				}
				s.pushState() // create new in-memory leaf which will be updated in display which updates leaf and back upto to objMenu node.
			}
			// now complete the request by morphing request to a next operation
			s.request = "select-next"
			s.curReqType = instructionRequest
		}
	}
	//
	return nil
}

func (s *sessCtx) initialiseRequest_() bool {
	switch s.request {
	case "book", "close", "recipe", "select", "search", "list", "yesno", "version", "resume", "resize", "scale":
		s.curReqType = initialiseRequest
		return true
	default:
		return false
	}
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
		fmt.Println(v)
		param := strings.Split(v, "=")
		r.QueryStringParameters[param[0]] = param[1]
	}
	r.PathItem = strings.Split(r.Path, "/")
}

func handler(ctx context.Context, request InputEvent) (RespEvent, error) {
	//func handler(request InputEvent) (RespEvent, error) {
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
	lc, _ := lambdacontext.FromContext(ctx)
	alxreqid := strings.Split(request.QueryStringParameters["reqId"], ".")

	// fmt.Println("Alexa reqId : ", request.QueryStringParameters["reqId"])
	// fmt.Println("invoke reqId : ", lc.AwsRequestID)

	// var body string
	// create a new session context and merge with last session data if present.
	sessctx := &sessCtx{
		userId:      request.QueryStringParameters["uid"], // empty when not called
		alxReqId:    alxreqid[len(alxreqid)-1],
		invkReqId:   lc.AwsRequestID,
		dynamodbSvc: dynamodbService(),
		request:     pathItem[0],
		origreq:     pathItem[0],
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
	fmt.Println("REQUEST: ", sessctx.request)
	switch sessctx.request {
	case "load", "print", "listcontainer", "purge":
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
			//scaleF = 1.0

			fmt.Println("Aboutto loadBaseRecipe()")
			err = sessctx.loadBaseRecipe()
			if err != nil {
				fmt.Println("Error in loadBaseRecipe: ", err.Error())
				break
			}
		case "print":
			//set unit format mode
			global.Set_WriteCtx(global.UPrint)
			as, err := sessctx.loadActivities()
			if err != nil {
				fmt.Printf("error: %s", err.Error())
			}
			fmt.Println("========")
			fmt.Println(as.String(r))
		case "purge":
			err = sessctx.purgeRecipe()
			if err != nil {
				break
			}
		}
		//sessctx.noGetRecRequired= true
		sessctx.reset = true
		return RespEvent{}, nil
	//
	case "addUser":
		bkids := request.QueryStringParameters["bkids"]
		fmt.Printf("bkids: %s\n", bkids)
		sessctx.bkids = strings.Split(bkids, ",")
		err := sessctx.addUserBooks()
		if err != nil {
			panic(err)
		}
		return RespEvent{}, nil
	// case "genSlotValues":
	// 	err = sessctx.generateSlotEntries()
	// 	if err != nil {
	// 		break
	// 	}
	// 	//sessctx.noGetRecRequired, sessctx.reset = true, true
	// 	sessctx.reset = true
	case "startWithEmail":
		sessctx.email = request.QueryStringParameters["email"]
		sessctx.request = "start"

	case "back":
		// used back button on display device. Note: back will ignore orachestrateRequest and go straight to displayGen()
		sessctx.back = true
		fmt.Println("** Back button hit")
		// _, err := sessctx.getState()
		// if err != nil {
		// 	fmt.Println("Error returned by getState()..")
		// 	break
		// }
		err = sessctx.popState()
		if err != nil {
			fmt.Println("Error returned by popState()..")
		}
		//
		// zero some attribute
		//
		s := sessctx
		if s.cThread > 0 || s.recId[1] > 0 || len(s.part) > 0 {
			s.cThread, s.oThread = 0, 0
			s.recId = [4]int{0, 0, 0, 0}
			s.part = ""
			s.updateState()
		}
		// zero these attributes
	case container_, task_:
		//  "object" request is required only for VUI, as all requests are be random by nature.
		//  GUI requests, on the other hand, are controlled by this app making the following request redundant.
		sessctx.object = sessctx.request
		sessctx.curReqType = objectRequest
		sessctx.request = "next"
		fmt.Printf("s.parts  %#v\n", sessctx.parts)
		// if sessctx.request == task_ {
		// 	if len(sessctx.parts) > 0 && len(sessctx.part) == 0 {
		// 		sessctx.dispPartMenu = true
		// 		sessctx.noGetRecRequired = true
		// 	}
		// 	//s.getInstructions()
		// }
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
	case "list":
		sessctx.action = request.QueryStringParameters["action"]

	case "resize": // specify size of your container
		var i int
		i, err = strconv.Atoi(request.QueryStringParameters["size"])
		if err != nil {
			err = fmt.Errorf("%s: %s", "Error in converting int of dimension request \n\n", err.Error())
		} else {
			sessctx.ctSize = i
		}

	}

	if sessctx.initialiseRequest_() && !sessctx.back {
		switch sessctx.request {
		case "book": // user reponse "open book" "close book"
			// book id and name  populated in this section
			sessctx.reqBkId = request.QueryStringParameters["bkid"]
			//err = sessctx.bookNameLookup()
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
			fmt.Println("YesNO: ", sessctx.yesno)
		case "select":
			var i int
			i, err = strconv.Atoi(request.QueryStringParameters["sId"])
			if err != nil {
				err = fmt.Errorf("%s: %s %s", "Error in converting int of select  request. Error:", request.QueryStringParameters["sId"], err.Error())
			} else {
				sessctx.selId = i
			}
		case "scale":
			var f float64
			fmt.Println("Processing scale request: ")
			frac, err := url.QueryUnescape(request.QueryStringParameters["frac"])
			if err != nil {
				panic(err)
			}
			f, err = strconv.ParseFloat(frac, 64)
			if err != nil {
				fmt.Printf(" %s", "Error in converting int of scale request \n\n", err.Error())
				err = fmt.Errorf(" %s", "Error in converting int of scale request \n\n", err.Error())
			} else {
				fmt.Println("scalef = ", f)
				sessctx.scalef = f
				// emulate select ingredient from objMenu, like "select", this will force update to screen
				//sessctx.selId = 1
			}
		}
	}

	if err != nil {
		fmt.Println("::::::: error :::::::")
		//TODO: create an ERROR screen APL
		return RespEvent{Type: "error", Header: "Internal error has occurred", Text: "Error: ", Error: err.Error()}, nil
	}
	//
	// validate the request and populate session context with appropriate metadata associated with request
	//
	if !sessctx.back {
		err = sessctx.orchestrateRequest()
		if err != nil {
			fmt.Println("error after orachestrateRequest ", err.Error())
			return RespEvent{Type: "error", Header: "Internal error has occurred: ", Text: "Error: ", Error: err.Error()}, nil
			//sessctx.derr = err.Error()
		}
	}
	//
	// package the response data RespEvent (an APL aware "display" structure) and return
	//
	if sessctx.displayData == nil {
		sessctx.derr = "displayData not set"
		return RespEvent{Type: "error", Header: "Internal error has occurred: ", Text: "Error: ", Error: err.Error()}, nil
	}
	fmt.Println("=========== displayData.GenDisplay =============")
	//
	var resp RespEvent
	resp = sessctx.displayData.GenDisplay(sessctx)
	//
	if sessctx.saveState && len(sessctx.derr) == 0 {
		err = sessctx.commitState()
		if err != nil {
			panic(err)
		}
	}
	return resp, nil
}

func main() {

	lambda.Start(handler)

	//p1 := InputEvent{Path: os.Args[1], Param: "uid=asdf-asdf-asdf-asdf-asdf-987654&rcp=Take-home Chocolate Cake"}
	//var i float64 = 1.0
	// p1 := InputEvent{Path: os.Args[1], Param: "uid=asdf-asdf-asdf-asdf-asdf-987654&bkid=" + "&srch=" + os.Args[2]}
	// //
	//
	//	var err error
	//	var scaleF float64
	// p1 := InputEvent{Path: os.Args[1], Param: "sid=asdf-asdf-asdf-asdf-asdf-987654&bkid=" + os.Args[2] + "&rid=" + os.Args[3]}

	// uid := `amzn1.ask.account.AFTQJDFZKJIDFN6GRQFTSILWMGO2BHFRTP55PK6KT42XY22GR4BABOP4Y663SUNVBWYABLLQCHEK22MZVUVR7HXVRO247IQZ5KSVNLMDBRDRYEINWGRB6N2U7J2BBWEOEKLY2HKQ6VQTTLGKT2JCH4VOE5A7XPFDI4VMNJW63YP4XCMYGIA5IU4VJGNHI2AAU33Q5J2TJIXP3DI`
	// // p2 := InputEvent{Path: "addUser", Param: "uid=" + uid + "&bkids=20,21"}
	// p2 := InputEvent{Path: os.Args[1], Param: "sid=" + uid + "&bkid=" + os.Args[2] + "&rid=" + os.Args[3]}

	// // if len(os.Args) < 5 {
	// // 	scaleF = 1.0
	// // } else {
	// // 	scaleF, err = strconv.ParseFloat(os.Args[4], 64)
	// // 	if err != nil {
	// // 		panic(err)
	// // 	}
	// // }

	// global.Set_WriteCtx(global.UDisplay)
	// p, _ := handler(p2)
	// if len(p.Error) > 0 {
	// 	fmt.Printf("%#v\n", p.Error)
	// } else {
	// 	fmt.Printf("output:   %s\n", p.Text)
	// 	fmt.Printf("output:   %s\n", p.Verbal)
	// }
}
