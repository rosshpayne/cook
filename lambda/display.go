package main

import (
	_ "context"
	_ "encoding/json"
	"fmt"
	_ "os"
	"strconv"
	"strings"

	"github.com/rosshpayne/cook/global"
)

type WelcomeT struct {
	msg     string
	Bkids   []string // registered book ids
	Display []DisplayItem
	//
	request string
}

type Displayer interface {
	GenDisplay(s *sessCtx) RespEvent
}

type RecipeListT []mRecipeT
type IngredientT string

type PartT struct {
	Index  string `json:"Idx"`   // short name which is attached to each activity JSON.
	Type_  string `json:"Typ"`   // either Part, Division or Thread
	Title  string `json:"Title"` // long name which is printed out in the ingredients listing
	Start  int    `json:"Start"` // SortK value in T-?-? that has first instruction for the partition
	InvisI bool   `json:"InvsI"` // portion after "|" invisible in Ingredient menu. Applies to Part but not forced.
}
type PartS []PartT

type BookT string

// part of session data that is persisted in state data under attribute I
// Activity (table) => taskRecT (by GenerateTasks) => InstructionT (by cacheInstruction)
type InstructionT struct {
	Text      string `json:"Txt"` // all Linked preps combined text into this field
	Verbal    string `json:"Vbl"`
	Part      string `json: "Pt"` // part index name - combines normal part and division as only one of these is displayed at a time.
	Thread    int    `json:"Thrd"`
	MergeThrd int    `json:"MThrd"`
	Division  string `json:"Div"`
	EOL       int    `json:"EOL"` // End-Of-List. Max Id assigned to each record
	PEOL      int    `json:"PEOL"`
	PID       int    `json:"PID"` // id within a part
}

type InstructionS []InstructionT

type ThreadT struct {
	Instructions InstructionS `json:"I"`
	Id           int          `json:"id"`     // active record in Instructions starting at 1
	Thread       int          `json:"thread"` // thread id
	EOL          int          `json:"eol"`    // total instructions across all threads
}

type Threads []ThreadT

type ObjMenuT struct {
	id   int
	item string
}

type ObjMenu []ObjMenuT

var objMenu ObjMenu = []ObjMenuT{
	ObjMenuT{0, `Ingredients `},
	ObjMenuT{1, `Containers `},
	ObjMenuT{2, `Size Container`},
	ObjMenuT{3, `Instructions`},
}

// instance of below type saved to state data in dynamo
type DispContainerT struct {
	Type_      string `json:"Type"`
	Shape      string
	Dimension  string `json:"dim"`  // recipe container size
	UDimension string `json:"udim"` // user defined container size
	Unit       string
}

type ContainerS []string

type menuList []int

type DisplayItem struct {
	Id        string
	Title     string
	SubTitle1 string
	SubTitle2 string
	Text      string
}
type RespEvent struct {
	Position string        `json:"Position"` // recId|EOL|PEOL|PName
	Timer    int           `json:"Tmr"`
	BackBtn  bool          `json:"BackBtn"`
	Type     string        `json:"Type"`
	Header   string        `json:"Header"`
	SubHdr   string        `json:"SubHdr"`
	Text     string        `json:"Text"`
	Verbal   string        `json:"Verbal"`
	Height   string        `json:"Height"`
	Error    string        `json:"Error"`
	Hint     string        `json:"Hint"`
	List     []DisplayItem `json:"List"` // recipe data: id|Title1|subTitle1|SubTitle2|Text
	ListA    []DisplayItem `json:"ListA"`
	ListB    []DisplayItem `json:"ListB"`
	ListC    []DisplayItem `json:"ListC"`
	ListD    []DisplayItem `json:"ListD"`
	ListE    []DisplayItem `json:"ListE"`
	ListF    []DisplayItem `json:"ListF"`
	Color1   string        `json:"Color1"`
	Color2   string        `json:"Color2"`
	Thread1  string        `json:"Thread1"`
	Thread2  string        `json:"Thread2"`
}

const (
	WELCOME int = iota + 1
	THREADS
	RECIPELIST
)

// cacheInstructions copies data from T- items in recipe table to session data (instructions) that is preserved

