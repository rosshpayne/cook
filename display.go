package main

import (
	_ "context"
	_ "encoding/json"
	"fmt"
	_ "os"
	"strconv"
	"strings"
)

type Displayer interface {
	GenDisplay(s *sessCtx) RespEvent
}

type RecipeListT []mRecipeT
type IngredientT string

type PartT struct {
	Index string `json:"Idx"`   // short name which is attached to each activity JSON.
	Type_ string `json:"Typ"`   // either Part, Division or Thread
	Title string `json:"Title"` // long name which is printed out in the ingredients listing
	Start int    `json:"Start"` // SortK value in T-?-? that has first instruction for the partition
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
	ThrdName  string `json:"ThrdNme"`
	Division  string `json:"Div"`
	EOL       int    `json:"EOL"` // End-Of-List. Max Id assigned to each record
	PEOL      int    `json:"PEOL"`
	PID       int    `json:"PID"` // id within a part
}

type InstructionS []InstructionT

type ThreadT struct {
	Instructions InstructionS `json:"I"`
	Id           int          `json:"id"` // active record in Instructions starting at 1
	Thread       int          `json:"thread"`
	EOL          int          `json:"eol"` // total instructions across all threads
}

type Threads []ThreadT

type ContainerS []string

type ObjMenuT []string

var objMenu = ObjMenuT{
	"Ingredients",
	"Containers and utensils",
	"Adjust Quantities...",
	`Start cooking...`}

type DisplayItem struct {
	Id        string
	Title     string
	SubTitle1 string
	SubTitle2 string
	Text      string
}
type RespEvent struct {
	Position string        `json:"Position"` // recId|EOL|PEOL|PName
	BackBtn  bool          `json:"BackBtn"`
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
	ListD    []DisplayItem `json:"ListD"`
	ListE    []DisplayItem `json:"ListE"`
	ListF    []DisplayItem `json:"ListF"`
	Color1   string        `json:"Color1"`
	Color2   string        `json:"Color2"`
	Thread1  string        `json:"Thread1"`
	Thread2  string        `json:"Thread2"`
}

// cacheInstructions copies data from T- items in recipe table to session data (instructions) that is preserved

