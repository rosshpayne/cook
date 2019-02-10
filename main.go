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

type stringS []string

func (w stringS) String() string {
	var a string
	for i, v := range w {
		switch i {
		case 0:
			a = v[strings.LastIndex(v, " ")+1:]
		case 1, 2, 3:
			a += ", " + v[strings.LastIndex(v, " ")+1:]
		default:
			break
		}
	}
	return a
}

//TODO
// change float32 to float64 as this is what dynamoAttribute.Unmarshal uses

// Session Context - assigned from request and Session table.
//  also contains state information relevant to current session and not the next session.
type sessCtx struct {
	operation   string // sourced from Sessions table or request
	sessionId   string // sourced from request. Used as PKey to Sessions table
	request     string // book or recipe request
	reqRName    string // requested recipe name - query param of recipe request
	reqBkName   string // requested book name - query param
	reqRId      string // Recipe Id - 0 means no recipe id has been assigned.  All RId's start at 1.
	reqBkId     string
	reqIngrdCat string // search for recipe. Query ingredients table.
	reqVersion  string // version id, starts at 0 which is blank??
	pkey        string // primary key
	recipe      *RecipeT
	swapBkName  string
	swapBkId    string
	authorS     stringS
	authors     string         // comma separted list of authors
	index       []string       // user defined entries under which recipe is indexed. Sourced from recipe not ingredient.
	indexRecs   []indexRecipeT // processed index entries as saved to dynamo
	dbatchNum   string         // mulit-records sent to display in fixed batch sizes (6 say).
	reset       bool           // zeros []RecId in session table during changes to recipe, as []RecId is recipe dependent
	curreq      int            // bookrecipe_, object_(ingredient,task,container,utensil), listing_(next,prev,goto,modify,repeat)
	questionId  int            // what question is the user responding to with a yes|no.
	dynamodbSvc *dynamodb.DynamoDB
	closeBook   bool
	object      string //container,ingredient,instruction,utensil. Sourced from Sessions table or request
	//updateAdd      int        // dynamodb Update ADD. Operation dependent
	gotoRecId      int        // sourced from request
	recId          int        // current record id for object list (ingredient,task,container,utensil) - sourced from Sessions table (updateSession()
	recIdNotExists bool       // determines whether to create []RecId set attribute in Session  table
	abort          bool       // early return to Alexa
	eol            int        // sourced from Sessions table
	mChoice        []mRecipeT // multi-choice select. Recipe name and ingredient searches can result in mutliple records being returned. Results are saved.
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
}

const (
	// objects to which operations apply - s.object values
	ingredient_ string = "ingredient"
	task_       string = "task"
	container_  string = "container"
	utensil_    string = "utensil"
	recipe_     string = "recipe" // list recipe in book
)

type selectCtxT int

const (
	ctxRecipe selectCtxT = 1
	ctxObject            = 2
)

type objectT int

const (
	// objects to which operations apply - s.object values
	objIngredient objectT = iota
	objTask
	objContainer
	objUtensil
	objRecipe // list recipe in book
)

