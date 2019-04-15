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

type sessCtx struct {
	err        error
	newSession bool
	//path      string // InputEvent.Path
	request string // pathItem[0]: request from user e.g. select, next, prev,..
	//param     string // InputEvent.Param
	state     stateStack
	lastState *stateRec // state attribute from state dynamo item - contains state history
	passErr   string
	//
	userId      string   // sourced from request. Used as PKey to Sessions table
	bkids       []string // registered books to user
	reqOpenBk   BookT    // BkId|BkName|authors
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
	//
	menuL     menuList
	dispCtr   *DispContainerT
	dimension int
	scalef    float64
	reScale   bool
	//
	//display *apldisplayT
	welcome *WelcomeT
	//
	email string
}

const scaleThreshold float64 = 0.9

func (s *sessCtx) closeBook() {
	fmt.Println("closeBook()")
	s.reqOpenBk = ""
	s.CloseBkName = s.reqBkName
	s.reqBkId, s.reqBkName, s.reqRId, s.reqRName = "", "", "", ""
	s.reqOpenBk, s.authorS, s.authors = "", nil, ""
	s.eol, s.peol = 0, 0
	s.displayData = s.reqOpenBk
	s.newSession = true
	s.cThread, s.oThread = 0, 0
}

func (s *sessCtx) openBook() {

	s.reqOpenBk = BookT(s.reqBkId + "|" + s.reqBkName + "|" + s.authors)
	s.eol, s.peol = 0, 0
	s.reqRName, s.reqRId = "", ""
	s.displayData = s.reqOpenBk
	s.newSession = true
	fmt.Println("openBook() - ", s.reqOpenBk)
	s.cThread, s.oThread = 0, 0
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

const (
	// objects to which future requests apply - s.object values
	ingredient_     string = "ingredient"
	task_           string = "task"
	container_      string = "container"
	CtrMsr_         string = "scaleContainer"
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
	lastState, err := s.getState()
	if err != nil {
		return err
	}
	// set current state based on last session
	s.setState(lastState)
	//
	// ******************************** process responses to request  ****************************************
	//
	// alexa launch request
	//
	if s.request == "start" {
		fmt.Println("start...")
		switch wx := s.displayData.(type) {
		case Threads:
			fmt.Println("Threads..")
			// redirect request
			s.request = "start-next"
			// s.objRecId = s.recId[objectMap[s.object]] + 1
			// s.recId[objectMap[s.object]] += 1
		case *WelcomeT:
			fmt.Println("Welcome..")
			// no previous session - check if userId is registered
			// check for row U-[userId] in ingredients table. This item contains books registered to userId
			//  if no data then user is not registered and therefore ineligble to uer app
			if len(s.email) > 0 {
				// resp from startwithEmail. (see below) for the case of no registerd books.
				var w WelcomeT
				w.msg = fmt.Sprintf("Hi, you have no books registered against this device. Please go to www.eburypress.co.uk and register your  book purchase, using the email: %s ", s.email)
				s.displayData = &w
				return nil
			}
			// always check for books = don't rely on cached result even if it reasonably current
			s.bkids, err = s.getUserBooks()
			if err != nil {
				return err
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
		}
	}
	if s.request == "dimension" {
		if s.dimension == 0 {
			return fmt.Errorf("Dimension must be greater than zero")
		}
		if s.dispCtr == nil {
			return fmt.Errorf("Dimension cannot be set until you have entered adjust menu option")
		}
		c := s.dispCtr
		cdim, err := strconv.Atoi(c.Dimension)
		fmt.Println("Dimension: cdim = ", cdim)
		fmt.Println("user entered dimension = ", s.dimension)
		if err != nil {
			panic(err.Error())
		}
		if float64(s.dimension)/float64(cdim) < 0.6 {
			return fmt.Errorf("Dimension cannot be less than 60% of recommended container size")
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
		// only save when in the appropriate state
		//
		// if s.request == "search" {
		// 	s.displayData = s.recipeList
		// 	fmt.Println("scaling will Search for ", s.reqSearch)

		// 	return nil
		// 	// s.reqSearch = lastState.Search
		// 	// s.curReqType = initialiseRequest
		// }
		if s.showObjMenu {
			s.displayData = objMenu
			global.SetScale(s.scalef)
			s.updateState()
			return nil
		}
		if s.selCtx != ctxPartMenu {
			// update scale if not in instruction screen
			global.SetScale(s.scalef)
		} else {
			global.SetScale(s.scalef)
		}
	}
	//
	// yes/no response. May assign new book
	//
	fmt.Println("yesno: ", s.yesno)
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
				s.openBook()
			} else {
				s.popState()
			}
		case 22:
			// swap to this book
			if s.selId == 1 {
				s.reqBkId, s.reqBkName, s.reset = lastState.SwpBkId, lastState.SwpBkNm, true
				s.vmsg = fmt.Sprintf(`Book [%s] is now open. You can now search or open a recipe within this book`, lastState.BkName)
				s.dmsg = fmt.Sprintf(`Book [%s] is now open. You can now search or open a recipe within this book`, lastState.BkName)
				s.openBook()
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
	//
	// respond to select from displayed items
	//
	if s.selId > 0 {
		// selId is the response from Alexa on the index (ordinal value) of the selected display item
		fmt.Println("SELCTX is : ", s.selCtx)

		switch s.selCtx {
		case ctxRecipeMenu:
			// select from: multiple recipes
			if !s.reScale {
				// select id only checked for genuine selection which doesn't happen if request is scale.
				if s.selId > len(s.recipeList) || s.selId < 1 {
					s.passErr = "selection is not within range"
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
			fmt.Println("selId: ", s.selId)
			// if s.reScale {
			// 	// current operation is a rescale - which only applies to ingredients so set Ingredient
			// 	s.object = ingredient_
			// } else {
			if !s.reScale {
				// check select id is within menu range. For reScale there is no menu listing in its current state.
				if s.selId > len(s.menuL) {
					//s.setDisplay(lastState)
					s.passErr = "selection is not within range"
					return nil
				}
				s.object = objectS[s.menuL[s.selId-1]]
			}
			fmt.Println("SElected: ", s.object)
			// menuL has done its job. Now zero it.
			s.menuL = nil
			// clear recipeList which is not necessary in this state. Held in previous state.
			s.recipeList = nil
			// object chosen, nothing more to select for the time being
			//s.selCtx = 0
			switch s.object {

			//  "lets start cooking" selected
			case task_:
				s.dispCtr = nil
				//s.selClear = true //TODO: what if they back at this point we have cleared sel.
				fmt.Printf("s.parts  %#v [%s] \n", s.parts, s.part)
				s.showObjMenu = false
				if len(s.parts) > 0 && len(s.part) == 0 {
					//s.dispPartMenu = true
					s.displayData = s.parts
					s.curReqType = 0
					//
					if s.reScale {
						s.updateState()
					} else {
						_, err = s.pushState()
						if err != nil {
							return err
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
				fmt.Println("Here in ingredient_.. about to read activity table and generated ingredient string")
				s.dispCtr = nil
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
				//
				s.reset = true
				s.showObjMenu = false
				s.curReqType = 0

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
				//	}
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
				fmt.Println("Here in container.. about to pushState")
				_, err = s.pushState()
				if err != nil {
					return err
				}
				return nil

			case CtrMsr_:
				s.displayData = s.dispCtr

				// s.reset = true
				// s.showObjMenu = false
				// s.curReqType = 0

				// _, err = s.pushState()
				// if err != nil {
				// 	return err
				// }
				return nil
			}

		// recipe part menu
		case ctxPartMenu:
			if s.selId > len(s.parts)+1 || s.selId < 1 {
				//s.setDisplay(lastState)
				s.passErr = "selection is not within range"
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
		if s.request == "search" {

			s.clearForSearch(lastState)

			fmt.Println("Search.....")
			fmt.Println("Before BookId, RecipeId ", s.reqBkId, s.reqRId)
			fmt.Printf("..about to call keywordSearch with [%s]\n", s.reqSearch)

			err := s.keywordSearch()
			if err != nil {
				return err
			}
			s.eol, s.reset, s.object = 0, true, ""
			s.showObjMenu = false
			fmt.Println("after search: BookId, RecipeId ", s.reqBkId, s.reqRId)
			s.pkey = s.reqBkId + "-" + s.reqRId
			if len(s.recipeList) == 0 && len(s.reqRId) > 0 {
				// found single recipe.
				s.displayData = objMenu
				s.showObjMenu = true
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
				s.displayData = s.recipeList
				fmt.Printf("No recipe found for search keywords \n", s.reqSearch)
			}

			return nil
		}
		//TODO: is showList required?
		if s.showList {
			if len(s.recipeList) > 0 {
				for i, v := range s.recipeList {
					s.dmsg = s.dmsg + fmt.Sprintf("%d. Recipe [%s] in book [%s] by [%s] quantity %s\n", i+1, v.RName, v.BkName, v.Authors, v.Quantity)
					s.vmsg = s.dmsg + fmt.Sprintf("%d. Recipe [%s] in book [%s] by [%s] quantity %s\n", i+1, v.RName, v.BkName, v.Authors, v.Quantity)
				}
			}

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
					s.dmsg = `There is no books open to close.`
					s.vmsg = `There is no books open to close.`
				default:
					//
					s.dmsg = s.reqBkName + ` is now closed. Any searches will be across all books`
					s.vmsg = s.reqBkName + ` is now closed. Any searches will be across all books`
					s.closeBook()
				}
			}
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
			s.displayData = s.reqOpenBk
			if len(lastState.BkId) > 0 {
				// some book was open previously
				switch {
				case lastState.BkId == s.reqBkId:
					// open currenly opened book. reqBkId was sourced using bookNameLookup()
					if lastState.activeRecipe() && len(lastState.OpenBk) == 0 {
						s.dmsg = "You are actively browsing a recipe from this book. Do you still want to open " + s.reqBkName + "?"
						s.vmsg = "You are actively browsing a recipe from this book. Do you still want to open " + s.reqBkName + "?"
						s.questionId = 21
					}
					if len(lastState.OpenBk) > 0 {
						s.dmsg = s.reqBkName + " is already open"
						s.vmsg = s.reqBkName + " is already open"
					}
				case lastState.BkId != s.reqBkId:
					if len(lastState.RName) == 0 {
						// no active recipe
						s.dmsg = "Opened " + s.reqBkName + " by " + s.authors + ". "
						s.vmsg = "Opened " + s.reqBkName + " by " + s.authors + ". "
						s.openBook()

					} else if lastState.activeRecipe() {
						s.dmsg = "You are actively browsing a recipe from book, " + lastState.BkName + ". Do you still want to open " + s.reqBkName + "?"
						s.vmsg = "You are actively browsing a recipe from book, " + lastState.BkName + ". Do you still want to open " + s.reqBkName + "?"
						s.questionId = 21
					}
				}
			} else {
				s.openBook()
				s.dmsg = "Opened " + s.reqBkName + " by " + s.authors + ". "
				s.vmsg = "Opened " + s.reqBkName + " by " + s.authors + ". "
			}

			s.ingrdList, s.recipeList, s.object, s.showObjMenu = "", nil, "", false
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

		return nil
	}
	//  if listing (next,prev,repeat,goto - curReqType object, listing) without object (container,ingredient,task,utensil) -
	if s.curReqType == instructionRequest && len(s.object) == 0 {
		s.dmsg = `You need to say what you want to list. Please say either "ingredients","start cooking","containers" or "utensils". Not hard really..`
		s.vmsg = `You need to say what you want to list. Please say either "ingredients","start cooking","containers" or "utensils". Not hard really..`

		return nil
	}
	//  if listing and not finished and object request changes object. Accept and zero or repeat last RecId for requested object.
	// if len(s.object) > 0 {
	// 	//if !s.finishedListing(s.recId[objectMap[s.object]], objectMap[s.object]) && (s.object != s.object) {
	// 	fmt.Println(s.object)
	// 	if len(s.recId) > 0 {
	// 		if s.eol != s.recId[objectMap[s.object]] && (s.object != s.object) {
	// 			// show last listed entry otherwise list first entry
	// 			switch s.recId[objectMap[s.object]] {
	// 			case 0: // not listed before or been reset after previously completing list
	// 				s.objRecId = 1 // show first entry
	// 			default: // in the process of listing
	// 				s.objRecId = s.recId[objectMap[s.object]] // repeat last shown entry
	// 			}
	// 		}
	// 	}
	// }
	// // if object specified and different from last one
	// if len(s.object) > 0 && len(s.recId) > 0 && (s.object != s.object) {
	// 	// show last listed entry otherwise list first entry
	// 	switch s.recId[objectMap[s.object]] {
	// 	case 0: // not listed before or been reset after previously completing list
	// 		s.objRecId = 1 // show first entry
	// 	default: // in the process of listing
	// 		s.objRecId = s.recId[objectMap[s.object]] // repeat last shown entry
	// 	}
	// }
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
		s.objRecId = s.gotoRecId
		s.recId[objectMap[s.object]] = s.gotoRecId
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
	}
	//
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

			fmt.Println("Aboutto loadBaeRecipe()")
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
	case "genSlotValues":
		err = sessctx.generateSlotEntries()
		if err != nil {
			break
		}
		//sessctx.noGetRecRequired, sessctx.reset = true, true
		sessctx.reset = true
	case "start":
		sessctx.curReqType = 0 //TODO: what to put here...if anything
	case "startWithEmail":
		sessctx.email = request.QueryStringParameters["email"]
		sessctx.request = "start"
		sessctx.curReqType = 0 //TODO: what to put here...if anything
	case "book", "recipe", "select", "search", "list", "yesno", "version", "back", "resume", "dimension", "scale":
		sessctx.curReqType = initialiseRequest
		switch sessctx.request {
		case "book": // user reponse "open book" "close book"
			// book id and name  populated in this section
			if len(pathItem) > 1 && pathItem[1] == "close" {
				sessctx.request = "book/close"
			} else { // open
				sessctx.reqBkId = request.QueryStringParameters["bkid"]
				err = sessctx.bookNameLookup()
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

		case "dimension":
			var i int
			i, err = strconv.Atoi(request.QueryStringParameters["dim"])
			if err != nil {
				err = fmt.Errorf("%s: %s", "Error in converting int of dimension request \n\n", err.Error())
			} else {
				sessctx.dimension = i
			}

		case "back":
			// used back button on display device. Note: back will ignore orachestrateRequest and go straight to displayGen()
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
	// package the response data RespEvent (an APL aware "display" structure) and return
	//
	var resp RespEvent
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
	// var err error
	// p1 := InputEvent{Path: os.Args[1], Param: "sid=asdf-asdf-asdf-asdf-asdf-987654&bkid=" + os.Args[2] + "&rid=" + os.Args[3]}
	// uid := `amzn1.ask.account.AFTQJDFZKJIDFN6GRQFTSILWMGO2BHFRTP55PK6KT42XY22GR4BABOP4Y663SUNVBWYABLLQCHEK22MZVUVR7HXVRO247IQZ5KSVNLMDBRDRYEINWGRB6N2U7J2BBWEOEKLY2HKQ6VQTTLGKT2JCH4VOE5A7XPFDI4VMNJW63YP4XCMYGIA5IU4VJGNHI2AAU33Q5J2TJIXP3DI`
	// p2 := InputEvent{Path: "addUser", Param: "uid=" + uid + "&bkids=20,21"}

	// if len(os.Args) < 5 {
	//scaleF = 1.0
	// } else {
	// 	scaleF, err = strconv.ParseFloat(os.Args[4], 64)
	// 	if err != nil {
	// 		panic(err)
	// 	}
	// }
	// global.Set_WriteCtx(global.UDisplay)
	// p, _ := handler(p2)
	// if len(p.Error) > 0 {
	// 	fmt.Printf("%#v\n", p.Error)
	// } else {
	// 	fmt.Printf("output:   %s\n", p.Text)
	// 	fmt.Printf("output:   %s\n", p.Verbal)
	// }
}