func (t Threads) GenDisplay(s *sessCtx) RespEvent {

	var (
		passErr string
		eol     string
		peol    string
		part    string
		pid     string
		hdr     string
		subh    string
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
	if len(t) == 0 {
		s.err = fmt.Errorf("Error: internal, instructions has not been cached")
	}
	//
	// check id within limits
	//
	//var id = t[s.cThread].id // cThread either zero (for no thread), 1 for one thread or 2 for two threads. Number of actual threads is len(t)
	fmt.Println("cThread = ", s.cThread)
	fmt.Println("object = ", s.object)

	id := s.recId[objectMap[s.object]] // is the next record id to speak

	//
	// check id within instruction set range - don't change to a lower thread here. Must be explicitly done by resume request.
	//
	switch {

	case id < 1 && len(t) == 1:
		passErr = "Reached first instruction in current thread"
		id = 1
		s.recId[objectMap[s.object]] = 1

	case id < 1 && len(t) > 1:
		// multithreade app - entered thread for first time
		id = 1
		s.recId[objectMap[s.object]] = 1

	case id > len(t[s.cThread].Instructions) && len(t) == 1:
		// single threaded recipe
		passErr = "Reached last instruction"
		id = len(t[s.cThread].Instructions)
		s.recId[objectMap[s.object]] = id

	case id > len(t[s.cThread].Instructions) && len(t) > 1 && s.cThread == len(t)-1:
		// last thread in multiple threaded recipe so check if all previous threads completed.
		if t[s.cThread-1].Id == len(t[s.cThread-1].Instructions) {
			// previous thead completed. Have reached end
			passErr = "Recipe completed"
			id = len(t[s.cThread].Instructions)
			s.recId[objectMap[s.object]] = id
		} else {
			// hint at unfinished thread. Must explicit go to it -not here.
			passErr = "You must resume previous thread and complete it"
			id = len(t[s.cThread].Instructions)
			s.recId[objectMap[s.object]] = id
		}

	case id > len(t[s.cThread].Instructions) && len(t) > 1:
		// recipe has forked or resumed thread has completed.
		switch s.oThread {
		case 0:
			// recipe forked
			s.oThread = s.oThread + 1
			s.cThread = s.oThread + 1
			id = 1
		case 1:
			// resumed thread completed
			s.oThread = -1 // resumed thread completed
			s.cThread = s.cThread + 1
			id = t[s.cThread].Id
		}
		s.recId[objectMap[s.object]] = id
		if t[s.cThread].Id == len(t[s.cThread].Instructions) {
			// previous thead completed. Have reached end
			passErr = "Recipe completed"
			id = len(t[s.cThread].Instructions) - 1
			s.recId[objectMap[s.object]] = id
		}
	}
	//
	t[s.cThread].Id = id
	fmt.Println("cThread, id = ", s.cThread, id)
	//
	// generate display
	//
	if len(passErr) > 0 || s.err != nil {
		if s.err != nil {
			hdr = "** Error **   " + s.err.Error()
		} else {
			hdr = "** Alert **   " + passErr
		}
		hdr = "** Alert **   " + passErr
	} else {
		hdr = s.reqRName
	}
	if len(part) > 0 {
		if part == CompleteRecipe_ {
			subh = "Cooking Instructions (Complete Recipe) " + "  -  " + strconv.Itoa(id) + " of " + eol
		} else {
			subh = "Cooking Instructions for " + part + "  -  " + pid + " of " + peol
		}
	} else {
		subh = "Cooking Instructions  -  " + strconv.Itoa(id) + " of " + eol
	}
	fmt.Println("switch on thread: ", t[s.cThread].Thread)
	//
	// local funcs
	//
	SectA := func(thread int) []DisplayItem {
		var list []DisplayItem
		var rows int
		for n, ir := t[thread].Id-1, t[thread].Instructions; n > 0 && rows < 3; rows++ {
			item := DisplayItem{Title: ir[n-1].Text}
			list = append(list, item)
			n--
		}
		if len(list) == 0 {
			list = []DisplayItem{DisplayItem{Title: " "}}
		}
		list2 := make([]DisplayItem, len(list))
		for n, i := 0, len(list)-1; i > -1; i-- {
			list2[n] = list[i]
			n++
		}
		return list2
	}
	SectB := func(thread int) []DisplayItem {
		list := make([]DisplayItem, 1)
		id := t[thread].Id
		if id == 0 {
			// thread not accessed yet - show last intruction in previous thread
			thread--
			id = t[thread].Id
		}
		list[0] = DisplayItem{Title: "<speak>" + t[thread].Instructions[id-1].Text + "</speak>"}
		return list
	}
	SectC := func(thread int, totrows int, terminate bool) []DisplayItem {
		var list []DisplayItem
		var rows int
		for tr := thread; tr < len(t); tr++ {
			var start int
			if tr == thread {
				start = t[tr].Id
			} else {
				start = 0
			}
			for n, ir := start, t[tr].Instructions; n < len(t[tr].Instructions) && rows < totrows; rows++ {
				fmt.Println("n: ", n)
				item := DisplayItem{Title: ir[n].Text}
				list = append(list, item)
				n++
			}
			if terminate {
				break
			}
		}
		if rows == totrows {
			list[rows-1] = DisplayItem{Title: "more.."}
		}

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
		listA := SectA(tc)
		listB := SectB(tc)
		listC := SectC(tc, 6, false)
		//
		rec := &t[s.cThread].Instructions[id-1]
		eol = strconv.Itoa(rec.EOL)
		peol = strconv.Itoa(rec.PEOL)
		part = getLongName(rec.Part)
		pid = strconv.Itoa(rec.PID)
		s.eol, s.peol, s.part, s.pid = rec.EOL, rec.PEOL, part, rec.PID
		type_ := "Tripple"
		if len(t[s.cThread].Instructions[id-1].Text) > 120 {
			type_ += "L" // larger text bounding box
		}
		speak := "<speak>" + rec.Verbal + "</speak>"
		return RespEvent{Type: type_, BackBtn: true, Header: hdr, SubHdr: subh, Text: rec.Text, Verbal: speak, ListA: listA, ListB: listB, ListC: listC}

	default:
		// two threads with 3 sections in each. should always display threads 1 and 2 in that order, never thread 0
		threadName := func(thread int) string {
			for _, v := range s.parts {
				if v.Type_ == "Thrd" && v.Index == strconv.Itoa(thread) {
					return v.Title
				}
			}
			return "no-thread"
		}
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
		listC := SectC(tc, 3, true)
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
		listF := SectC(tc, 12, false)

		fmt.Println("cThread, id = ", s.cThread, id)
		//
		// rec is the Instruction record, read from the Thread which is held in state.
		//
		rec := &t[s.cThread].Instructions[id-1]
		eol = strconv.Itoa(rec.EOL)
		peol = strconv.Itoa(rec.PEOL)
		part = getLongName(rec.Part)
		pid = strconv.Itoa(rec.PID)
		s.eol, s.peol, s.part, s.pid = rec.EOL, rec.PEOL, part, rec.PID
		fmt.Println("4 cThread, id = ", s.cThread, id)

		if len(rec.Text) > 120 {
			type_ += "L" // TODO: create ThreadedL script
		}
		speak := "<speak>" + rec.Verbal + "</speak>"

		return RespEvent{Type: type_, BackBtn: true, Header: hdr, SubHdr: subh, Text: rec.Text, Verbal: speak, ListA: listA, ListB: listB, ListC: listC,
			ListD: listD, ListE: listE, ListF: listF, Color1: color1, Color2: color2, Thread1: trName1, Thread2: trName2,
		}
	}
}

func (c ContainerS) GenDisplay(s *sessCtx) RespEvent {

	fmt.Printf("in GenDisplay for containers: %#v\n", c)
	hdr := s.reqRName
	subh := "Containers and Utensils"

	var list []DisplayItem
	for _, v := range c {
		di := DisplayItem{Title: v}
		list = append(list, di)
	}
	type_ := "Ingredient"
	return RespEvent{Type: type_, BackBtn: true, Header: hdr, SubHdr: subh, List: list}
}

func (p PartS) GenDisplay(s *sessCtx) RespEvent {

	var (
		hdr  string
		subh string
	)
	if len(s.passErr) > 0 {
		hdr = s.passErr
	} else {
		hdr = s.reqRName
		subh = `Recipe is divided into parts. Select first option to follow complete recipe`
	}
	list := make([]DisplayItem, 1)
	list[0] = DisplayItem{Id: "1", Title: CompleteRecipe_}
	for i, v := range p {
		// ignore threads
		if v.Type_ == "Thrd" {
			continue
		}
		id := strconv.Itoa(i + 2)
		list = append(list, DisplayItem{Id: id, Title: v.Title})
	}
	return RespEvent{Type: "Select", BackBtn: true, Header: hdr, SubHdr: subh, Text: s.vmsg, Verbal: s.dmsg, List: list}

}

func (i IngredientT) GenDisplay(s *sessCtx) RespEvent {

	var list []DisplayItem
	for _, v := range strings.Split(string(i), "\n") {
		item := DisplayItem{Title: v}
		list = append(list, item)
	}

	return RespEvent{Type: "Ingredient", BackBtn: true, Header: s.reqRName, SubHdr: "Ingredients", List: list}

}

func (r RecipeListT) GenDisplay(s *sessCtx) RespEvent {
	// display recipes
	var (
		list    []DisplayItem
		op      string
		hdr     string
		subh    string
		type_   string
		backBtn bool
	)
	if len(s.reqOpenBk) > 0 {
		op = "Opened "
	}
	//
	// No recipes found
	//
	if len(r) == 0 {
		// empty list as a result of no-data-found in search
		words := strings.Split(s.reqSearch, " ")
		if len(s.reqOpenBk) > 0 {
			if len(words) == 1 {
				hdr = fmt.Sprintf(`No recipes found in opened book "%s" for keyword: %s`, s.reqBkName, s.reqSearch)
				subh = "Either close the book or try multiple keywords e.g search orange chocolate tart"
			} else {
				hdr = fmt.Sprintf(`No recipes found in opened book "%s" for keywords: %s`, s.reqBkName, s.reqSearch)
				subh = "Either close the book to search the entire libary or try altenative keywords"
			}
		} else {
			words := strings.Split(s.reqSearch, " ")
			if len(words) == 1 {
				hdr = fmt.Sprintf(`No recipes found for keyword: %s`, s.reqSearch)
				subh = "Try multiple keywords e.g search orange chocolate tart"
			} else {
				hdr = fmt.Sprintf(`No recipes found for keywords: %s`, s.reqSearch)
				subh = "Change the order of the keywords or try alternative keywords"
			}
		}
		type_ = "header"
		backBtn = true
		if len(s.state) < 2 {
			backBtn = false
		}
		return RespEvent{Type: type_, BackBtn: backBtn, Header: hdr, SubHdr: subh, Text: s.vmsg, Verbal: s.dmsg, List: nil}
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
	backBtn = true
	if len(s.state) < 2 {
		backBtn = false
	}

	return RespEvent{Type: type_, BackBtn: backBtn, Header: "Search results: " + s.reqSearch, Text: s.vmsg, Verbal: s.dmsg, List: list}
}

func (o ObjMenuT) GenDisplay(s *sessCtx) RespEvent {
	var (
		hdr     string
		subh    string
		op      string
		backBtn bool
	)
	if len(s.passErr) > 0 {
		hdr = s.passErr
	} else {
		if len(s.reqOpenBk) > 0 {
			op = "Opened "
		}
		hdr = s.reqRName
		subh = op + "Book:  " + s.reqBkName + "  Authors: " + s.authors
	}
	list := make([]DisplayItem, 4)
	//for i, v := range []string{ingredient_, utensil_, container_, task_} {
	for i, v := range o {
		id := strconv.Itoa(i + 1)
		list[i] = DisplayItem{Id: id, Title: v}
	}
	backBtn = true
	if len(s.state) < 2 {
		backBtn = false
	}
	return RespEvent{Type: "Select", BackBtn: backBtn, Header: hdr, SubHdr: subh, Text: s.vmsg, Verbal: s.dmsg, List: list}
}

func (b BookT) GenDisplay(s *sessCtx) RespEvent {
	var (
		hdr     string
		subh    string
		type_   string
		backBtn bool
	)

	type_ = "header"
	backBtn = true
	if len(s.state) < 2 {
		backBtn = false
	}
	id := strings.Split(string(b), "|")
	switch len(id) {
	case 1:
		if s.request == "book/close" {
			hdr = s.CloseBkName + " closed."
			subh = "Future searches will be across all books"
		} else {
			type_ = "Select"
			hdr = "Issue with opening book " + s.reqBkName
			subh = s.dmsg
			list := make([]DisplayItem, 2)
			list[0] = DisplayItem{Id: "1", Title: "Yes"}
			list[1] = DisplayItem{Id: "2", Title: "No"}
			return RespEvent{Type: type_, BackBtn: backBtn, Header: hdr, SubHdr: subh, Text: s.vmsg, Verbal: s.dmsg, List: list}
		}
	default:
		// book successfully opened. No errors can occur during book close so b will always be empty.
		_, BkName := id[0], id[1]
		authors := id[2]
		hdr = "Opened book " + BkName + "  by " + authors
		subh = "All searches will be restricted to this book until it is closed"
	}

	return RespEvent{Type: type_, BackBtn: backBtn, Header: hdr, SubHdr: subh, Text: s.vmsg, Verbal: s.dmsg, List: nil}
}
