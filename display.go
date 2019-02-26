package main

import (
	_ "context"
	_ "encoding/json"
	"fmt"
	_ "os"
	"strconv"
	"strings"
)

type DisplayI interface {
	GenDisplay(id int, s *sessCtx) RespEvent
}

type RecipeListT []mRecipeT
type IngredientT string

type PartT struct {
	Index string `json:"Idx"`   // short name which is attached to each activity JSON.
	Title string `json:"Title"` // long name which is printed out in the ingredients listing
	Start int    `json:"Start"` // SortK value in T-?-? that has first instruction for the partition
}
type PartS []PartT

// part of session data that is persisted.
type InstructionT struct {
	Text   string `json:"Txt"` // all Linked preps combined text into this field
	Verbal string `json:"Vbl"`
	Part   string `json: "Pt"` // part index name
	EOL    int    `json:"EOL"` // End-Of-List. Max Id assigned to each record
	PEOL   int    `json:"PEOL"`
	PID    int    `json:"PID"` // id within a part
}

type InstructionS []InstructionT
type ContainerS []string

type ObjMenuT []string

var objMenu = ObjMenuT{
	"List ingredients",
	"What containers and utensils do I need?",
	"What prep tasks do I need to do before I can start cooking?",
	`Let's start cooking...`}

// cacheInstructions copies data from T- items in recipe table to session data (instructions) that is preserved

func (i InstructionS) GenDisplay(id int, s *sessCtx) RespEvent {

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
	if len(i) == 0 {
		panic(fmt.Errorf("Error: internal, instructions has not been cached"))
	}
	//
	// check id within limits
	//
	if id < 1 {
		passErr = "Reached first instruction"
		id = 1
		s.recId[objectMap[s.object]] = 1
	}
	if id > len(i)-1 {
		passErr = "Reached last instruction"
		id = len(i) - 1
		s.recId[objectMap[s.object]] = id
	}

	rec := &i[id]

	// session context needs to be updated as these values are used to determine state
	eol = strconv.Itoa(rec.EOL)
	if part != CompleteRecipe_ {
		peol = strconv.Itoa(rec.PEOL)
		part = getLongName(rec.Part)
		pid = strconv.Itoa(rec.PID)
	}
	s.eol, s.peol, s.part, s.pid = rec.EOL, rec.PEOL, part, rec.PID
	//
	// generate display
	//
	if len(passErr) > 0 {
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
	//
	//split instructions across three lists - used by echo Show only
	//
	var listA []DisplayItem
	for k, n, ir := 0, id-3, i; k < 3; k++ {
		if n >= 0 {
			item := DisplayItem{Title: ir[n].Text}
			listA = append(listA, item)
		}
		n++
	}
	if len(listA) == 0 {
		listA = []DisplayItem{DisplayItem{Title: " "}}
	}
	listB := make([]DisplayItem, 1)
	listB[0] = DisplayItem{Title: i[id].Text}
	listC := make([]DisplayItem, len(i)-id)
	for k, n, ir := 0, id+1, i; n < len(ir); n++ {
		listC[k] = DisplayItem{Title: ir[n].Text}
		k++
	}
	type_ := "Tripple"
	if len(i[id].Text) > 120 {
		type_ = "Tripple2" // larger text bounding box
	}
	return RespEvent{Type: type_, Header: hdr, SubHdr: subh, Text: rec.Text, Verbal: rec.Verbal, ListA: listA, ListB: listB, ListC: listC}
}

func (c ContainerS) GenDisplay(id int, s *sessCtx) RespEvent {

	fmt.Printf("in GenDisplay for containers: %#v\n", c)
	hdr := s.reqRName
	subh := "Containers and Utensils"

	var list []DisplayItem
	for _, v := range c {
		di := DisplayItem{Title: v}
		list = append(list, di)
	}
	type_ := "Ingredient"
	return RespEvent{Type: type_, Header: hdr, SubHdr: subh, List: list}
}

func (p PartS) GenDisplay(id int, s *sessCtx) RespEvent {

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
	list := make([]DisplayItem, len(p)+1)
	list[0] = DisplayItem{Id: "1", Title: CompleteRecipe_}
	for i, v := range p {
		id := strconv.Itoa(i + 2)
		list[i+1] = DisplayItem{Id: id, Title: v.Title}
	}
	return RespEvent{Type: "Select", Header: hdr, SubHdr: subh, Text: s.vmsg, Verbal: s.dmsg, List: list}

}

func (i IngredientT) GenDisplay(id int, s *sessCtx) RespEvent {

	var list []DisplayItem
	for _, v := range strings.Split(string(i), "\n") {
		item := DisplayItem{Title: v}
		list = append(list, item)
	}

	return RespEvent{Type: "Ingredient", Header: s.reqRName, SubHdr: "Ingredients", List: list}

}

func (r RecipeListT) GenDisplay(id int, s *sessCtx) RespEvent {
	// display recipes
	var list []DisplayItem
	for _, v := range r {
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
		list = append(list, item)
	}

	return RespEvent{Type: "Search", Header: "Search results: " + s.reqSearch, Text: s.vmsg, Verbal: s.dmsg, List: list}
}

func (o ObjMenuT) GenDisplay(id int, s *sessCtx) RespEvent {
	var (
		hdr  string
		subh string
	)
	if len(s.passErr) > 0 {
		hdr = s.passErr
	} else {
		hdr = s.reqRName
		subh = "Book:  " + s.reqBkName + "  Authors: " + s.authors
	}
	list := make([]DisplayItem, 4)
	//for i, v := range []string{ingredient_, utensil_, container_, task_} {
	for i, v := range o {
		id := strconv.Itoa(i + 1)
		list[i] = DisplayItem{Id: id, Title: v}
	}
	return RespEvent{Type: "Select", Header: hdr, SubHdr: subh, Text: s.vmsg, Verbal: s.dmsg, List: list}
}
