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
	Id           int          `json:"id"` // active record in Instructions starting at 1
	Thread       int          `json:"thread"`
	EOL          int          `json:"eol"` // total instructions across all threads
}

type Threads []ThreadT

type ObjMenuT struct {
	id   int
	item string
}

type ObjMenu []ObjMenuT

var objMenu ObjMenu = []ObjMenuT{
	ObjMenuT{0, "Ingredients"},
	ObjMenuT{1, "Containers and utensils"},
	ObjMenuT{2, "Modify container size"},
	ObjMenuT{3, `Start cooking...`},
}

// instance of below type saved to state data in dynamo
type DispContainerT struct {
	Type_      string `json:"Type"`
	Shape      string
	Dimension  string `json:"dim"`  // recipe container size
	UDimension string `json:"udim"` // user defined container size
	Unit       string
}

type WelcomeT string

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
	BackBtn  bool          `json:"BackBtn"`
	Type     string        `json:"Type"`
	Header   string        `json:"Header"`
	SubHdr   string        `json:"SubHdr"`
	Text     string        `json:"Text"`
	Verbal   string        `json:"Verbal"`
	Height   int           `json:"Height"`
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
		p       []string
		//eol     int
		peol int
		part string
		//pid     string
		hdr  string
		subh string
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
			id = len(t[s.cThread].Instructions)
			s.recId[objectMap[s.object]] = id
		}
	}
	//
	t[s.cThread].Id = id
	//rec := &t[s.cThread].Instructions[id-1]
	fmt.Println("cThread, id = ", s.cThread, id)
	//eol := t[s.cThread].EOL
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
	if len(s.part) > 0 {
		if s.part == CompleteRecipe_ {
			p = strings.Split(s.part, "|")
			peol = len(t[s.cThread].Instructions)
			hdr += "  -  " + strings.ToUpper(p[0])
			subh = "Cooking Instructions  " + "  -  " + strconv.Itoa(id) + " of " + strconv.Itoa(peol)
		} else {
			p = strings.Split(s.part, "|")
			hdr += "  -  " + strings.ToUpper(p[0])
			peol := len(t[s.cThread].Instructions)
			subh = "Cooking Instructions " + "  -  " + strconv.Itoa(id) + " of " + strconv.Itoa(peol)
		}
	} else {
		// p = getLongName(rec.part)
		// p = strings.Split(s.part, "|")
		// hdr += "  -  " + strings.ToUpper(p[0])
		eol := len(t[s.cThread].Instructions)
		subh = "Cooking Instructions  -  " + strconv.Itoa(id) + " of " + strconv.Itoa(eol)
	}
	fmt.Println("switch on thread: ", t[s.cThread].Thread)
	//
	// local funcs
	//
	SectA := func(thread int) []DisplayItem {
		var rows int

		list := make([]DisplayItem, 3)
		for k, n, ir := 2, t[thread].Id-1, t[thread].Instructions; n > 0 && rows < 3; rows++ {
			list[k] = DisplayItem{Title: ir[n-1].Text}
			n--
			k--
		}

		return list
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
		// eol = strconv.Itoa(rec.EOL)
		// peol = strconv.Itoa(rec.PEOL)
		part = getLongName(rec.Part)
		// pid = strconv.Itoa(rec.PID)
		//s.eol, s.peol, s.part, s.pid = rec.EOL, rec.PEOL, part, rec.PID
		s.part = part
		type_ := "Tripple"
		// if len(t[s.cThread].Instructions[id-1].Text) > 80 {
		// 	type_ += "L" // larger text bounding box
		// }
		speak := "<speak>" + rec.Verbal + "</speak>"

		s.menuL = nil
		err := s.updateState()
		if err != nil {
			return RespEvent{Text: s.vmsg, Verbal: s.dmsg, Error: err.Error()}
		}
		return RespEvent{Type: type_, BackBtn: true, Header: hdr, SubHdr: subh, Text: rec.Text, Verbal: speak, ListA: listA, ListB: listB, ListC: listC}

	default:
		// two threads with 3 sections in each. should always display threads 1 and 2 in that order, never thread 0
		threadName := func(thread int) string {
			for i := 0; i < len(s.parts); i++ {
				v := &s.parts[i]
				if v.Type_ == "Thrd" && v.Index == strconv.Itoa(thread) {
					return v.Title
				}
			}
			return "no-thread-foundik8,"
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
		// eol = strconv.Itoa(rec.EOL)
		// peol = strconv.Itoa(rec.PEOL)
		part = getLongName(rec.Part)
		// pid = strconv.Itoa(rec.PID)
		//s.eol, s.peol, s.part, s.pid = rec.EOL, rec.PEOL, part, rec.PID
		s.part = part
		fmt.Println("4 cThread, id = ", s.cThread, id)

		// if len(rec.Text) > 80 {
		// 	type_ += "L" // TODO: create ThreadedL script
		// }
		speak := "<speak>" + rec.Verbal + "</speak>"

		s.menuL = nil
		err := s.updateState()
		if err != nil {
			return RespEvent{Text: s.vmsg, Verbal: s.dmsg, Error: err.Error()}
		}

		return RespEvent{Type: type_, BackBtn: true, Header: hdr, SubHdr: subh, Text: rec.Text, Verbal: speak, ListA: listA, ListB: listB, ListC: listC,
			ListD: listD, ListE: listE, ListF: listF, Color1: color1, Color2: color2, Thread1: trName1, Thread2: trName2,
		}
	}
}