func (t Threads) GenDisplay(s *sessCtx) RespEvent {

	var (
		p []string
		//eol     int
		peol int
		part string
		//pid     string
		hdr  string
		subh string
		hint string
	)

	getLongName := func(index string) string {
		if index == "" || index == "nopart_" {
			return ""
		}
		for _, v := range s.parts {
			if v.Index == index {
				return v.Title
			}
		}
		//panic(fmt.Errorf("Error: in getInstruction, recipe part index [%s] not found in s.parts ", index))
		return ""
	}
	threadName := func(thread int) string {
		for i := 0; i < len(s.parts); i++ {
			v := &s.parts[i]
			if v.Type_ == "Thrd" && v.Index == strconv.Itoa(thread) {
				return v.Title
			}
		}
		return "no-thread-found,"
	}
	fmt.Println("GenDisplay:  Threads")
	if len(t) == 0 {
		s.derr = "Error: internal, instructions has not been cached"
	}
	//
	// check id within limits
	//
	//var id = t[s.cThread].id // cThread either zero (for no thread), 1 for one thread or 2 for two threads. Number of actual threads is len(t)
	fmt.Println("cThread = ", s.cThread)
	fmt.Println("object = ", s.object)
	if s.cThread == 0 {
		s.oThread = 0
	}
	id := s.recId[objectMap[s.object]] // is the next record id to speak
	//
	fmt.Println("id = ", id)
	if id > 0 && id <= len(t[s.cThread].Instructions) {
		cthread := t[s.cThread].Instructions[id-1].Thread
		if cthread == 0 {
			s.cThread, s.oThread = 0, 0
		}
		fmt.Println("new cThread = ", cthread)
	}

	fmt.Println("object = ", s.object)
	fmt.Println("id = ", id)
	//
	// check id within instruction set range - don't change to a lower thread here. Must be explicitly done by resume request.
	//
	switch {

	case id < 1 && len(t) == 1:
		id = 1
		s.recId[objectMap[s.object]] = 1

	case id < 1 && len(t) > 1:
		// multithreade app - entered thread for first time
		id = 1
		s.recId[objectMap[s.object]] = 1

	case id > len(t[s.cThread].Instructions) && len(t) == 1:
		// single threaded recipe
		s.derr = "Reached last instruction"
		id = len(t[s.cThread].Instructions)
		s.recId[objectMap[s.object]] = id

	case id > len(t[s.cThread].Instructions) && len(t) > 1 && s.cThread == 0: // thread 1
		// i.e. current thread in multiple threaded recipe so check if all previous threads completed.
		s.cThread, s.oThread = 2, 1
		s.recId[objectMap[s.object]], id, t[s.cThread].Id = 1, 1, 1
	case id > len(t[s.cThread].Instructions) && len(t) > 1 && s.cThread == 1: // thread 1
		// i.e. current thread in multiple threaded recipe so check if all previous threads completed.
		s.cThread, s.oThread = 2, 1
		//id = len(t[s.cThread].Instructions)
		t[s.cThread].Id++
		id = t[s.cThread].Id
		s.recId[objectMap[s.object]] = t[s.cThread].Id
		if t[s.cThread].Id == len(t[s.cThread].Instructions) {
			// previous thead completed. Have reached end
			//s.derr = "Thread " + threadName() + " completed"
			id = len(t[s.cThread].Instructions)
			s.cThread, s.oThread = s.oThread, s.cThread
			if t[s.cThread].Id == len(t[s.cThread].Instructions) {
				s.derr = "Recipe completed"
			} else {
				s.recId[objectMap[s.object]] = 1
				id = t[s.cThread].Id + 1
			}
		}

	case id > len(t[s.cThread].Instructions) && s.cThread == 2:
		// recipe has forked or resumed thread has completed.
		switch s.oThread {
		case 0:
			// recipe forked
			s.oThread = s.oThread + 1
			s.cThread = s.oThread + 1
			id = 1
			fmt.Println("recipe forked - othr, cthr, id ", s.oThread, s.cThread, id)
		case 1:
			// resumed thread completed
			s.oThread = -1 // resumed thread completed
			s.cThread = s.cThread + 1
			id = t[s.cThread].Id
			fmt.Println("resumed thread completed - othr, cthr, id ", s.oThread, s.cThread, id)
		}
		s.recId[objectMap[s.object]] = id
		if t[s.cThread].Id == len(t[s.cThread].Instructions) {
			// previous thead completed. Have reached end
			s.derr = "Recipe completed"
			id = len(t[s.cThread].Instructions)
			s.recId[objectMap[s.object]] = id
		}
	}
	//
	t[s.cThread].Id = id
	//
	fmt.Println("t[s.cThread].Id, s.cThread, id  = ", t[s.cThread].Id, s.cThread, id)
	//rec := &t[s.cThread].Instructions[id-1]
	//eol := t[s.cThread].EOL
	//
	// generate display
	//
	if len(s.derr) > 0 {
		hdr = "** Alert **   " + s.derr
	} else {
		hdr = s.reqRName
	}
	//
	if len(s.part) > 0 {
		if s.part == CompleteRecipe_ {
			p = strings.Split(s.part, "|")
			peol = len(t[s.cThread].Instructions)
			hdr += "  -  " + strings.ToUpper(p[0])
		} else {
			p = strings.Split(s.part, "|")
			hdr += "  -  " + strings.ToUpper(p[0])
			peol = len(t[s.cThread].Instructions)
		}
		sf := strconv.FormatFloat(global.GetScale(), 'g', 2, 64)
		subh = "Cooking Instructions  -  " + strconv.Itoa(id) + " of " + strconv.Itoa(peol) + "   (Fixed Scale: " + sf + ")"
	} else {
		// p = getLongName(rec.part)
		// p = strings.Split(s.part, "|")
		// hdr += "  -  " + strings.ToUpper(p[0])
		eol := len(t[s.cThread].Instructions)
		sf := strconv.FormatFloat(global.GetScale(), 'g', 2, 64)
		subh = "Cooking Instructions  -  " + strconv.Itoa(id) + " of " + strconv.Itoa(eol) + "   (Fixed Scale: " + sf + ")"
	}

	fmt.Println("switch on thread: ", t[s.cThread].Thread)
	fmt.Println("oThread: ", s.oThread)
	//
	// local funcs
	//
	SectA := func(thread int) []DisplayItem {
		var rows int
		lines := 1
		list := make([]DisplayItem, lines)
		for k, n, ir := lines-1, t[thread].Id-1, t[thread].Instructions; n > 0 && rows < lines; rows++ {
			list[k] = DisplayItem{Title: ir[n-1].Text}
			n--
			k--
		}

		return list
	}
	SectB := func(thread int) []DisplayItem {
		list := make([]DisplayItem, 1)
		id := t[thread].Id
		if t[thread].Id == 0 {
			t[thread].Id = 1
			id = t[thread].Id
		}
		if id == 0 {
			// thread not accessed yet - show last intruction in previous thread
			thread--
			id = t[thread].Id
		}
		fmt.Println("Sect B: id,thread = ", id, thread)
		list[0] = DisplayItem{Title: "<speak>" + t[thread].Instructions[id-1].Text + "</speak>"}
		return list
	}
	SectC := func(thread int, lines int, terminate bool) []DisplayItem {
		var list []DisplayItem
		var rows int
		for tr := thread; tr < len(t); tr++ {
			var start int
			if tr == thread {
				start = t[tr].Id
			} else {
				start = 0
			}
			for n, ir := start, t[tr].Instructions; n < len(t[tr].Instructions) && rows < lines; rows++ {
				fmt.Println("n: ", n)
				item := DisplayItem{Title: ir[n].Text}
				list = append(list, item)
				n++
			}
			if terminate {
				break
			}
		}
		// if rows == lines {
		// 	list[rows-1] = DisplayItem{Title: "more.."}
		// }

		return list
	}
	//
	// is there an other thread means there are two active threads that need to be displayed on a Echo Show.
	//

	switch s.oThread {
	case 0, -1:
		//
		// only one thread, split instructions across three sections - used by echo Show only
		//
		tc := s.cThread
		fmt.Println("tc=", tc)
		listA := SectA(tc)
		listB := SectB(tc)
		listC := SectC(tc, 3, false)
		//
		rec := &t[s.cThread].Instructions[id-1]
		// eol = strconv.Itoa(rec.EOL)
		// peol = strconv.Itoa(rec.PEOL)
		part = getLongName(rec.Part)
		// pid = strconv.Itoa(rec.PID)
		//s.eol, s.peol, s.part, s.pid = rec.EOL, rec.PEOL, part, rec.PID
		s.part = part

		speak := "<speak>" + rec.Verbal + "</speak>"
		var height string
		fmt.Println(len(rec.Verbal), " ", rec.Verbal)
		l := rec.Verbal
		switch {
		case len(l) < 65:
			height = "15vh"
		case len(l) < 120:
			height = "25vh"
		case len(l) < 180:
			height = "35vh"
		default:
			height = "45vh"
		}
		fmt.Println("height: ", height)
		//
		s.menuL = nil
		//
		// alert if inappropriate verbal interaction
		//
		type_ := "Tripple"
		if len(s.derr) > 0 {
			type_ += "Err"
		} else {
			s.updateState()
		}
		if len(s.parts) > 0 {
			hint = `hint:  "next", "previous", "repeat", "list ingredients", "list containers", "list parts", back", "restart" `
		} else {
			hint = `hint:  "next", "previous", "repeat", "list ingredients", "list containers", "back", "restart" `
		}
		fmt.Println("hint: ", hint)

		return RespEvent{Type: type_, BackBtn: true, Header: hdr, SubHdr: subh, Hint: hint, Text: rec.Text, Height: height, Verbal: speak, ListA: listA, ListB: listB, ListC: listC, Error: s.derr}

	default:
		// two threads with 3 sections in each. should always display threads 1 and 2 in that order, never thread 0

		tc := s.oThread
		type_ := "threadedBottom"
		color1 := "yellow"
		if s.cThread < s.oThread {
			tc = s.cThread
			color1 = "green"
			type_ = "threadedTop"
		}
		trName1 := threadName(tc)
		// lower thread
		listA := SectA(tc)
		listB := SectB(tc)
		listC := SectC(tc, 2, true)
		// higher thread
		tc = s.cThread
		color2 := "green"
		if s.oThread > s.cThread {
			tc = s.oThread
			color2 = "yellow"
		}
		trName2 := threadName(tc)
		listD := SectA(tc)
		listE := SectB(tc)
		listF := SectC(tc, 3, false)

		fmt.Println("cThread, id = ", s.cThread, id)
		//
		// rec is the Instruction record, read from the Thread which is held in state.
		//
		rec := &t[s.cThread].Instructions[id-1]
		fmt.Println("type_ : ", type_)
		fmt.Printf("** rec = [%#v]\n", rec)
		// eol = strconv.Itoa(rec.EOL)
		// peol = strconv.Itoa(rec.PEOL)
		part = getLongName(rec.Part)
		// pid = strconv.Itoa(rec.PID)
		//s.eol, s.peol, s.part, s.pid = rec.EOL, rec.PEOL, part, rec.PID
		s.part = part
		fmt.Println("4 cThread, id , part = ", s.cThread, id, part)
		speak := "<speak>" + rec.Verbal + "</speak>"

		s.menuL = nil
		s.updateState()
		hint = "hint:  next, previous, say again, resume"
		return RespEvent{Type: type_, BackBtn: true, Header: hdr, SubHdr: subh, Hint: hint, Text: rec.Text, Verbal: speak, ListA: listA, ListB: listB, ListC: listC,
			ListD: listD, ListE: listE, ListF: listF, Color1: color1, Color2: color2, Thread1: trName1, Thread2: trName2,
		}
	}
}

