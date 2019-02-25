package main

import (
	_ "context"
	_ "encoding/json"
	"fmt"
	_ "os"
	"strconv"
)

type DisplayI interface {
	GenDisplay(id int, s *sessCtx) RespEvent
}

// part of session data that is persisted.
type InstructionT struct {
	Text   string `json:"Txt"` // all Linked preps combined text into this field
	Verbal string `json:"Vbl"`
	Part   string `json: "Pt"` // part index name
	EOL    int    `json:"EOL"` // End-Of-List. Max Id assigned to each record
	PEOL   int    `json:"PEOL"`
	PID    int    `json:"PID"` // id within a part
}

type ContainerT struct {
	Text   string
	Verbal string
}

type InstructionS []InstructionT
type ContainerS []ContainerT

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
