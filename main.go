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
	//path      string // InputEvent.Path
	request string // pathItem[0]: request from user e.g. select, next, prev,..
	//param     string // InputEvent.Param
	state     stateStack
	lastState *stateRec // state attribute from state dynamo item - contains state history
	//
	sessionId  string // sourced from request. Used as PKey to Sessions table
	reqRName   string // requested recipe name - query param of recipe request
	reqBkName  string // requested book name - query param
	reqRId     string // Recipe Id - 0 means no recipe id has been assigned.  All RId's start at 1.
	reqBkId    string
	reqSearch  string // keyword search value
	reqVersion string // version id, starts at 0 which is blank??
	//reqSearch   string   // search value
	recId       []int    // record id for each object (ingredient, container, utensils, containers). No display will use verbal for all object listings.
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
	closeBook   bool
	object      string //container,ingredient,instruction,utensil. Sourced from Sessions table or request
	//updateAdd      int        // dynamodb Update ADD. Operation dependent
	gotoRecId        int        // sourced from request
	objRecId         int        // current record id for object. Object is a ingredient,task,container,utensil.- displayed record id persisted to session after use.
	recIdNotExists   bool       // determines whether to create []RecId set attribute in Session  table
	noGetRecRequired bool       // a mutliple record request e.g. ingredients listing
	eol              int        // sourced from Sessions table
	mChoice          []mRecipeT // multi-choice select. Recipe name and ingredient searches can result in mutliple records being returned. Results are saved.
	//
	selCtx   selectCtxT // select context either recipe or other (i.e. object)
	selId    int        // value selected by user of index in itemList
	selClear bool
	//
	showList  bool   // show what ever is in the current list (books, recipes)
	ingrdList string // output of activity.String() - ingredient listing
	// vPreMsg        string
	// dPreMsg        string
	displayHdr    string // passed to alexa display
	displaySubHdr string // passed to alexa display
	activityS     Activities
	dmsg          string
	vmsg          string
	ddata         string
	yesno         string
	// Recipe Part data
	peol  int      // End-of-List-for-part
	part  string   // part index name - if no value then no part is being used eventhough recipe may be have a part defined i.e nopart_ & a part
	parts []*PartT // sourced from Recipe (R-)
	next  int      // next SortK (recId)
	prev  int      // previous SortK (recId) when in part mode as opposed to full recipe mode
	pid   int      // record id within a part 1..peol
	//
	back bool // back button pressed on display
}

const (
	// objects to which future requests apply - s.object values
	ingredient_ string = "ingredient"
	task_       string = "task"
	container_  string = "container"
	utensil_    string = "utensil"
	recipe_     string = "recipe" // list recipe in book
)

type selectCtxT int