// type apldisplayT struct {
// 	Type     int           `json:"typ"`
// 	DispList []DisplayItem `json:"dlist"`
// }

func (w *WelcomeT) GenDisplay(s *sessCtx) RespEvent {
	var (
		hdr    string
		subh   string
		title  string
		hint   string
		openBk string
	)
	//
	OpenBkName := func() string {
		var bkname string
		for i, v := range w.Display {
			fmt.Println("OpenBkName: ", v.Title)
			if i < 2 {
				bkname = v.Title[:strings.Index(v.Title, " by ")]
				continue
			}
			break
		}
		return `"open book ` + bkname + `"`
	}

	if w == nil {
		panic(fmt.Errorf("WelcomeT: w not assigned"))
	}
	fmt.Printf("GenDisplay:  WelcomeT %#v\n", *w)

	if len(w.Display) == 0 || (len(w.Bkids) != len(s.bkids) && s.back == false) || s.openBkChange {
		fmt.Println("**  push or update.... len(s.state) = ", len(s.state))
		fmt.Println(" openBkChange.. ", s.openBkChange)
		fmt.Println("w.Bkids ", len(w.Bkids))
		fmt.Println("s.bkids ", len(s.bkids))
		// s.bkids is the latest book register value
		// w.Bkids is the cached value and hence maybe out-of-date
		w.Bkids = s.bkids
		fmt.Printf("w.Bkids = %#v\n", w.Bkids)
		var list []DisplayItem
		for _, v := range s.bkids {
			s.reqBkId = v
			err := s.bookNameLookup()
			if err != nil {
				panic(err)
			}
			msg := s.reqBkName + "  by " + s.authors
			// if i < 2 {
			// 	openBk = `"open book ` + s.reqBkName + `"`
			// }
			list = append(list, DisplayItem{Title: msg})
		}
		w.Display = list
		// clear all book/recipe data as populated by bookNameLookup(), as we currently have not user selected book/recipe.
		s.reqBkId, s.reqBkName, s.reqRId, s.reqRName, s.authors = "", "", "", "", ""
		//
		s.welcome = w
		if len(s.state) == 0 {
			s.pushState()
		} else {
			s.updateState()
		}
	}

	// list for screen display only not to save to session data.
	list := make([]DisplayItem, len(w.Display))
	copy(list, w.Display)
	//
	hdr = "Welcome to your Ebury Press Cook books on Alexa"
	srch := `, "search [ingredient..]", "search [keyword]", "search [recipe-name]", "search [any-part-of-recipe-name]" e.g. "search tart", "search tarragon", "search chocolate cake"`

	type_ := "Start"
	if len(s.derr) > 0 {
		type_ += "Err"
	}

	if len(w.msg) > 0 {
		fmt.Println("w.msg -------- ", w.msg)
		title = w.msg
		//type_ = "Start2"
		s.saveState = false // no point in saving state as nothing to transfer to next session.

	} else if len(w.Bkids) > 0 || s.back {
		fmt.Printf("w.Bkids > 0 or s.back ")
		if len(w.Bkids) > 1 {
			title = "Listed below are the books registered to this device. Searches will be applied to all these books unless you open one"
		} else {
			title = "You have the following book registered to this device. "
		}

		hint = `hint: ` + OpenBkName() + srch

		if len(s.reqOpenBk) > 0 {
			fmt.Println("reqOpenBK --------")
			bk := strings.Split(string(s.reqOpenBk), "|")
			if s.back {
				title = bk[1] + " is open. Searches will be restricted to this book"
			} else {
				title = bk[1] + " is now open. Searches will be restricted to this book"
			}
			ob := strings.Split(string(s.reqOpenBk), "|")
			for i, v := range w.Bkids {
				if v == ob[0] {
					list[i].Title += "  (opened and searcheable)"
				}
			}
			hint = `hint: "close book"` + srch

		} else if len(s.CloseBkName) > 0 {
			title = s.CloseBkName + " is now closed. Searches will be across all your books"
		}

	}

	if len(w.request) > 0 {
		type_ = w.request // nb: email - as in get me email. index.js handles it.
		if len(openBk) == 0 {
			hint = `hint: ` + OpenBkName() + srch
		}
	}

	return RespEvent{Type: type_, BackBtn: false, Header: hdr, SubHdr: subh, Hint: hint, Text: title, Verbal: title, List: list, Error: s.derr}
}