const (
	// user request grouped into three types
	bookrecipe_ = iota
	object_
	listing_
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

func (s *sessCtx) processRequest() error {
	//
	// merge and validate with last session
	//
	lastSess, err := s.GetSession()
	if err != nil {
		return err
	}
	//
	// initialise session data from last session where missing from current request data
	//
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
		s.reqBkName = lastSess.BkName
	}
	if len(s.reqRName) == 0 {
		s.reqRName = lastSess.RName
	}
	if len(s.reqRId) == 0 {
		s.reqRId = lastSess.RId
	}
	fmt.Printf("reqVersion: [%s]\n", s.reqVersion)
	fmt.Println("len(s.reqVersion) = ", len(s.reqVersion))
	if len(s.reqVersion) == 0 {
		s.reqVersion = lastSess.Ver
	}
	//
	// Recipe Part related data
	//
	if len(s.part) == 0 {
		s.part = lastSess.Part
	}
	if len(s.parts) == 0 {
		s.parts = lastSess.Parts
	}
	if s.peol == 0 {
		s.peol = lastSess.PEOL
	}
	if lastSess.PId > 0 {
		s.pid = lastSess.PId
	}
	if lastSess.Prev != 0 {
		s.prev = lastSess.Prev
	}
	if lastSess.Next != 0 {
		s.next = lastSess.Next
	}
	//
	// assign primary key - used for most dyamo accesses
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
		if len(lastSess.Ver) == 0 {
			s.reset = true
		} else if len(lastSess.Ver) > 0 && s.reqVersion != lastSess.Ver {
			s.reset = true
		}
	}
	//
	// **************** come this far if previous session exists *********************
	//
	// responsd to yes no answer. May assign new book
	//
	if len(s.yesno) > 0 && lastSess.Qid > 0 {
		if s.yesno == "yes" {
			var err error
			switch lastSess.Qid {
			case 20:
				// close active book
				//  as req struct fields still have their zero value they will clear session state during updateSession
				s.reset = true
				s.vmsg = fmt.Sprintf(`%s is closed. You can now search across all recipe books`, lastSess.BkName)
				s.dmsg = fmt.Sprintf(`%s is closed. You can now search across all recipe books`, lastSess.BkName)
				s.reqBkId, s.reqBkName, s.reqRName, s.reqRId, s.eol, s.reset = "", "", "", "", 0, true
				err = s.updateSession()
			case 21:
				// swap book
				s.reqBkId, s.reqBkName, s.reset = lastSess.SwpBkId, lastSess.SwpBkNm, true
				s.vmsg = fmt.Sprintf(`Book [%s] is now open. You can now search or open a recipe within this book`, lastSess.BkName)
				s.dmsg = fmt.Sprintf(`Book [%s] is now open. You can now search or open a recipe within this book`, lastSess.BkName)
				err = s.updateSession()
			default:
				// TODO: log error to error table
			}
			if err != nil {
				return fmt.Errorf("Error: in mergeAndValidateWithLastSession of updateSession() - %s", err.Error())
			}
		}
		if len(s.dmsg) == 0 {
			s.dmsg = lastSess.Dmsg
		}
		s.abort = true
		err = s.updateSession()
		if err != nil {
			return err
		}
		return nil
	}
	//
	// responsd to select from list - sets new book recipe.
	//
	if len(lastSess.MChoice) > 0 && s.selId > 0 {
		// selId is the response from Alexa on the index (ordinal value) of the selected display item
		switch lastSess.CtxSel {
		case ctxRecipe:
			p := lastSess.MChoice[s.selId-1]
			s.reqRId, s.reqRName, s.reqBkId, s.reqBkName = p.RId, p.RName, p.BkId, p.BkName
			s.dmsg = fmt.Sprintf(`Now that you have selected [%s] recipe would you like to list ingredients, cooking instructions, utensils or containers or cancel`, s.reqRName)
			s.vmsg = fmt.Sprintf(`Now that you have selected {%s] recipe would you like to list ingredients, cooking instructions, utensils or containers or cancel`, s.reqRName)
			// chosen recipe, so set select context to object (ingredient, utensil, container, task
			s.selCtx = ctxObject
			s.abort = true
			//
			err = s.updateSession()
			if err != nil {
				return err
			}
			return nil
		case ctxObject:
			//	for i, v := range []string{ingredient_, utensil_, container_, task_} {
			fmt.Println("selId: ", s.selId)
			s.object = objectS[s.selId-1]
			fmt.Println("object: ", s.object)
			// object chosen, nothing more to select for the time being
			s.selCtx = 0
			switch s.object {
			case task_:
				s.operation = "next"
				s.curreq = object_
				s.selClear = true
			case ingredient_:
				s.pkey = s.reqBkId + "-" + s.reqRId
				if s.reqVersion != "" {
					s.pkey += "-" + s.reqVersion
				}
				// fetch recipe name and book name
				r, err := s.recipeRSearch()
				if err != nil {
					break
				}
				// set unit formating mode
				writeCtx = uPrint
				as, err := s.loadActivities()
				if err != nil {
					fmt.Printf("error: %s", err.Error())
				}
				// generate ingredient listing
				s.ingrdList = as.String(r)
				// TODO: persist display fields to session table.
				s.displayHdr = s.reqRName
				s.displaySubHdr = "Book:  " + s.reqBkName
				if len(r.Serves) > 0 {
					s.displaySubHdr += "              Serves: " + r.Serves
				}
				s.abort, s.reset = true, true
				//
				err = s.updateSession()
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
	// 1. ALL conditional paths return  and most update Sessions. Any updateSessions do not change RecId as current state is bookrecipe_.
	//
	// 2. BookName will always co-exist with BookId in session table by the end of this section
	//	  similarly, RecipeName will always co-exist with RecipeId in the session table by the end of this section
	//
	if s.curreq == bookrecipe_ {
		//
		if s.request == "search" {
			// search only applies to recipes, ie. select context Recipe (ctxRecipe).
			s.selCtx = ctxRecipe
			// we have fully populated session context from previous session e.g. BkName etc, now lets see what recipes we find
			// populates sessCtx.mChoice if search results in a list.
			err := s.ingredientSearch()
			if err != nil {
				panic(err)
			}
			s.eol, s.reset, s.object = 0, true, ""
			err = s.updateSession()
			if err != nil {
				return err
			}
			return nil
		}
		if s.showList {
			if len(lastSess.MChoice) > 0 {
				for i, v := range lastSess.MChoice {
					s.dmsg = s.dmsg + fmt.Sprintf("%d. Recipe [%s] in book [%s] by [%s] quantity %s\n", i+1, v.RName, v.BkName, v.Authors, v.Quantity)
					s.vmsg = s.dmsg + fmt.Sprintf("%d. Recipe [%s] in book [%s] by [%s] quantity %s\n", i+1, v.RName, v.BkName, v.Authors, v.Quantity)
				}
			}
			s.abort = true
			return nil
		}
		if s.request == "book" {
			//
			// book requested
			//
			switch len(lastSess.BkId) {
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
				err := s.updateSession()
				if err != nil {
					return err
				}
				return nil
			default:
				// there is already an initialised book for this session
				if s.closeBook || s.reqBkName != lastSess.BkName {
					// closing or open a book different from current opened book
					switch len(lastSess.RName) {
					case 0:
						// no active recipe, then open or close book
						if s.closeBook {
							s.dmsg = fmt.Sprintf("Book %s is closed", lastSess.BkName)
							s.vmsg = fmt.Sprintf("Book %s is closed", lastSess.BkName)
							s.reqBkId, s.reqBkName, s.reqRId, s.reqRName, s.reset = "", "", "", "", true //TODO - should recipe be closed
						} else {
							// no active recipe. Open book.
							s.vmsg = `Please state what recipe you would like from this book or I can list them if you like. Say "list" or recipe name.`
							s.dmsg = `Please state what recipe you would like from this book or I can list them if you like. Say "list" or recipe name.`
						}
						// Book initialises. No recipe provided. Persist.
						s.eol, s.reset = 0, true
						err := (*s).updateSession()
						if err != nil {
							return err
						}
						return nil
					default:
						// open different book with an active recipe.
						//s.reqRName = lastSess.RName
						if len(lastSess.Obj) > 0 && s.eol != lastSess.RecId[objectMap[lastSess.Obj]] {
							if s.closeBook {
								s.dmsg = fmt.Sprintf("You currently have recipe %s open. Do you still want to close the book?", lastSess.RName)
								s.vmsg = fmt.Sprintf("You currently have recipe %s open. Do you still want to close the book?", lastSess.RName)
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
							err := s.updateSession()
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
							err := (*s).updateSession()
							if err != nil {
								return err
							}
							return nil
						}
					}
				} else {
					// open book. same initialised book requested
					switch len(lastSess.RName) {
					case 0:
						// no active recipe
						s.dmsg = `Book is currenlty open. Please request a recipe from the book or say "list" and I will print the recipe names to the display.`
						s.abort = true
						return nil
					default:
						s.dmsg = `Book is already open at recipe ` + lastSess.RName
						s.abort = true
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
			err = s.updateSession()
			if err != nil {
				return err
			}
			return nil
		}
	}
	//
	// request must set recipe id before proceeding to list an object
	if s.curreq == listing_ && len(lastSess.RId) == 0 || s.curreq == object_ && len(lastSess.RId) == 0 {
		s.dmsg = `You have not specified a recipe yet. Please say "recipe" followed by it\'s name`
		s.vmsg = `You have not specified a recipe yet. Please say "recipe" followed by it\'s name`
		s.abort = true
		return nil
	}
	//  if listing (next,prev,repeat,goto - curreq object, listing) without object (container,ingredient,task,utensil) -
	if s.curreq == listing_ && len(lastSess.Obj) == 0 {
		s.dmsg = `You need to say what you want to list. Please say either "ingredients","start cooking","containers" or "utensils". Not hard really..`
		s.vmsg = `You need to say what you want to list. Please say either "ingredients","start cooking","containers" or "utensils". Not hard really..`
		s.abort = true
		return nil
	}
	//  if listing and not finished and object request changes object. Accept and zero or repeat last RecId for requested object.
	if len(lastSess.Obj) > 0 {
		//if !s.finishedListing(lastSess.RecId[objectMap[lastSess.Obj]], objectMap[lastSess.Obj]) && (lastSess.Obj != s.object) {
		fmt.Println(s.object)
		if len(lastSess.RecId) > 0 {
			if s.eol != lastSess.RecId[objectMap[s.object]] && (lastSess.Obj != s.object) {
				// show last listed entry otherwise list first entry
				switch lastSess.RecId[objectMap[s.object]] {
				case 0: // not listed before or been reset after previously completing list
					s.recId = 1 // show first entry
				default: // in the process of listing
					s.recId = lastSess.RecId[objectMap[s.object]] // repeat last shown entry
				}
			}
		}
	}
	// if object specified and different from last one
	if len(s.object) > 0 && len(lastSess.RecId) > 0 && (lastSess.Obj != s.object) {
		// show last listed entry otherwise list first entry
		switch lastSess.RecId[objectMap[s.object]] {
		case 0: // not listed before or been reset after previously completing list
			s.recId = 1 // show first entry
		default: // in the process of listing
			s.recId = lastSess.RecId[objectMap[s.object]] // repeat last shown entry
		}
	}
	//  If listing and not finished and object request  has changed (task, ingredient, container, utensil) reset RecId
	// change in operation does not not need to be taken into account as this is part of the initialisation phase
	// copy object from last session
	if s.curreq == listing_ {
		// object is same as last call
		s.object = lastSess.Obj
	}
	switch s.operation {
	case "goto":
		fmt.Printf("gotoRecId = %d  %d\n", s.gotoRecId, lastSess.EOL)
		//TODO goto not implemented - maybe no use case
		// if s.gotoRecId > lastSess.EOL { //EOL of current object (ingredients,tasks..) data
		// 	// do a repeat operation ie. display current record and define new message
		// 	s.dmsg = "request goes beyond last item in list. Please say again"
		// 	s.vmsg = "request goes beyond last item in list. Please say again"
		// 	s.recId = lastSess.RecId[objectMap[s.object]]
		// 	fmt.Printf("gotoRecId = %d  %d %d\n", s.gotoRecId, lastSess.EOL, s.recId)
		// 	s.abort = true
		// 	return nil
		// } else {
		// 	// use updateAdd value to assign new recId during updateSession
		// 	s.updateAdd = s.gotoRecId - lastSess.RecId[objectMap[s.object]]
		// }
	case "repeat":
		// return - no need to updateSession as nothing has changed.  Select current recId.
		s.recId = lastSess.RecId[objectMap[s.object]]
		s.dmsg, s.vmsg, s.ddata = lastSess.Dmsg, lastSess.Vmsg, lastSess.DData
		s.abort = true
		return nil
	case "prev":
		if len(s.part) == 0 {
			if len(s.object) == 0 {
				return fmt.Errorf("Error: no object defined when in processRequest - next")
			}
			if lastSess.EOL > 0 && len(lastSess.RecId) > 0 {
				if lastSess.RecId[objectMap[s.object]] == 1 {
					s.recId = lastSess.RecId[objectMap[s.object]]
					// s.dPreMsg = "You have reached the end. "
					// s.vPreMsg = "You have reached the end. "
					return nil
				}
				if len(s.object) == 0 {
					return fmt.Errorf("Error: no object defined when in processRequest - next")
				}
				s.recId = lastSess.RecId[objectMap[s.object]] - 1
			}
		} else {
			s.recId = lastSess.Prev
			if s.recId == -1 {
				s.abort = true
				return nil
			}
		}
	case "next":
		if len(s.part) == 0 {
			// no part mode ie. user has not elected to follow a part if one exists, so follows normal non-part mode.
			if len(s.object) == 0 {
				return fmt.Errorf("Error: no object defined when in processRequest - next")
			}
			if lastSess.EOL > 0 && len(lastSess.RecId) > 0 {
				if lastSess.RecId[objectMap[s.object]] == s.eol {
					s.recId = lastSess.EOL
					// s.dPreMsg = "You have reached the end. "
					// s.vPreMsg = "You have reached the end. "
					s.abort = true
					return nil
				}
			}
			s.recId = lastSess.RecId[objectMap[s.object]] + 1
		} else {
			s.recId = lastSess.Next
			if s.recId == -1 {
				// TODO: finished current part. Options go onto next or has recipe been completed - how to determine?
				// CptPart[k]=true
				// save ComPlTedPart to session to keep track completed Part for user.
				s.abort = true
				return nil
			}
		}
	}
	// check if we have Dynamodb Recid Set defined, this will be useful in updateSession
	if len(lastSess.RecId) == 0 {
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
	if s.recId == rec.EOL {
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
	err = s.updateSession()
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
	// 	s.updateSessionEOL()
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

type Item struct {
	Id        string
	Title     string
	SubTitle1 string
	SubTitle2 string
	Text      string
}
type RespEvent struct {
	Position string `json:"Position"` // recId|EOL|PEOL|PName
	Type     string `json:"Type"`
	Header   string `json:"Header"`
	SubHdr   string `json:"SubHdr"`
	Text     string `json:"Text"`
	Verbal   string `json:"Verbal"`
	Error    string `json:"Error"`
	List     []Item `json:"List"` // id|Title1|subTitle1|SubTitle2|Text
}

//
//  handler executed via API Gateway via ALexa Lambda
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
	}

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
		sessctx.abort, sessctx.reset = true, true
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
		sessctx.abort, sessctx.reset = true, true
	//
	case "genSlotValues":
		err = sessctx.generateSlotEntries()
		if err != nil {
			break
		}
		sessctx.abort, sessctx.reset = true, true
	case "book", "recipe", "select", "search", "list", "yesno", "version":
		sessctx.curreq = bookrecipe_
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
			sessctx.reqIngrdCat = strings.ToLower(sq)
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
				err = fmt.Errorf("%s: %s", "Error in converting int of select operation \n\n", err.Error())
			} else {
				sessctx.selId = i
			}
			// remainder of sessctx populated in processRequest
		}
	// object ingredient_, utensil_
	case container_, task_:
		sessctx.object = sessctx.request
		sessctx.curreq = object_
		sessctx.operation = "next"
	// operation
	case "next", "prev", "goto", "repeat", "modify":
		sessctx.operation = sessctx.request
		sessctx.curreq = listing_
		// sessctx.object will depend on value from last session, which must exist for an operation.
		switch sessctx.request {
		case "goto":
			var i int
			i, err = strconv.Atoi(request.QueryStringParameters["goId"])
			if err != nil {
				err = fmt.Errorf("%s: %s", "Error in converting int of goto operation ", err.Error())
			} else {
				sessctx.gotoRecId = i
			}
		}
		// case "yes":  check questions asked from last session (session table)
		// case "no":
	}
	if err != nil {
		return RespEvent{Text: sessctx.vmsg, Verbal: sessctx.dmsg + sessctx.ddata, Error: err.Error()}, nil
	}
	//
	// compare with last session if it exists and update remaining session data
	//
	err = sessctx.processRequest()
	if err != nil {
		return RespEvent{Text: sessctx.vmsg, Verbal: sessctx.dmsg + sessctx.ddata, Error: err.Error()}, nil
	}
	if sessctx.curreq == bookrecipe_ || sessctx.abort {

		if sessctx.request == "select" && sessctx.object == ingredient_ {
			var mchoice []Item
			for _, v := range strings.Split(sessctx.ingrdList, "\n") {
				item := Item{Title: v}
				mchoice = append(mchoice, item)
			}
			s := sessctx
			return RespEvent{Type: "Ingredient", Header: s.displayHdr, SubHdr: s.displaySubHdr, List: mchoice}, nil
		}
		if sessctx.request == "select" && sessctx.object == container_ {
			var mchoice []Item
			fmt.Println("Here..")
			ss := sessctx.getContainers()
			fmt.Printf("ss: %#v\n", ss)
			for _, v := range ss {
				fmt.Println(v)
				item := Item{Title: v}
				mchoice = append(mchoice, item)
			}
			s := sessctx
			return RespEvent{Type: "Ingredient", Header: s.displayHdr, SubHdr: s.displaySubHdr, List: mchoice}, nil
		}
		if len(sessctx.mChoice) > 0 && sessctx.request == "search" {

			var mchoice []Item
			for _, v := range sessctx.mChoice {
				var (
					subTitle2 string
				)
				id := strconv.Itoa(v.Id)
				if a := strings.Split(v.Authors, ","); len(a) > 1 {
					subTitle2 = "Authors: " + v.Authors
				} else {
					subTitle2 = "Author: " + v.Authors
				}
				item := Item{Id: id, Title: v.RName, SubTitle1: "Book: " + v.BkName, SubTitle2: subTitle2, Text: v.Quantity}
				mchoice = append(mchoice, item)
			}
			//fmt.Println(lines)
			s := sessctx
			return RespEvent{Header: "Search results for: " + s.reqIngrdCat, Text: s.vmsg, Verbal: s.dmsg, List: mchoice}, nil
		}
		if sessctx.request == "select" && sessctx.selCtx == ctxObject {
			s := sessctx
			mchoice := make([]Item, 4)
			//for i, v := range []string{ingredient_, utensil_, container_, task_} {
			for i, v := range []string{"List ingredients", "List utensils", "List containers", `Let's start cooking..`} {
				id := strconv.Itoa(i + 1)
				mchoice[i] = Item{Id: id, Title: v}
			}
			return RespEvent{Header: s.reqRName, Text: s.vmsg, Verbal: s.dmsg, List: mchoice}, nil
		}
		return RespEvent{Text: sessctx.vmsg, Verbal: sessctx.dmsg + sessctx.ddata}, nil
	}
	//
	// session ctx validated and fully populated - now fetch required record
	//
	err = sessctx.getRecById() // returns [text, verbal] response
	if err != nil {
		return RespEvent{Text: sessctx.vmsg, Verbal: sessctx.dmsg + sessctx.ddata, Error: err.Error()}, nil
	}
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
