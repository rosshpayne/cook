package main

import (
	_ "context"
	_ "encoding/json"
	"fmt"
	"log"
	"net/url"
	_ "os"
	"strconv"
	"strings"

	"github.com/cook/global"

	"github.com/aws/aws-lambda-go/lambda"
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
	err        error
	newSession bool
	//path      string // InputEvent.Path
	request string // pathItem[0]: request from user e.g. select, next, prev,..
	lastreq string // previous request
	//param     string // InputEvent.Param
	state     stateStack
	lastState *stateRec // state attribute from state dynamo item - contains state history
	passErr   bool      // error in logic or operation. display() functions uses this flag to present error to display.
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
	gotoRecId      int  // sourced from request
	objRecId       int  // current record id for object. Object is a ingredient,task,container,utensil.- displayed record id persisted to session after use.
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
	vmsg          string // verbal msg
	ddata         string
	yesno         string
	//
	displayData Displayer
	//instructions []InstructionT // cached instructions for complete or part based recipe
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
		err := s.popState() // will set request to "start" assigned from stateRec[0].Request.
		if err != nil {
			return err
		}
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
	err := s.updateState()
	if err != nil {
		return err
	}
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
	// err := s.updateState()
	// if err != nil {
	// 	return err
	// }
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
	s.objRecId = 0
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

func incrementRequest(r string) bool {
	switch r {
	case "restart", "book", "close", "search", "start", "resize", "scale", "list":
		return false
	default:
		return true
	}
}
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

const (
	// objects to which future requests apply - s.object values
	ingredient_     string = "ingredient"
	task_           string = "task"
	container_      string = "container"
	CtrMsr_         string = "sizeContainer"
	recipe_         string = "recipe" // list recipe in book
	CompleteRecipe_ string = "Complete recipe"
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
	s.setState(lastState)

	if incrementRequest(s.request) {
		s.incrementState(lastState)
	}
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
		fmt.Println("start...")
		switch wx := s.displayData.(type) {
		case Threads:
			fmt.Println("displayData: Threads")
			// redirect request
			s.request = "start-next"

		case *WelcomeT:
			fmt.Println("displayData: *WelcomeT")
			// no previous session - check if userId is registered
			// check for row U-[userId] in ingredients table. This item contains books registered to userId
			//  if no data then user is not registered and therefore ineligble to use app
			if len(s.email) > 0 {
				// resp from startwithEmail. (see below) for the case of no registerd books.
				var w WelcomeT
				w.msg = fmt.Sprintf("Hi, you have no books registered. Please go to www.eburypress.co.uk and register your  book purchase, using the email: %s ", s.email)
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
				w.msg = "You have the following books registered to your email."
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
		}
	}

	if s.request == "list" {
		//
		if len(lastState.RecipeList) > 0 || len(s.state) == 1 {
			fmt.Println("Cannot list from this context - must choose a recipe first")
			s.dmsg = `*** Alert: Cannot list from this context -  choose a recipe first. Say "find recipe" or "restart" or "open book"`
			s.passErr = true
			// displayData set in setState()
			return nil
		}
		if lastState.ShowObjMenu != true {
			err := s.popState() // will set request to "start" assigned from stateRec[0].Request.
			if err != nil {
				return err
			}
		}
		s.displayData = nil
		s.request = "select"
		s.selCtx = 2
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
				s.dmsg = `*** Alert: Resizing the container is not available for this recipe`
				s.passErr = true
			}
		case "instructions", "instruction", "start cooking", "steps", "step":
			s.selId = len(s.menuL)
		default:
			fmt.Println("Invalid action ", s.action)
			s.passErr = true
			s.dmsg = `Invalid list action. Valid actions are "ingredient","container","instruction"`
		}
	}

	if s.request == "resize" { // user enters "size [integer]" only from container size screen
		//
		// user sepecified size of container e.g "size 23"
		if s.object != "sizeContainer" {
			fmt.Println("Cannot change container size from this context - must choose a recipe first")
			s.dmsg = `*** Alert: Cannot change container size from this context -  Say "list resize" and then "resize"`
			s.passErr = true
			// user lastState selCtx & selId to complete processing
		}
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
	//
	// check if scale requested and state is listing ingredients, otherwise ignore
	//
	if s.request == "scale" {
		fmt.Println(" save scale - updateState() ")
		s.reScale = true
		//
		//  define current state
		//
		s.request = lastState.Request
		s.selId = lastState.SelId
		fmt.Println("lastState.Request (now s.request) = ", lastState.Request)
		s.selCtx = lastState.SelCtx
		fmt.Println("lastState.selCtx (now s.selCtx) = ", lastState.SelCtx)
		s.object = lastState.Obj
		//
		// error if state/screen does not permit to change scale
		//
		switch s.displayData.(type) {
		case Threads:
			s.passErr = true
			s.dmsg = ` *** Alert :  you cannot scale a recipe while following instructions. Say "go back" or "restart" and scale from there.`
			return nil
		default:
			if s.object == "sizeContainer" {
				s.dmsg = ` *** Alert :  you should scale a recipe using the size of your container. Say "size [integer]" or "restart" or a "list" option.`
				s.passErr = true
				s.displayData = s.dispCtr
				return nil
			}
			global.SetScale(s.scalef)
			if s.showObjMenu {
				s.displayData = objMenu
				s.updateState()
			}
			//return nil - needs to use selId & selCtx to regen instructions
		}
	}
	//
	// yes/no response. May assign new book
	//
	fmt.Println("yesno: ", s.yesno)
	fmt.Println("selCtx: ", s.selCtx)
	fmt.Println("selId: ", s.selId)
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
		_, err = s.pushState()
		if err != nil {
			return err
		}
		return nil
	}
	if s.request == "book" { // open book
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
				s.passErr = true
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
					s.passErr = true
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
			fmt.Println("search for: ", srch)
			err := s.keywordSearch(srch)
			if err != nil {
				return err
			}
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
		s.recipeList = nil
		// remove superflous stuff
		//s.recipeMap = make(mRecipeM)
		srch := s.reqSearch
		// full search for recipe name
		err := s.keywordSearch(srch)
		if err != nil {
			return err
		}
		if len(s.recipeList) == 0 {
			// remove filler words
			for _, v := range []string{" of ", " with ", " the ", " has ", " and ", " recipe "} {
				srch = strings.Replace(srch, v, " ", -1)
			}
			// remove double spaces
			srch = strings.Replace(srch, "  ", " ", -1)
			search(srch)
		}
		// for k, _ := range s.recipeMap {
		// 	s.recipeList = append(s.recipeList, k)
		// }
		//
		s.eol, s.reset, s.object = 0, true, ""
		s.showObjMenu = false
		fmt.Println("after search: BookId, RecipeId ", s.reqBkId, s.reqRId)
		s.pkey = s.reqBkId + "-" + s.reqRId
		if len(s.recipeList) == 0 && len(s.reqRId) > 0 {
			// found single recipe.
			s.displayData = objMenu
			s.showObjMenu = true
			s.selId = 0
			fmt.Printf("recipe found [%s]\n", s.reqRName)

			_, err = s.pushState()
			if err != nil {
				return err
			}
		} else if len(s.recipeList) > 0 {
			// found multiple recipes
			s.curReqType = 0
			s.displayData = s.recipeList
			fmt.Printf("recipe List found [%#v]\n", s.recipeList)

			_, err = s.pushState()
			if err != nil {
				return err
			}
		} else {
			// no recipe found
			//	s.displayData = s.recipeList
			s.passErr = true
			if len(s.reqOpenBk) > 0 {
				s.dmsg = `*** No recipe found in ` + s.reqBkName + ` matching "` + s.reqSearch + `". `
			} else {
				s.dmsg = `*** No recipe found in any of your books matching "` + s.reqSearch + `". `
			}
			words := strings.Split(s.reqSearch, " ")
			if len(words) == 1 {
				s.dmsg += "Try multiple keywords e.g search orange chocolate tart. "
			} else {
				s.dmsg += "Change the order of the keywords or try alternative keywords. "
			}
			s.dmsg += "Otherwise continue."
		}
		return nil
	}
	//
	if s.request == "resume" {
		if s.cThread == 0 || s.object != "task" || s.reqRId == "0" {
			s.err = fmt.Errorf("There is nothing to resume")
			return nil
		}
		if t, ok := s.displayData.(Threads); ok {
			if s.oThread == -1 {
				s.err = fmt.Errorf("All is completed. There is nothing to resume")
				return nil
			}
			if t[s.oThread].Id == len(t[s.oThread].Instructions) {
				s.err = fmt.Errorf("All is completed. There is nothing to resume")
				return nil
			}
			x := s.cThread
			s.cThread = s.oThread
			s.oThread = x
			s.recId[objectMap[s.object]] = t[s.cThread].Id
		} else {
			s.err = fmt.Errorf("There is nothing to resume")
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
		s.objRecId = s.gotoRecId
		s.recId[objectMap[s.object]] = s.gotoRecId
		return nil
	case "repeat":
		// return - no need to pushState as nothing has changed.  Select current recId.
		s.objRecId = s.recId[objectMap[s.object]]
		s.dmsg, s.vmsg, s.ddata = lastState.Dmsg, lastState.Vmsg, lastState.DData
		return nil
	case "prev":
		if len(s.object) == 0 {
			return fmt.Errorf("Error: no object defined for previous in orchestrateRequest")
		}
		s.objRecId = s.recId[objectMap[s.object]] - 1
		s.recId[objectMap[s.object]] -= 1
		return nil
	case "next", "select-next", "start-next":
		if len(s.recId) == 0 {
			s.recId = [4]int{}
		}
		//
		if len(s.object) == 0 {
			return fmt.Errorf("Error: no object defined for next in orchestrateRequest")
		}
		// recId contains last processed instruction. So must add one to get current instruction.
		s.objRecId = s.recId[objectMap[s.object]] + 1
		s.recId[objectMap[s.object]] += 1
		return nil
	}
	//
	// respond to select from displayed items
	//
	if (s.request == "select" || s.request == "start") && s.selId > 0 {
		// selId is the response from Alexa on the index (ordinal value) of the selected display item
		fmt.Println("SELCTX is : ", s.selCtx)

		switch s.selCtx {
		case ctxRecipeMenu:
			// select from: multiple recipes
			if !s.reScale {
				// select id only checked for genuine selection which doesn't happen if request is scale.
				if s.selId > len(s.recipeList) || s.selId < 1 {
					s.passErr = true
					s.dmsg = "selection is not within range"
					return nil
				}

				p := s.recipeList[s.selId-1]
				s.reqRId, s.reqRName, s.reqBkId, s.reqBkName = p.RId, p.RName, p.BkId, p.BkName
				s.dmsg = fmt.Sprintf(`Now that you have selected [%s] recipe would you like to list ingredients, cooking instructions, utensils or containers or cancel`, s.reqRName)
				s.vmsg = fmt.Sprintf(`Now that you have selected {%s] recipe would you like to list ingredients, cooking instructions, utensils or containers or cancel`, s.reqRName)
				// chosen recipe, so set select context to object (ingredient, utensil, container, tas			s.selCtx = ctxObjectMenu
				//
				_, err := s.recipeRSearch()
				if err != nil {
					return err
				}
			}
			s.displayData = objMenu
			s.selId = 0
			s.showObjMenu = true
			s.recipeList = nil

			s.reset = true
			fmt.Println("ctxRecipeMenu: about to pushState")
			if !s.reScale {
				_, err = s.pushState()
				if err != nil {
					return err
				}
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
					s.dmsg = "selection is not within range"
					s.passErr = true
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
				if len(s.parts) > 0 && len(s.part) == 0 {
					//s.dispPartMenu = true
					s.displayData = s.parts
					s.curReqType = 0
					//
					if s.request == "select" {
						if s.reScale {
							s.updateState()
						} else {
							_, err = s.pushState()
							if err != nil {
								return err
							}
						}
					}
					return nil
				} else {
					// go straight to instructions
					s.displayData, err = s.cacheInstructions()
					if err != nil {
						return err
					}
					//
					if s.request == "select" {
						_, err = s.pushState()
						if err != nil {
							return err
						}
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
					r, err := s.recipeRSearch()
					if err != nil {
						return err
					}
					// set unit formating mode
					global.Set_WriteCtx(global.UPrint)
					as, err := s.loadActivities()
					if err != nil {
						return err
					}
					// generate ingredient listing

					s.ingrdList = IngredientT(as.String(r))
					s.displayData = s.ingrdList

					if s.reScale { // change in scale from last session
						fmt.Println("in ingredient List: update state not pushState")
						s.updateState()
					} else {
						fmt.Println("in ingredient List: pushState")
						_, err = s.pushState()
					}
					if err != nil {
						return err
					}

				case "start", "list":
					s.ingrdList = IngredientT(s.ingrdList)
				}
				//
				s.reset = true
				s.showObjMenu = false
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
				if s.request == "select" {
					_, err = s.pushState()
					if err != nil {
						return err
					}
				}
				return nil

			case CtrMsr_:
				// displayData assigned in setState()
				fmt.Printf("CtrMsr - %#v\n", *(s.dispCtr))
				s.displayData = s.dispCtr
				return nil
			}

		// recipe part menu
		case ctxPartMenu:
			s.dispCtr = nil
			if s.selId > len(s.parts)+1 || s.selId < 1 {
				//s.setDisplay(lastState)
				s.passErr = true
				s.dmsg = "selection is not within range"
				return nil
			}
			s.part = ""
			if s.selId == 1 {
				s.part = CompleteRecipe_
			} else {
				s.part = s.parts[s.selId-2].Title
			}
			fmt.Printf("selId  %d   parts   %#v\n", s.selId, s.part)
			s.reset = true
			s.showObjMenu = false
			s.displayData, err = s.cacheInstructions(s.selId)
			if err != nil {
				return err
			}
			if s.request == "select" {
				_, err = s.pushState()
				if err != nil {
					return err
				}
			}
			// now complete the request by morphing request to a next operation
			s.request = "select-next"
			s.curReqType = instructionRequest
			s.object = "task"
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
		userId:      request.QueryStringParameters["uid"], // empty when not called
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
		err = sessctx.popState()
		if err != nil {
			fmt.Println("Error returned by popState..")
		}
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
	// package the response data RespEvent (an APL aware "display" structure) and return
	//
	if sessctx.displayData == nil {
		sessctx.dmsg = "displayData not set"
		return RespEvent{Text: sessctx.vmsg, Verbal: sessctx.dmsg + sessctx.ddata}, nil
	}
	var resp RespEvent
	fmt.Println("=========== displayData.GenDisplay =============")
	resp = sessctx.displayData.GenDisplay(sessctx)
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

	// if len(os.Args) < 5 {
	// 	scaleF = 1.0
	// } else {
	// 	scaleF, err = strconv.ParseFloat(os.Args[4], 64)
	// 	if err != nil {
	// 		panic(err)
	// 	}
	// }
	// 	global.Set_WriteCtx(global.UDisplay)
	// 	p, _ := handler(p2)
	// 	if len(p.Error) > 0 {
	// 		fmt.Printf("%#v\n", p.Error)
	// 	} else {
	// 		fmt.Printf("output:   %s\n", p.Text)
	// 		fmt.Printf("output:   %s\n", p.Verbal)
	// 	}
}