func (c ContainerS) GenDisplay(s *sessCtx) RespEvent {

	fmt.Printf("in GenDisplay for containers: %#v\n", c)
	hdr := s.reqRName
	subh := "Containers and Utensils"
	if global.GetScale() < 1 {
		sf := strconv.FormatFloat(global.GetScale(), 'g', 2, 64)
		subh += "  (scale: " + sf + ")"
	}
	var hint string
	if len(s.parts) > 0 {
		hint = `hint: "list ingredients", "list instructions", "list parts", back", "restart" `
	} else {
		hint = `hint: "list ingredients", "list instructions", "back", "restart" `
	}
	if len(s.reqOpenBk) > 0 {
		hint += `, "close book" `
	}
	var list []DisplayItem
	for _, v := range c {
		di := DisplayItem{Title: v}
		// add blank line to separate footer *
		if len(v) > 1 && v[1] == '*' {
			list = append(list, DisplayItem{Title: " "})
		}
		list = append(list, di)
	}
	type_ := "Ingredient"
	if len(s.derr) > 0 {
		type_ += "Err"
	}
	return RespEvent{Type: type_, BackBtn: true, Header: hdr, SubHdr: subh, Hint: hint, List: list, Error: s.derr}
}

func (p PartS) GenDisplay(s *sessCtx) RespEvent {
	// parts sourced from recipe.parts (json list loaded into go slice)
	var (
		hdr  string
		subh string
		hint string
		k    int
	)
	fmt.Printf("in GenDisplay for PartS: %#v\n", p)
	hdr = s.reqRName
	sf := strconv.FormatFloat(global.GetScale(), 'g', 2, 64)
	subh = `Recipe is divided into parts. Select first option to follow complete recipe  (Scale: ` + sf + ")"
	//
	list := make([]DisplayItem, 1)
	list[0] = DisplayItem{Id: "1", Title: CompleteRecipe_}
	for _, v := range p {
		// ignore threads and invisible parts
		if v.Type_ == "Thrd" {
			continue
		}
		id := strconv.Itoa(k + 2)
		s := strings.Split(v.Title, "|")
		if len(s) == 1 {
			list = append(list, DisplayItem{Id: id, Title: s[0]})
		} else {
			list = append(list, DisplayItem{Id: id, Title: s[0], SubTitle1: s[1]})
		}
		k++
	}
	type_ := "PartList"
	if len(s.derr) > 0 {
		type_ += "Err"
	}
	hint = `hint: "select [integer]", "list ingredients", "list containers", "back", "restart"`
	return RespEvent{Type: type_, BackBtn: true, Header: hdr, SubHdr: subh, Hint: hint, Height: "90", Verbal: s.vmsg, List: list, Error: s.derr}

}