func (m WelcomeT) GenDisplay(s *sessCtx) RespEvent {

	fmt.Printf("in GenDisplay for Welcome message")
	hdr := "Welcome. Please search for a recipe"
	subh := "example search chocolate cake, search tarragon, search pastry, search strawberry tart"

	list := make([]DisplayItem, 1)
	list[0] = DisplayItem{Title: string(m)}

	type_ := "Ingredient"
	return RespEvent{Type: type_, BackBtn: true, Header: hdr, SubHdr: subh, List: list}
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
	// parts sourced from recipe.parts (json list loaded into go slice)
	var (
		hdr  string
		subh string
		k    int
	)
	if len(s.passErr) > 0 {
		hdr = s.passErr
	} else {
		hdr = s.reqRName
		subh = `Recipe is divided into parts. Select first option to follow complete recipe`
	}
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
	return RespEvent{Type: "PartList", BackBtn: true, Header: hdr, SubHdr: subh, Height: 90, Text: s.vmsg, Verbal: s.dmsg, List: list}

}

func (i IngredientT) GenDisplay(s *sessCtx) RespEvent {

	var (
		list   []DisplayItem
		subhdr string
	)
	fmt.Println("HEre in GenDisplay for ingredients. ScaleF = ", scaleF)
	for _, v := range strings.Split(string(i), "\n") {
		item := DisplayItem{Title: v}
		list = append(list, item)
	}
	if scaleF <= scaleThreshold {
		sf := strconv.FormatFloat(scaleF, 'g', 2, 64)
		subhdr = "Ingredients       (Scale Factor: " + sf + " )"
	} else {
		subhdr = "Ingredients       (Scale Factor: 1.0 )"
	}

	return RespEvent{Type: "Ingredient", BackBtn: true, Header: s.reqRName, SubHdr: subhdr, List: list}

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

func (o ObjMenu) GenDisplay(s *sessCtx) RespEvent {
	var (
		hdr     string
		subh    string
		op      string
		backBtn bool
		noScale bool
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
	//
	// if back button pressed then s.menuL is assigned via state pop(). menuL is empty during normal forward processing.
	//
	var list []DisplayItem

	switch len(s.menuL) {
	case 0:
		ct, err := s.getScaleContainer()
		if err != nil {
			return RespEvent{Text: s.vmsg, Verbal: s.dmsg, Error: err.Error()}
		}
		size := 4
		if len(ct.Cid) == 0 {
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
		if len(s.menuL) > 0 {
			list = make([]DisplayItem, len(s.menuL))
			for i, v := range s.menuL {
				id := strconv.Itoa(v + 1)
				list[i] = DisplayItem{Id: id, Title: objMenu[v].item}
			}
		}
	}
	var err error

	backBtn = true
	if len(s.state) < 2 {
		backBtn = false
	}
	if !s.back {
		err = s.updateState()
		if err != nil {
			return RespEvent{Text: s.vmsg, Verbal: s.dmsg, Error: err.Error()}
		}
	}
	//return RespEvent{Type: "Select", BackBtn: backBtn, Header: hdr, SubHdr: subh, Text: s.vmsg, Verbal: s.dmsg, List: list}
	return RespEvent{Type: "Search", BackBtn: backBtn, Header: hdr, SubHdr: subh, Text: s.vmsg, Verbal: s.dmsg, List: list}

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

func (c *DispContainerT) GenDisplay(s *sessCtx) RespEvent {
	var (
		sf   string
		hdr  string
		subh string
		text string
		list []DisplayItem
	)
	if c == nil {
		panic("in GenDisplay(): DispContainerT instance is nil ")
	}
	if s.dimension > 0 {
		// response to user size request
		cdim, err := strconv.Atoi(c.Dimension)
		if err != nil {
			panic(err.Error())
		}
		c.UDimension = strconv.Itoa(s.dimension)
		scaleF = float64(s.dimension*s.dimension) / float64(cdim*cdim)
		// persist  new data
		err = s.updateState()
		if err != nil {
			return RespEvent{Text: s.vmsg, Verbal: s.dmsg, Error: err.Error()}
		}
		sf = strconv.FormatFloat(scaleF, 'g', 2, 64)
		hdr = "1 Your container"
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
			list[6] = DisplayItem{Title: `To change your container? Say 'size [newsize]' e.g. size ` + suggdim}
		} else {
			list[6] = DisplayItem{Title: `What is the size of your container? Say 'size [newsize]' e.g. size ` + suggdim}
		}
		list[7] = DisplayItem{Title: `Note: must be less than the recipe container size`}

	} else if len(c.UDimension) > 0 {
		// info screen where user has specified resize. Can resize again.
		sf = strconv.FormatFloat(scaleF, 'g', 2, 64)
		hdr = "2 Your container"
		subh = "Scale Factor:  " + sf
		text = "All quantities will be adjusted to your container size: "
		list = make([]DisplayItem, 8)
		list[0] = DisplayItem{Title: text}
		list[1] = DisplayItem{Title: " "}
		list[2] = DisplayItem{Title: "Type:       " + c.Type_}
		list[3] = DisplayItem{Title: "Original container Size: " + c.Dimension + " " + c.Unit}
		list[4] = DisplayItem{Title: "Your container Size: " + c.UDimension + " " + c.Unit}
		list[5] = DisplayItem{Title: " "}
		// suggested resize
		odim, err := strconv.Atoi(c.Dimension)
		if err != nil {
			panic(fmt.Errorf("in GenDisplay for container: cannot covert container dimension to int [%s]", err.Error()))
		}
		suggdim := strconv.Itoa(odim - 3)
		if suggdim == c.UDimension {
			suggdim = strconv.Itoa(odim - 2)
		}
		list[6] = DisplayItem{Title: `To change your container size, say 'size [newsize]' e.g. size ` + suggdim}
		list[7] = DisplayItem{Title: `Note: must be less than the original recipe container size `}

	} else {
		// info screen where user has NOT specified resize yet. Can resize if necessary
		sf = strconv.FormatFloat(scaleF, 'g', 2, 64)
		hdr = "3 Specify the size of your container"
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
		list[5] = DisplayItem{Title: `What is the size of your container? Say 'size [newsize]' e.g. size ` + suggdim}
		list[6] = DisplayItem{Title: `Note: your container size must be less than the recipe container size displayed above`}
	}
	type_ := "Ingredient"
	backBtn := true
	if len(s.state) < 2 {
		backBtn = false
	}
	return RespEvent{Type: type_, BackBtn: backBtn, Header: hdr, SubHdr: subh, List: list}
}