const (
	ctxRecipe     selectCtxT = 1
	ctxObjectList            = 2
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
	xx = iota
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
	// merge ssession context with last session where appropiate
	//
	lastState, err := s.getState()
	if err != nil {
		return err
	}
	//
	// determine select context
	//
	if s.selId > 0 {
		if lastState.Request == "search" {
			fmt.Println("last Request search, last SelCtx ", lastState.SelCtx)
			s.selCtx = lastState.SelCtx
		} else if lastState.Request == "select" {
			fmt.Println("lastState.Request == select,last SelCtx ", lastState.SelCtx)
			s.selCtx = lastState.SelCtx + 1
		}
		if s.selCtx > ctxObjectList {
			s.ingrdList = lastState.Ingredients
			return nil
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
		if len(lastState.Ver) == 0 {
			s.reset = true
		} else if len(lastState.Ver) > 0 && s.reqVersion != lastState.Ver {
			s.reset = true
		}
	}
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
	fmt.Println("in validateRequest .  Select: ", s.selId, lastState.SelCtx)
	if s.selId > 0 {
		// selId is the response from Alexa on the index (ordinal value) of the selected display item
		switch s.selCtx {
		case ctxRecipe:
			// select from: multiple recipes
			if s.back {
				return nil
			}
			if s.selId > len(s.mChoice) || s.selId < 1 {
				return fmt.Errorf("Selection out of range")
			}
			p := s.mChoice[s.selId-1]
			s.reqRId, s.reqRName, s.reqBkId, s.reqBkName = p.RId, p.RName, p.BkId, p.BkName
			s.dmsg = fmt.Sprintf(`Now that you have selected [%s] recipe would you like to list ingredients, cooking instructions, utensils or containers or cancel`, s.reqRName)
			s.vmsg = fmt.Sprintf(`Now that you have selected {%s] recipe would you like to list ingredients, cooking instructions, utensils or containers or cancel`, s.reqRName)
			// chosen recipe, so set select context to object (ingredient, utensil, container, tas			s.selCtx = ctxObjectList
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

		case ctxObjectList:
			//	select from: list ingredient, list utensils, list containers, start cooking
			fmt.Println("selId: ", s.selId)
			s.object = objectS[s.selId-1]
			fmt.Println("object: ", s.object)
			// object chosen, nothing more to select for the time being
			//s.selCtx = 0
			switch s.object {
			case task_:
				//  "lets start cooking" selected
				s.request = "next"
				s.curReqType = instructionRequest
				s.selClear = true
			case ingredient_:
				//  "list ingredients" selected
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
				//
				_, err = s.pushState()
				if err != nil {
					return err
				}
				return nil
			case container_:
				return nil
				// case utensil_:
			}
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
			if s.back {
				return nil // have all the results in lastSess
			}
			// search only applies to recipes, ie. select context Recipe (ctxRecipe).
			s.selCtx = ctxRecipe
			// we have fully populated session context from previous session e.g. BkName etc, now lets see what recipes we find
			// populates sessCtx.mChoice if search results in a list.
			fmt.Println("In validateRequest. About to call keywordSearch")
			err := s.keywordSearch()
			if err != nil {
				panic(err)
			}
			s.eol, s.reset, s.object = 0, true, ""
			if len(s.mChoice) == 0 || len(s.reqRId) > 0 {
				// single recipe found in search. Select context must now reflect object list. Persist value.
				s.selCtx = ctxObjectList
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
		if s.request == "book" {
			//
			// book requested
			//
			switch len(lastState.BkId) {
			case 0:
				if s.closeBook {
					s.dmsg = `Book is already closed.`
					s.vmsg = `Book is already closed.`
					s.reqBkId, s.reqBkName, s.reqRId, s.reqRName = "", "", "", ""
				} else {
					// no books currently open. Open this one.
					s.vmsg = "Found " + s.reqBkName + " by " + s.authors + ". "
					s.vmsg += `You can ask for a recipe in this book by saying "open at" recipe name or search for recipes by saying "search for " ingredient and category for example "search for chocolate cake"`
					s.dmsg = "Found " + s.reqBkName + " by " + s.authors + ". "
					s.dmsg += `You can ask for a recipe in this book by saying "open at" recipe name or search for recipes by saying "search for " ingredient and category  for example "search for chocolate cake"`
				}
				//
				s.eol = 0
				_, err := s.pushState()
				if err != nil {
					return err
				}
				return nil
			default:
				// there is already an initialised book for this session
				if s.closeBook || s.reqBkName != lastState.BkName {
					// closing or open a book different from current opened book
					switch len(lastState.RName) {
					case 0:
						// no active recipe, then open or close book
						if s.closeBook {
							s.dmsg = fmt.Sprintf("Book %s is closed", lastState.BkName)
							s.vmsg = fmt.Sprintf("Book %s is closed", lastState.BkName)
							s.reqBkId, s.reqBkName, s.reqRId, s.reqRName, s.reset = "", "", "", "", true //TODO - should recipe be closed
						} else {
							// no active recipe. Open book.
							s.vmsg = `Please state what recipe you would like from this book or I can list them if you like. Say "list" or recipe name.`
							s.dmsg = `Please state what recipe you would like from this book or I can list them if you like. Say "list" or recipe name.`
						}
						// Book initialises. No recipe provided. Persist.
						s.eol, s.reset = 0, true
						_, err := s.pushState()
						if err != nil {
							return err
						}
						return nil
					default:
						// open different book with an active recipe.
						//s.reqRName = lastState.RName
						if len(s.object) > 0 && s.eol != s.recId[objectMap[s.object]] {
							if s.closeBook {
								s.dmsg = fmt.Sprintf("You currently have recipe %s open. Do you still want to close the book?", lastState.RName)
								s.vmsg = fmt.Sprintf("You currently have recipe %s open. Do you still want to close the book?", lastState.RName)
								s.questionId = 20
							} else {
								// currently listing
								s.dmsg = `You have specified a different book while having an active recipe from which you are currently listing. Do you want to swap to this book, "Yes" or "No" or "Cancel" for no?`
								s.vmsg = `You have specified a different book while having an active recipe from which you are currently listing. Do you want to swap to this book, "Yes" or "No" or "Cancel" for no?`
								s.questionId = 21
								s.swapBkName = s.reqBkName
								s.swapBkId = s.reqBkId
							}
							// save book details to swap attributes in Session table
							//s.eol, s.reset = 0, true - depends on yes/no answer
							_, err := s.pushState()
							if err != nil {
								return err
							}
							return nil
						} else {
							// finished listing or no object selected, swap to this book
							if s.closeBook {
								s.reqBkId, s.reqBkName = "", ""
								s.dmsg = `Book closed. `
								s.vmsg = `Book closed. `
							} else {
								s.vmsg = `What recipe from this book might you be interested in. Please say a recipe name or I can list them to the display if you like if you "list".`
								s.dmsg = `What recipe from this book might you be interested in. Please say a recipe name or I can list them to the display if you like if you "list".`
							}
							// new book selected, zero recipe etc
							s.eol, s.reset, s.reqRId, s.reqRName = 0, true, "", ""
							_, err := s.pushState()
							if err != nil {
								return err
							}
							return nil
						}
					}
				} else {
					// open book. same initialised book requested
					switch len(lastState.RName) {
					case 0:
						// no active recipe
						s.dmsg = `Book is currenlty open. Please request a recipe from the book or say "list" and I will print the recipe names to the display.`
						s.noGetRecRequired = true
						return nil
					default:
						s.dmsg = `Book is already open at recipe ` + lastState.RName
						s.noGetRecRequired = true
						return nil
					}
				}
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
		if len(s.part) == 0 {
			if len(s.object) == 0 {
				return fmt.Errorf("Error: no object defined when in validateRequest - next")
			}
			if lastState.EOL > 0 && len(s.recId) > 0 {
				if s.recId[objectMap[s.object]] == 1 {
					s.objRecId = s.recId[objectMap[s.object]]
					// s.dPreMsg = "You have reached the end. "
					// s.vPreMsg = "You have reached the end. "
					return nil
				}
				if len(s.object) == 0 {
					return fmt.Errorf("Error: no object defined when in validateRequest - next")
				}
				s.recId[objectMap[s.object]] -= 1
				s.objRecId = s.recId[objectMap[s.object]]
			}
		} else {
			s.objRecId = lastState.Prev
			if s.objRecId == -1 {
				s.noGetRecRequired = true
				return nil
			}
		}
	case "next":
		if len(s.part) == 0 {
			// no part mode ie. user has not elected to follow a part if one exists, so follows normal non-part mode.
			if len(s.object) == 0 {
				return fmt.Errorf("Error: no object defined when in validateRequest - next")
			}
			if lastState.EOL > 0 && len(s.recId) > 0 {
				if s.recId[objectMap[s.object]] == s.eol {
					s.objRecId = lastState.EOL
					// s.dPreMsg = "You have reached the end. "
					// s.vPreMsg = "You have reached the end. "
					s.noGetRecRequired = true
					return nil
				}
			}
			s.recId[objectMap[s.object]] += 1
			s.objRecId = s.recId[objectMap[s.object]]
		} else {
			s.objRecId = lastState.Next
			if s.objRecId == -1 {
				// TODO: finished current part. Options go onto next or has recipe been completed - how to determine?
				// CptPart[k]=true
				// save ComPlTedPart to session to keep track completed Part for user.
				s.noGetRecRequired = true
				return nil
			}
		}
	}
	// check if we have Dynamodb Recid Set defined, this will be useful in pushState
	if len(s.recId) == 0 {
		s.recIdNotExists = true
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
	//	s.dmsg = expandLiteralTags(rec.Display)
	if s.objRecId == rec.EOL {
		writeCtx = uSay
		s.vmsg = "and finally, " + expandScalableTags(expandLiteralTags(rec.Verbal))
		writeCtx = uDisplay
		s.dmsg = "and finally, " + expandScalableTags(expandLiteralTags(rec.Display))
	} else {
		writeCtx = uSay
		s.vmsg = expandScalableTags(expandLiteralTags(rec.Verbal))
		writeCtx = uDisplay
		s.dmsg = expandScalableTags(expandLiteralTags(rec.Display))
	}
	if len(s.part) > 0 {
		s.dmsg = "[" + strconv.Itoa(s.pid) + "|" + strconv.Itoa(s.peol) + "|" + strconv.Itoa(s.eol) + "]  " + s.dmsg
	} else {
		s.dmsg = "[" + strconv.Itoa(s.pid) + "|" + strconv.Itoa(s.eol) + "]  " + s.dmsg
	}
	//
	// save state to dynamo
	//
	err = s.updateState()
	if err != nil {
		return err
	}
	return nil
	//
	// if EOL has changed because of object change then update session context with new EOL
	//	use EOL on next session to determine if RecId is at EOL and print end-of-list message
	// fmt.Println("getRecById rec.EOL, s.eol", rec.EOL, s.eol)
	// also save next, prev to session table if using part mode
	// if s.eol != rec.EOL {
	// 	s.eol = rec.EOL
	// 	s.pushStateEOL()
	// }
	// if len(s.curPart) > 0 {
	// 	// update next & prev values

	// }

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
	List     []DisplayItem `json:"List"` // id|Title1|subTitle1|SubTitle2|Text
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
				fmt.Println("** closeBook.")
				sessctx.closeBook = true
			} else {
				sessctx.reqBkId = request.QueryStringParameters["bkid"]
				err = sessctx.bookNameLookup()
			}
		// case "yes", "no":
		// 	sessctx.yesno = pathItem[0]
		case "version":
			sessctx.reqVersion = request.QueryStringParameters["ver"]
		case "list":
			sessctx.showList = true
		case "search":
			sq, err := url.QueryUnescape(request.QueryStringParameters["srch"])
			if err != nil {
				panic(err)
			}
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
			//
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
	if sessctx.curReqType == initialiseRequest || sessctx.noGetRecRequired {

		switch {
		case sessctx.request == "select" && sessctx.object == ingredient_:

			var ingrdlst []DisplayItem
			for _, v := range strings.Split(sessctx.ingrdList, "\n") {
				item := DisplayItem{Title: v}
				ingrdlst = append(ingrdlst, item)
			}
			s := sessctx
			return RespEvent{Type: "Ingredient", Header: s.reqRName, SubHdr: "Ingredients", List: ingrdlst}, nil

		case sessctx.request == "select" && sessctx.object == container_:

			var mchoice []DisplayItem
			for _, v := range sessctx.getContainers() {
				item := DisplayItem{Title: v.Verbal}
				mchoice = append(mchoice, item)
			}
			s := sessctx
			return RespEvent{Type: "Ingredient", Header: s.reqRName, SubHdr: "Containers", List: mchoice}, nil

		case sessctx.request == "search" && len(sessctx.mChoice) > 0:
			// search ingredient/title keywords may return list of recipes
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
			return RespEvent{Header: "Search results for: " + s.reqSearch, Text: s.vmsg, Verbal: s.dmsg, List: mchoice}, nil

		case (sessctx.request == "select" || sessctx.request == "search") && len(sessctx.reqRName) > 0:

			s := sessctx
			mchoice := make([]DisplayItem, 4)
			//for i, v := range []string{ingredient_, utensil_, container_, task_} {
			for i, v := range []string{"List ingredients", "List utensils", "List containers", `Let's start cooking..`} {
				id := strconv.Itoa(i + 1)
				mchoice[i] = DisplayItem{Id: id, Title: v}
			}
			return RespEvent{Header: s.reqRName, Text: s.vmsg, Verbal: s.dmsg, List: mchoice}, nil

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
		return RespEvent{Text: sessctx.vmsg, Verbal: sessctx.dmsg + sessctx.ddata, Error: err.Error()}, nil
	}
	//
	// respond with next record from task (instruction). May support verbal listing of container, utensils at some stage (current displayed listing only)
	//
	return RespEvent{Text: sessctx.vmsg, Verbal: sessctx.dmsg + sessctx.ddata}, nil
}

func main() {
	lambda.Start(handler)
	//p1 := InputEvent{Path: os.Args[1], Param: "sid=asdf-asdf-asdf-asdf-asdf-987654&bkid=" + os.Args[2] + "&rid=" + os.Args[3]}
	//p1 := InputEvent{Path: os.Args[1], Param: "sid=asdf-asdf-asdf-asdf-asdf-987654&rcp=Rhubarb and strawberry crumble cake"}
	//var i float64 = 1.0
	// p1 := InputEvent{Path: os.Args[1], Param: "sid=asdf-asdf-asdf-asdf-asdf-987654&bkid=" + "&srch=" + os.Args[2]}
	// //
	// pIngrdScale = 1.0
	// writeCtx = uDisplay
	// p, _ := handler(p1)
	// if len(p.Error) > 0 {
	// 	fmt.Printf("%#v\n", p.Error)
	// } else {
	// 	fmt.Printf("output:   %s\n", p.Text)
	// 	fmt.Printf("output:   %s\n", p.Verbal)
	// }
}