func (i IngredientT) GenDisplay(s *sessCtx) RespEvent {

	var (
		list   []DisplayItem
		subhdr string
		hint   string
	)
	fmt.Printf("in GenDisplay for Ingredient: %#v\n", i)
	for _, v := range strings.Split(string(i), "\n") {
		item := DisplayItem{Title: v}
		list = append(list, item)
	}
	sf := strconv.FormatFloat(global.GetScale(), 'g', 2, 64)
	subhdr = "Ingredients       (Scale: " + sf + " )"
	if len(s.parts) > 0 {
		hint = `hint:  "scale [integer]", "scale reset", "list containers", "list insructions", "list parts", "back", "restart"`
	} else {
		hint = `hint:  "scale [integer]", "scale reset", "list containers", "list insructions", "back", "restart"`
	}
	type_ := "Ingredient"
	if len(s.derr) > 0 {
		type_ += "Err"
	}
	return RespEvent{Type: type_, BackBtn: true, Header: s.reqRName, SubHdr: subhdr, List: list, Hint: hint, Error: s.derr}

}

func (r RecipeListT) GenDisplay(s *sessCtx) RespEvent {
	// display recipes
	var (
		list    []DisplayItem
		op      string
		hdr     string
		subhdr  string
		type_   string
		hint    string
		backBtn bool
	)
	fmt.Printf("in GenDisplay for RecipeList: %#v\n", r)
	if len(s.reqOpenBk) > 0 {
		op = "Opened "
	}
	//
	// mutli-choice recipes
	//
	for _, v := range r {
		var item DisplayItem
		id := strconv.Itoa(v.Id)

		if len(v.Serves) > 0 {
			item = DisplayItem{Id: id, Title: v.RName, SubTitle1: op + "Book: " + v.BkName, SubTitle2: "Serves:  " + v.Serves, Text: v.Quantity}
		} else {
			var subTitle2 string
			if a := strings.Split(v.Authors, ","); len(a) > 1 {
				subTitle2 = "Authors: " + v.Authors
			} else {
				subTitle2 = "Author: " + v.Authors
			}
			item = DisplayItem{Id: id, Title: v.RName, SubTitle1: op + "Book: " + v.BkName, SubTitle2: subTitle2, Text: v.Quantity}
		}
		list = append(list, item)
	}
	type_ = "Search"
	if len(s.derr) > 0 {
		type_ += "Err"
	}
	hint = `hint: "select [integer]", "back", "restart"`
	hdr = "Search results for: " + s.reqSearch
	if len(s.reqOpenBk) > 0 {
		hint = `hint: "select [integer]", "close book", "back", "restart"`
		bk := strings.Split(string(s.reqOpenBk), "|")
		subhdr = "Open book: " + bk[1]
	} else {
		subhdr = "Searched across all your books"
	}
	backBtn = true
	if len(s.state) < 2 {
		backBtn = false
	}

	return RespEvent{Type: type_, BackBtn: backBtn, Header: hdr, SubHdr: subhdr, Hint: hint, Text: s.vmsg, Verbal: s.dmsg, List: list, Error: s.derr}
}

func (o ObjMenu) GenDisplay(s *sessCtx) RespEvent {
	var (
		hdr     string
		subh    string
		hint    string
		backBtn bool
		noScale bool
	)

	fmt.Println("GenDisplay:  ObjMenu")
	if len(s.derr) > 0 {
		fmt.Printf("with Error\n")
	}

	hdr = s.reqRName
	if len(s.reqOpenBk) > 0 {
		subh = "Opened book: " + s.reqBkName + "  Authors: " + s.authors
	} else {
		subh = "Book:  " + s.reqBkName + "  Authors: " + s.authors
	}
	//	if global.GetScale() < 1.0 {
	sf := strconv.FormatFloat(global.GetScale(), 'g', 2, 64)
	subh += "  (Scale: " + sf + " )"
	//	}
	//
	// if back button pressed then s.menuL is assigned via state pop(). menuL is empty, at this point, during normal forward processing.
	//
	var list []DisplayItem
	fmt.Println("len(menuL) = ", len(s.menuL))
	switch len(s.menuL) {
	case 0:
		ct, err := s.getScaleContainer()
		if err != nil {
			return RespEvent{Text: s.vmsg, Verbal: s.dmsg, Error: err.Error()}
		}
		size := 4
		if ct == nil {
			// recipe has no scalable container
			fmt.Println("*** no scalable container found.....")
			noScale = true
			size = 3
		} else {
			s.dispCtr = &DispContainerT{Type_: ct.Label, Shape: ct.Measure.Shape, Dimension: ct.Measure.Dimension, Unit: ct.Measure.Unit}
			fmt.Printf("** dispCtr %#v\n", *(s.dispCtr))
		}

		list = make([]DisplayItem, size)
		s.menuL = make(menuList, size)
		k := 0
		for i, v := range o {
			if i == 2 && noScale {
				continue
			}
			id := strconv.Itoa(k + 1)
			list[k] = DisplayItem{Id: id, Title: v.item}
			s.menuL[k] = v.id
			k++
		}
	default:
		//if len(s.menuL) > 0 {
		list = make([]DisplayItem, len(s.menuL))
		for i, v := range s.menuL {
			id := strconv.Itoa(v + 1)
			list[i] = DisplayItem{Id: id, Title: objMenu[v].item}
		}
		//}
	}

	backBtn = true
	if len(s.state) < 2 {
		backBtn = false
	}
	if !s.back {
		// only update state if going forward not going back
		s.updateState()
	}
	type_ := "Search"
	if len(s.derr) > 0 {
		type_ += "Err"
	}
	if len(s.parts) > 0 {
		hint = `hint: "select [integer]", "list instructions", "list ingredients", "list parts" , "list containers"`
	} else {
		hint = `hint: "select [integer]", "list instructions", "list ingredients", "list containers"`
	}
	if len(s.reqOpenBk) > 0 {
		hint += `,"close book"`
	}
	fmt.Println("Screen: ", type_)

	return RespEvent{Type: type_, BackBtn: backBtn, Header: hdr, SubHdr: subh, Text: s.vmsg, Hint: hint, Verbal: s.vmsg, List: list, Error: s.derr}

}

// func (b BookT) GenDisplay(s *sessCtx) RespEvent {
// 	var (
// 		hdr     string
// 		subh    string
// 		hint    string
// 		type_   string
// 		text    string
// 		backBtn bool
// 	)

// 	backBtn = true
// 	if len(s.state) < 2 {
// 		backBtn = false
// 	}
// 	fmt.Println("in GenDisplay for BookT: [", string(b), "]")
// 	id := strings.Split(string(b), "|")
// 	fmt.Printf("id = %d %#v\n", len(id), id)
// 	switch len(id) {
// 	case 0, 1:
// 		// only bkid
// 		if s.request == "close" {
// 			hdr = s.CloseBkName + " closed."
// 			subh = "Future searches will be across all your books"
// 		} else {
// 			type_ = "Select"
// 			hdr = "Issue with opening book " + s.reqBkName
// 			subh = s.dmsg
// 			list := make([]DisplayItem, 2)
// 			list[0] = DisplayItem{Id: "1", Title: "Yes"}
// 			list[1] = DisplayItem{Id: "2", Title: "No"}
// 			return RespEvent{Type: type_, BackBtn: backBtn, Header: hdr, SubHdr: subh, Text: s.vmsg, Verbal: s.dmsg, List: list}
// 		}
// 	default:
// 		// book successfully opened. No errors can occur during book close so b will always be empty.
// 		BkName := id[1]
// 		authors := id[2]
// 		hdr = "Opened book " + BkName + "  by " + authors
// 		text = "All searches will be restricted to " + BkName + " until it is closed"
// 	}
// 	fmt.Println("in GenDisplay: ", hdr)
// 	type_ = "OpenBook"
// 	hint = "hint: search orange tart, search chocolate cake, close book"
// 	return RespEvent{Type: type_, Header: hdr, SubHdr: subh, Hint: hint, Text: text, Verbal: text}
// }

func (c *DispContainerT) GenDisplay(s *sessCtx) RespEvent {
	var (
		sf   string
		hdr  string
		subh string
		hint string
		text string
		list []DisplayItem
	)
	fmt.Printf("in GenDisplay for DispContainerT: %#v\n", *c)
	if c == nil {
		panic("in GenDisplay(): DispContainerT instance is nil ")
	}

	if s.ctSize > 0 || len(c.UDimension) > 0 {
		// response to user size request
		cdim, err := strconv.Atoi(c.Dimension)
		if err != nil {
			s.derr = err.Error()
		}
		if s.ctSize > 0 {
			c.UDimension = strconv.Itoa(s.ctSize)
			global.SetScale(float64(s.ctSize*s.ctSize) / float64(cdim*cdim))
		} else if len(c.UDimension) > 0 {
			udim, err := strconv.Atoi(c.UDimension)
			if err != nil {
				s.derr = err.Error()
			}
			global.SetScale(float64(udim*udim) / float64(cdim*cdim))
		}
		// persist  new scaleF
		switch s.request {
		case "select":
			s.pushState()
		case "resize":
			s.updateState()
		}
		sf = strconv.FormatFloat(global.GetScale(), 'g', 2, 64)
		fmt.Println("s.scalef, global.GetScale() = ", s.scalef, global.GetScale())
		hdr = "Your container"
		subh = "Scale Factor:  " + sf
		text = "All quantities will be adjusted to your container size: "
		list = make([]DisplayItem, 8)
		list[0] = DisplayItem{Title: text}
		list[1] = DisplayItem{Title: " "}
		list[2] = DisplayItem{Title: "Type:       " + c.Type_}
		if len(c.UDimension) > 0 {
			list[3] = DisplayItem{Title: "Original container Size: " + c.Dimension + " " + c.Unit}
			list[4] = DisplayItem{Title: "Your container Size: " + c.UDimension + " " + c.Unit}
			list[5] = DisplayItem{Title: " "}
		} else {
			list[3] = DisplayItem{Title: "Size: " + c.UDimension + " " + c.Unit}
			list[4] = DisplayItem{Title: " "}
		}
		odim, err := strconv.Atoi(c.Dimension)
		if err != nil {
			panic(fmt.Errorf("in GenDisplay for container: cannot covert container dimension to int [%s]", err.Error()))
		}
		suggdim := strconv.Itoa(odim - 3)
		if suggdim == c.UDimension {
			suggdim = strconv.Itoa(odim - 2)
		}
		if len(c.UDimension) > 0 {
			list[6] = DisplayItem{Title: `To change your container "size [integer]" e.g. "size ` + suggdim + `"`}
		} else {
			list[6] = DisplayItem{Title: `What is the size of your container? Say "size [integer]" e.g. "size ` + suggdim + `"`}
		}
		list[7] = DisplayItem{Title: `Note: the size must be an whole number and be less than the original container size`}

	} else {
		// info screen where user has NOT specified resize yet. Can resize if necessary
		sf = strconv.FormatFloat(global.GetScale(), 'g', 2, 64)
		hdr = "Specify the size of your container"
		subh = "Scale Factor:  " + sf
		text = "Quantities are based on the following container specification: "
		list = make([]DisplayItem, 7)
		list[0] = DisplayItem{Title: text}
		list[1] = DisplayItem{Title: " "}
		list[2] = DisplayItem{Title: "Type:       " + c.Type_}
		list[3] = DisplayItem{Title: "Container Size: " + c.Dimension + " " + c.Unit}
		list[4] = DisplayItem{Title: " "}
		odim, err := strconv.Atoi(c.Dimension)
		if err != nil {
			panic(fmt.Errorf("in GenDisplay for container: cannot covert container dimension to int [%s]", err.Error()))
		}
		suggdim := strconv.Itoa(odim - 3)
		list[5] = DisplayItem{Title: `What is the size of your container? Say 'size [newsize]' e.g. "size ` + suggdim + `"`}
		list[6] = DisplayItem{Title: `Note: your container size must be less than the recipe container size displayed above`}
		if s.request != "start" {
			s.pushState()
		}
	}
	// create new state - must do this so when back button is hit we can pop this state otherwise we loose objMenu state
	//  alternatively we cou ld updateState to indicate this state - but that invovles a write so we may as well create a new state.
	type_ := "Ingredient"
	if len(s.derr) > 0 {
		type_ += "Err"
	}
	backBtn := true
	if len(s.state) < 2 {
		backBtn = false
	}
	if len(s.parts) > 0 {
		hint = `hint: "size [integer]", "size clear", list ingredients", "list parts", "list instructions", "back", "restart"`
	} else {
		hint = `hint: "size [integer]", "size clear", list ingredients", "list instructions", "back", "restart"`
	}
	return RespEvent{Type: type_, BackBtn: backBtn, Header: hdr, SubHdr: subh, Hint: hint, List: list, Error: s.derr}
}
