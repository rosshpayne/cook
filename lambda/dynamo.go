package main

import (
	_ "encoding/json"
	"fmt"
	_ "os"
	"strconv"
	"strings"
	_ "time"

	"github.com/rosshpayne/cook/global"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	_ "github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/dynamodb/expression"

	_ "github.com/aws/aws-lambda-go/lambdacontext"
)

//

// recipe lookup
type RnLkup struct {
	BkId   string `json:"PKey"`
	RId    int    `json:"SortK"`
	bkname string
}

type ctRec struct {
	Text   string `json:"txt"`
	Verbal string `json:"vbl"`
	EOL    int
}

// func (ct ctRec) Alexa() dialog {
// 	return dialog{Verbal: ct.Verbal, Display: ct.Text, EOL: ct.EOL}
// }

type taskRecT struct {
	PKey     string `json:"PKey"`  // R-[BkId]
	SortK    int    `json:"SortK"` // monotonically increasing - task at which user is upto in recipe
	AId      int    `json:"AId"`   // Activity Id
	Type     byte   `json:"Type"`
	time     int    // all Linked preps sum time components into this field
	Division string `json:"Div"`     // divide tasks/instructs into divisions, e.g. day-before, on-day
	DivOnly  bool   `json:"divOnly"` // divide tasks/instructs into divisions, e.g. day-before, on-day. Not to be printed for non-Division parts
	//Thread    string  `json:"Thrd"` // instruction thread
	Thread    int    `json:"Thrd"`
	Text      string `json:"Text"` // all Linked preps combined text into this field
	MergeThrd int    `json:"Mthrd"`
	Verbal    string `json:"Verbal"`
	//
	Timer []TimerT `json:"Tmr"`
	//
	EOL int `json:"EOL"` // End-Of-List. Max Id assigned to each record
	// Recipe Part metadata
	PEOL int    `json:"PEOL"` // End-of-List-for-part
	PId  int    `json:"PId"`  // instruction id within a part
	Part string `json:"PT"`   // part index name
	Next int    `json:"nxt"`  // next SortK (recId)
	Prev int    `json:"prv"`  // previous SortK (recId) when in part mode as opposed to full recipe mode
	// not persisted
	taskp *PerformT // used in GenerateTasks and loadBaseRecipe
}

// func (pt taskRecT) Alexa() dialog {
// 	return dialog{Verbal: pt.Verbal, Display: pt.Text, EOL: pt.EOL, PEOL: pt.PEOL, PID: pt.PId, PART: pt.Part}
// }

type prepTaskS []*taskRecT

func (od prepTaskS) Len() int           { return len(od) }
func (od prepTaskS) Less(i, j int) bool { return od[i].time > od[j].time }
func (od prepTaskS) Swap(i, j int)      { od[i], od[j] = od[j], od[i] }

func (a ContainerMap) saveContainerUsage(s *sessCtx) error {
	type ctRow struct {
		PKey  string
		SortK float64
		EOL   int
		Txt   string `json:"txt"`
		Vbl   string `json:"vbl"`
	}
	ctS := a.generateContainerUsage(s.dynamodbSvc)
	//
	var rows int
	eol := len(ctS)
	for i, v := range ctS {
		rows++
		ctd := ctRow{PKey: "C-" + s.pkey, SortK: float64(i + 1), Txt: v, Vbl: v, EOL: eol}
		av, err := dynamodbattribute.MarshalMap(ctd)
		if err != nil {
			return fmt.Errorf("%s: %s", "Error: failed to marshal Record in saveContainerUsage", err.Error())
		}
		_, err = s.dynamodbSvc.PutItem(&dynamodb.PutItemInput{
			TableName: aws.String("Recipe"),
			Item:      av,
		})
		if err != nil {
			return fmt.Errorf("%s: %s", "Error: failed to PutItem in saveContainerUsage", err.Error())
		}
		//time.Sleep(50 * time.Millisecond)
	}

	return nil
}

func (s *sessCtx) loadInstructions() (Threads, error) {
	// based around part display
	// part_ respresents a division of a recipe by ingredients e.g. topping, or a division by instructions e.g. day-before
	pKey := "T-" + s.pkey
	keyC := expression.KeyEqual(expression.Key("PKey"), expression.Value(pKey))
	expr, err := expression.NewBuilder().WithKeyCondition(keyC).Build()
	if err != nil {
		panic(err)
	}
	//
	input := &dynamodb.QueryInput{
		KeyConditionExpression:    expr.KeyCondition(),
		FilterExpression:          expr.Filter(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
	}
	input = input.SetTableName("Recipe").SetReturnConsumedCapacity("TOTAL").SetConsistentRead(false)
	//
	// TODO - should be GetItem not Query as we are providing the primary key however a future feature to display 3 records instead of one would user query.
	result, err := s.dynamodbSvc.Query(input)
	if err != nil {
		//return taskRecT{}, fmt.Errorf("Error in Query of Tasks: " + err.Error())
		return nil, err
	}
	fmt.Println("loadInstructions: Query: Query ConsumedCapacity: \n", result.ConsumedCapacity)
	if int(*result.Count) == 0 { //TODO - put this code back so it makes sense
		// this is caused by a goto operation exceeding EOL
		return nil, fmt.Errorf("Error: %s [%s] ", "Internal error: no instructions found for recipe ", s.reqRName)
	}
	ptR := make([]taskRecT, len(result.Items))
	err = dynamodbattribute.UnmarshalListOfMaps(result.Items, &ptR)
	if err != nil {
		return nil, fmt.Errorf("Error: %s - %s", "in UnmarshalMap in loadInstructions ", err.Error())
	}
	fmt.Printf(" loadInstructions len() %d part [%s]   s.selId = [%d]	\n ", len(ptR), s.part, s.selId)

	var listPart string
	if len(s.parts) == 0 {
		listPart = CompleteRecipe_
	} else {
		listPart = s.part
		if s.part != CompleteRecipe_ {
			listPart = "DivPt"
		}
		listPart = "DivPt"
		if s.selId == 1 {
			// either no parts menu or item 1 is chosen
			listPart = CompleteRecipe_
		}
	}
	fmt.Printf(" loadInstructions len() %d part [%s]   s.selId = [%d]  listPart [%s]	\n ", len(ptR), s.part, s.selId, listPart)
	// part := s.part
	// fmt.Printf(" loadInstructions len() %d .  s.selId = %d	\n ", len(ptR), s.selId)
	// if len(s.part) == 0 {
	// 	part = "DivPt"
	// 	if s.selId == 0 || s.selId == 1 {
	// 		// either no parts menu or item 1 is chosen
	// 		part = CompleteRecipe_
	// 	}
	// } else {
	// 	if s.part != CompleteRecipe_ {
	// 		part = "DivPt"
	// 	}
	// }
	var (
		threads   Threads // []ThreadT
		instructs InstructionS
	)

	switch listPart {
	case CompleteRecipe_:
		// look for multiple threads within CompleteRecipe_
		threads = make(Threads, 1)
		for thread, i := 0, 0; i < len(ptR); i++ {
			if ptR[i].Thread > 0 {
				if ptR[i].Thread > thread {
					thread = ptR[i].Thread
					threads = append(threads, ThreadT{Thread: thread})
				}
			}
		}
		// 0 index for no thead case, index 1,2 for threads 1,2
		for t := 0; t < len(threads); t++ {
			for _, v := range ptR {
				if v.Thread == threads[t].Thread && !v.DivOnly {
					global.Set_WriteCtx(global.USay)
					vmsg := expandLiteralTags(v.Verbal, s)
					global.Set_WriteCtx(global.UDisplay)
					dmsg := expandLiteralTags(v.Text, s)
					instruct := InstructionT{Text: dmsg, Verbal: vmsg, Thread: v.Thread, Division: v.Division, MergeThrd: v.MergeThrd}
					threads[t].Instructions = append(threads[t].Instructions, instruct)
				}
			}
			threads[t].EOL = len(ptR)
		}
	case "DivPt":
		// find instructions associated with the part
		// s.selId is value from part list screen
		v := s.parts[s.selId-2]
		instructs = make(InstructionS, 1)
		instructs[0] = InstructionT{} // blank instruction at index 0, so Instructions start at index 1.

		switch v.Type_ {
		case "Div":
			// division by instruction - loop through all instructions looking for any threads
			// generate threads: look for multiple threads within division/part
			var thrdCnt int
			threads = make(Threads, 1)
			for thread, i := 0, 0; i < len(ptR); i++ {
				if len(ptR[i].Division) > 0 && ptR[i].Division == v.Index {
					if ptR[i].Thread > thread {
						thread = ptR[i].Thread
						thrdCnt++
					}
				}
			}
			// only multiple parallel threads if more than two threads are active otherwise if only two or less, thread 0 and 1 are be performed in series.
			if thrdCnt > 2 {
				for i := 1; i < thrdCnt; i++ {
					threads = append(threads, ThreadT{Thread: i})
				}
			}
			fmt.Println("Threads len(threads) = ", len(threads))
			// generate instructions: within a thread for the division
			// 0 index for no thead case, index 1,2 for threads 1,2
			for t := 0; t < len(threads); t++ {
				for i, _ := range ptR {
					if len(ptR[i].Division) > 0 && ptR[i].Division == v.Index {
						switch len(threads) {
						case 1: // ignore threads if only thread values detected
							global.Set_WriteCtx(global.USay)
							vmsg := expandLiteralTags(ptR[i].Verbal, s)
							global.Set_WriteCtx(global.UDisplay)
							dmsg := expandLiteralTags(ptR[i].Text, s)
							instruct := InstructionT{Text: dmsg, Verbal: vmsg, Thread: ptR[i].Thread, Division: ptR[i].Division}
							threads[t].Instructions = append(threads[t].Instructions, instruct)
						default:
							if ptR[i].Thread == threads[t].Thread {
								global.Set_WriteCtx(global.USay)
								vmsg := expandLiteralTags(ptR[i].Verbal, s)
								global.Set_WriteCtx(global.UDisplay)
								dmsg := expandLiteralTags(ptR[i].Text, s)
								instruct := InstructionT{Text: dmsg, Verbal: vmsg, Thread: ptR[i].Thread, Division: ptR[i].Division}
								threads[t].Instructions = append(threads[t].Instructions, instruct)
							}
						}
					}
				}
			}
			s.cThread = threads[0].Thread

		default:
			// v.type_=Thread
			// division by ingredient (part) - currently uses linked instructions. May go scan like Div in future.
			// generate threads: look for multiple threads within part
			var thrdCnt int
			threads = make(Threads, 1)
			for thread, id := 0, v.Start; id != -1; id = ptR[id-1].Next {
				i := id - 1
				//fmt.Println("id = ", id)
				if ptR[i].Thread > 0 {
					if ptR[i].Thread > thread {
						thread = ptR[i].Thread
						thrdCnt++
					}
				}
			}
			// only multiple parallel threads if more than two threads are active otherwise if only two or less, thread 0 and 1 are be performed in series.
			if thrdCnt > 2 {
				for i := 1; i < thrdCnt; i++ {
					threads = append(threads, ThreadT{Thread: i})
				}
			}
			// generate instructions: within a thread for the part
			// 0 index for no thead case, index 1,2 for threads 1,2
			for t := 0; t < len(threads); t++ {
				for id := v.Start; id != -1; id = ptR[id-1].Next {
					// ptR.Next points to SortK in table - which starts at 1
					i := id - 1
					switch len(threads) {
					case 1: // ignore threads if only two or less thread values detected  (see thrdCnt)
						if !ptR[i].DivOnly && !ptR[i].DivOnly {
							global.Set_WriteCtx(global.USay)
							vmsg := expandLiteralTags(ptR[i].Verbal, s)
							global.Set_WriteCtx(global.UDisplay)
							dmsg := expandLiteralTags(ptR[i].Text, s)
							instruct := InstructionT{Text: dmsg, Verbal: vmsg, Part: ptR[i].Part, EOL: ptR[i].EOL, PEOL: ptR[i].PEOL, PID: ptR[i].PId}
							threads[t].Instructions = append(threads[t].Instructions, instruct)
						}
					default:
						if ptR[i].Thread == threads[t].Thread && !ptR[i].DivOnly {
							global.Set_WriteCtx(global.USay)
							vmsg := expandLiteralTags(ptR[i].Verbal, s)
							global.Set_WriteCtx(global.UDisplay)
							dmsg := expandLiteralTags(ptR[i].Text, s)
							instruct := InstructionT{Text: dmsg, Verbal: vmsg, Part: ptR[i].Part, EOL: ptR[i].EOL, PEOL: ptR[i].PEOL, PID: ptR[i].PId}
							threads[t].Instructions = append(threads[t].Instructions, instruct)
						}
					}
				}
			}
		}
	}
	//s.cThread = threads[0].Thread // index into threads
	//
	return threads, nil
}

type containerT struct {
	Verbal string `json:"vbl"`
	Text   string `json:"txt"`
}

func (s *sessCtx) getScaleContainer() (*Container, error) {

	// return a single scale container (containers that determine quantity of ingredients)
	//  if more than one scale container first one is returned.
	// PKey = C-[BkId]-[RId]
	s.pkey = s.reqBkId + "-" + s.reqRId
	fmt.Println("in getScaleContainer - Pkey: ", s.pkey)
	keyC := expression.KeyEqual(expression.Key("PKey"), expression.Value("C-"+s.pkey))
	fcond := expression.Equal(expression.Name("scale"), expression.Value(true))
	expr, err := expression.NewBuilder().WithKeyCondition(keyC).WithFilter(fcond).Build()
	if err != nil {
		panic(err)
	}
	//
	input := &dynamodb.QueryInput{
		KeyConditionExpression:    expr.KeyCondition(),
		FilterExpression:          expr.Filter(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
	}
	input = input.SetTableName("Ingredient").SetReturnConsumedCapacity("TOTAL").SetConsistentRead(false)
	//
	result, err := s.dynamodbSvc.Query(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case dynamodb.ErrCodeProvisionedThroughputExceededException:
				fmt.Println(dynamodb.ErrCodeProvisionedThroughputExceededException, aerr.Error())
			case dynamodb.ErrCodeResourceNotFoundException:
				fmt.Println(dynamodb.ErrCodeResourceNotFoundException, aerr.Error())
			// case dynamodb.ErrCodeRequestLimitExceeded:
			// 	fmt.Println(dynamodb.ErrCodeRequestLimitExceeded, aerr.Error())
			case dynamodb.ErrCodeInternalServerError:
				fmt.Println(dynamodb.ErrCodeInternalServerError, aerr.Error())
			default:
				fmt.Println(aerr.Error())
			}
			panic(aerr.Error())
		} else {
			// Print the error, cast err to awserr.Error to get the Code and
			// Message from an error.
			panic(err.Error())
		}
		panic(fmt.Errorf("%s: %s", "Error in GetItem of getContainerRecById", err.Error()))
	}
	fmt.Println("getScaleContainer: Query: Query ConsumedCapacity:\n", result.ConsumedCapacity)
	if len(result.Items) == 0 {
		fmt.Println("in getScaleContainer - no records found.")
		return nil, nil
	}
	recS := make([]Container, int(*result.Count))
	err = dynamodbattribute.UnmarshalListOfMaps(result.Items, &recS)
	if err != nil {
		return nil, fmt.Errorf("Error: %s [%s] err", "in UnmarshalListMaps of getScaleContainer ", s.reqRName, err.Error())
	}
	fmt.Printf("in getScaleContainer - recS  %#v\n", recS[0])
	return &recS[0], nil
}

type indexRecipeT struct {
	PKey     string
	SortK    string
	Quantity string
	BkName   string
	RName    string
	Authors  string
	Srv      string
}

func (s *sessCtx) generateAndSaveIndex(labelM map[string]*Activity, ingrdM map[string]*Activity) error {

	var (
		indexRecS []indexRecipeT
		subject   string
		type_     string
	)
	indexRow := make(map[string]bool)
	// any string() methods will be writing for the display
	global.Set_WriteCtx(global.UDisplay)

	saveIngdIndex := func() error {

		for _, v := range indexRecS {
			av, err := dynamodbattribute.MarshalMap(v)
			if err != nil {
				panic(fmt.Sprintf("failed in IndexIngd to marshal Record, %v", err))
			}
			_, err = s.dynamodbSvc.PutItem(&dynamodb.PutItemInput{
				TableName: aws.String("Ingredient"),
				Item:      av,
			})
			if err != nil {
				return fmt.Errorf("failed in IndexIngd to PutItem into Ingredient table - %v", err)
			}
		}
		s.indexRecs = indexRecS // free memory. Probably redundant as its local to this func so once func exists memory would be freed anyway.
		return nil
	}

	indexBasicEntry := func(entry string) {
		//
		if indexRow[entry] {
			return
		}
		irec := indexRecipeT{SortK: s.reqBkId + "-" + s.reqRId, BkName: s.reqBkName, RName: s.reqRName, Authors: s.authors, Srv: s.recipe.Serves}
		irec.PKey = strings.Replace(entry, "-", " ", -1)
		// only append unique values..
		indexRow[entry] = true
		indexRecS = append(indexRecS, irec)
	}

	makeIndexRecs := func(entry string, ap *Activity) {
		// for each index property value, add to index
		if indexRow[entry] {
			return
		}
		irec := indexRecipeT{SortK: s.reqBkId + "-" + s.reqRId, BkName: s.reqBkName, RName: s.reqRName, Authors: s.authors, Srv: s.recipe.Serves}
		irec.PKey = strings.Replace(entry, "-", " ", -1)
		irec.Quantity = ap.String()
		// only append unique values..
		indexRow[entry] = true
		indexRecS = append(indexRecS, irec)
	}

	AddEntry := func(entry string) {
		// entry with hyphon is treated as one word
		entry = strings.Replace(entry, "-", " ", -1)
		entry = strings.Replace(entry, "  ", " ", -1)
		// remove hyphon when saving as index entry though
		entry = strings.TrimRight(strings.TrimLeft(strings.ToLower(entry), " "), " ")
		// for each word in index entry find associated activity via its label or ingredient
		//  if not found then create a basic index entry (withou ingredient details)
		var indexed bool
		w := entry
		if a, ok := labelM[strings.ToLower(w)]; ok {
			makeIndexRecs(entry, a)
			indexed = true
		}
		if !indexed {
			if a, ok := ingrdM[strings.ToLower(w)]; ok {
				makeIndexRecs(entry, a)
				indexed = true
			}
		}
		// if not indexed, try searching using individual words in entry
		//  - but index using whole entry not the word.
		if !indexed {
			for _, w := range strings.Fields(entry) {
				if a, ok := labelM[strings.ToLower(w)]; ok {
					makeIndexRecs(entry, a)
					break
				}
				if a, ok := ingrdM[strings.ToLower(w)]; ok {
					makeIndexRecs(entry, a)
					break
				}
			}
		}
		if !indexRow[entry] {
			indexBasicEntry(entry)
		}
	}

	GenerateEntries := func(s string) {
		// generate phases based on combination of words from recipe

		AddEntry(subject)
		if len(type_) > 0 {
			AddEntry(type_)
			AddEntry(type_ + " " + subject)
		}

		e := strings.Fields(s)
		for _, v := range e {
			AddEntry(v)
			if len(type_) > 0 {
				AddEntry(v + " " + type_)
				AddEntry(v + " " + subject)
				AddEntry(v + " " + type_ + " " + subject)
			} else {
				AddEntry(v + " " + subject)
			}

		}
		switch len(e) {
		case 0, 1:
		case 2:
			AddEntry(e[0] + " " + e[1])
			AddEntry(e[0] + " " + e[1] + " " + subject)
			//AddEntry(e[1] + " " + e[0] + " " + subject)
			AddEntry(e[0] + " " + e[1] + " " + type_ + " " + subject)
			//AddEntry(e[1] + " " + e[0] + " " + type_ + " " + subject)
		default:
			AddEntry(e[0] + " " + e[1])
			AddEntry(e[0] + " " + e[1] + " " + subject)
			AddEntry(e[0] + " " + e[2] + " " + subject)
			AddEntry(e[1] + " " + e[2] + " " + subject)
		}
	}
	removePunc := []string{",", ";", "!", "@", "&", "(", ")", "{", "}"}
	removeWords := []string{"the", "and", "of", "with", "fresh", "a", "to", "from", "by"} //TODO: source from dynamo

	RemovePuncs := func(entry string) string {
		a := strings.TrimRight(strings.TrimLeft(strings.ToLower(entry), " "), " ")
		for _, v := range removePunc {
			a = strings.Replace(a, v, " ", -1)
		}
		a = strings.Replace(a, "  ", " ", -1) // so strings.Split works properly
		return a
	}

	RemoveCommonWords := func(entry string) string {
		a := strings.TrimRight(strings.TrimLeft(entry, " "), " ")
		for _, v := range removeWords {
			a = strings.Replace(a+" ", " "+v+" ", " ", -1)
		}
		return a
	}

	var mword []string = []string{
		"rose water",
		"ice cream",
		"tin can",
		"olive oil",
		"source cream",
		"soured cream",
		"rum and raisin",
		"rum caramel",
		"tropical fruit",
		"baileys irish cream",
		"custard cream",
		"? topping",
		"star anise",
		"white chocolate",
		"dark chocolate",
		"baby black",
		"raw tomato",
	}

	joinWords := func(s string) string {
		for _, v := range mword {
			if i := strings.Index(v, "?"); i > -1 {
				v := v[i+2:] // substring - part 2 of join words
				//
				for i, w := range strings.Fields(s) {
					if w == v {
						if i < 2 {
							break
						}
						// get all words in recipe
						allw := strings.Fields(s)
						var b strings.Builder
						// now join
						for _, v := range allw[:i-1] {
							b.WriteString(v + " ")
						}
						fmt.Println("Pre: ", b.String())
						for _, v := range allw[i-1 : i] {
							b.WriteString(v + "-")
						}
						for _, v := range allw[i:] {
							b.WriteString(v + " ")
						}
						s = b.String()
					}
					continue
				}
			}
			if i := strings.Index(s, v); i > -1 {
				hv := strings.Replace(v, " ", "-", -1)
				s = strings.Replace(s, v, hv, -1)
			}
		}
		return s
	}
	//*****************************************************************************//
	//
	// index using recipe name
	//
	AddEntry(RemovePuncs(s.recipe.RName))
	//
	// check reference to "with"
	//
	var recipeIndexed bool
	//
	// recipes seem to follow this format:  [a [[,b] and] ] <type> subject [with c [and d]]
	//
	words := strings.Fields(joinWords(strings.ToLower(s.recipe.RName)))
	//
	// look for "with" in recipe name
	//
	for i, v := range words {
		if v == "with" {
			var s strings.Builder
			subject = words[i-1]
			if i > 1 {
				type_ = words[i-2]
			}
			if i > 3 {
				for _, word := range words[:i-3] {
					w := strings.Split(word, ",")
					s.WriteString(w[0] + " ")
				}
			}
			for _, word := range words[i+1:] {
				w := strings.Split(word, ",")
				s.WriteString(w[0] + " ")
			}
			str := RemoveCommonWords(RemovePuncs(s.String()))
			fmt.Println("** with .. ", str)
			words := strings.Fields(str)
			GenerateEntries(str)
			if len(words) > 2 {
				GenerateEntries(words[1] + " " + words[2])
			}

			recipeIndexed = true
		}
	}
	//
	// if no "with", recipe name format becomes:  [a [[,b] and] ] <type> subject
	//
	if !recipeIndexed {
		if len(words) > 1 {
			type_ = words[len(words)-2]
		}
		subject = words[len(words)-1]

		var s strings.Builder
		for _, word := range words[:len(words)-2] {
			w := strings.Split(word, ",")
			s.WriteString(w[0] + " ")
		}
		pre := RemoveCommonWords(RemovePuncs(s.String()))

		GenerateEntries(pre)
	}
	//
	// index using index attribute from recipe item //TODO is this necessary as index make not be useful.
	//
	index := s.recipe.Index
	for _, entry := range index {
		AddEntry(entry)
		e := strings.Fields(entry)
		for _, v := range e {
			AddEntry(v)
		}
		switch len(e) {
		case 3:
			AddEntry(e[0] + " " + e[1])
			AddEntry(e[0] + " " + e[2])
			AddEntry(e[1] + " " + e[2])
		}
	}
	//s.indexRecs = indexRecS

	err := saveIngdIndex()
	if err != nil {
		return fmt.Errorf("Error in generateAndSaveIndex at  generateSlotEntries - %s", err.Error())
	}
	//
	// must refresh Alexa slot entries on each new index build
	//
	//err = s.generateSlotEntries() // nolonger use slots to handle recipe indexing.
	return err

}

func (d DevicesMap) saveDevices(s *sessCtx) error {
	var row int
	//
	type Pkey struct {
		PKey    string `json:"PKey"`
		SortK   int    `json:"SortK"`
		Device  string `json:"Device"`
		Comment string `json:"Comment"`
	}
	for k, v := range d {
		r := &Pkey{PKey: "D-" + s.pkey, SortK: row, Device: k, Comment: v}
		row++
		av, err := dynamodbattribute.MarshalMap(r)
		if err != nil {
			return fmt.Errorf("Error in saveDevices, MarshalMap - %s", err.Error())
		}
		_, err = s.dynamodbSvc.PutItem(&dynamodb.PutItemInput{
			TableName: aws.String("Recipe"),
			Item:      av,
		})
		if err != nil {
			return fmt.Errorf("Error in saveDevices, failed to put Record to DynamoDB - %s", err.Error())
		}
		//time.Sleep(50 * time.Millisecond)
	}
	return nil
}

func (a Activities) generateAndSaveTasks(s *sessCtx) (prepTaskS, error) {
	var rows int
	// only prep & task verbal and its text equivalent are saved.
	// Generate prep and tasks from Activities.
	ptS := a.GenerateTasks("T-"+s.pkey, s.recipe, s)
	//
	// Fast bulk load is not a priority - trickle insert will suffice atleast for the moment.
	//
	for _, v := range ptS {
		rows++
		av, err := dynamodbattribute.MarshalMap(v)
		if err != nil {
			panic(fmt.Sprintf("failed to DynamoDB marshal Record, %v", err))
		}
		_, err = s.dynamodbSvc.PutItem(&dynamodb.PutItemInput{
			TableName: aws.String("Recipe"),
			Item:      av,
		})
		if err != nil {
			return prepTaskS{}, fmt.Errorf("failed to put Record to DynamoDB, %v", err)
		}
	}
	return ptS, nil
}

func (s *sessCtx) updateRecipe(r *RecipeT) error {
	//
	type pKey struct {
		PKey  string
		SortK float64
	}
	//var updateC expression.UpdateBuilder

	rId, err := strconv.Atoi(s.reqRId)
	if err != nil {
		return fmt.Errorf("Error: in updateRecipe converting reqRId  [%s] to int - %s", s.reqRId, err.Error())
	}
	pkey := pKey{PKey: "R-" + s.reqBkId, SortK: float64(rId)}
	av, err := dynamodbattribute.MarshalMap(&pkey)
	if err != nil {
		return fmt.Errorf("%s: %s", "Error in MarshalMap of recipeIdLookup", err.Error())
	}
	// update Part attribute
	updateC := expression.Set(expression.Name("Part"), expression.Value(r.Part))
	expr, err := expression.NewBuilder().WithUpdate(updateC).Build()
	if err != nil {
		//return taskRecT{}, fmt.Errorf("Error in Query of Tasks: " + err.Error())
		panic(err)
	}
	input := &dynamodb.UpdateItemInput{
		TableName:                 aws.String("Recipe"),
		Key:                       av, // accepts []map[]*attributeValues not string so must use marshal rather than expression
		UpdateExpression:          expr.Update(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		ReturnValues:              aws.String("UPDATED_NEW"),
	}
	_, err = s.dynamodbSvc.UpdateItem(input) // do an updateitem and return original id value so only one call.
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case dynamodb.ErrCodeProvisionedThroughputExceededException:
				fmt.Println(dynamodb.ErrCodeProvisionedThroughputExceededException, aerr.Error())
			case dynamodb.ErrCodeResourceNotFoundException:
				fmt.Println(dynamodb.ErrCodeResourceNotFoundException, aerr.Error())
			case dynamodb.ErrCodeInternalServerError:
				fmt.Println(dynamodb.ErrCodeInternalServerError, aerr.Error())
			default:
				fmt.Println(aerr.Error())
			}
		} else {
			// Print the error, cast err to awserr.Error to get the Code and
			// Message from an error.
			fmt.Println(err.Error())
		}
		return err
	}

	return nil
}

var recipeParts []string

func (s *sessCtx) recipeRSearch() (*RecipeT, error) {
	//
	// query on recipe name to get RecipeId and  book name
	//
	type pKey struct {
		PKey  string
		SortK float64
	}

	errmsg := " in recipeRSearch():"
	fmt.Println("***** ENTERED recipeRSearch *********")
	rId, err := strconv.Atoi(s.reqRId)
	if err != nil {
		return nil, fmt.Errorf("%s. Converting reqRId  [%s] to int - %s", errmsg, s.reqRId, err.Error())
	}
	pkey := pKey{PKey: "R-" + s.reqBkId, SortK: float64(rId)}
	av, err := dynamodbattribute.MarshalMap(&pkey)
	if err != nil {
		return nil, fmt.Errorf("%s. MarshalMap: %s", errmsg, err.Error())
	}
	input := &dynamodb.GetItemInput{
		Key:       av,
		TableName: aws.String("Recipe"),
	}
	input = input.SetTableName("Recipe").SetReturnConsumedCapacity("TOTAL").SetConsistentRead(false)
	//
	result, err := s.dynamodbSvc.GetItem(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case dynamodb.ErrCodeProvisionedThroughputExceededException:
				fmt.Println(dynamodb.ErrCodeProvisionedThroughputExceededException, aerr.Error())
			case dynamodb.ErrCodeResourceNotFoundException:
				fmt.Println(dynamodb.ErrCodeResourceNotFoundException, aerr.Error())
			//case dynamodb.ErrCodeRequestLimitExceeded:
			//	fmt.Println(dynamodb.ErrCodeRequestLimitExceeded, aerr.Error())
			case dynamodb.ErrCodeInternalServerError:
				fmt.Println(dynamodb.ErrCodeInternalServerError, aerr.Error())
			default:
				fmt.Println(aerr.Error())
			}
		} else {
			// Print the error, cast err to awserr.Error to get the Code and
			// Message from an error.
			fmt.Println(err.Error())
		}
		return nil, fmt.Errorf("%s %s: %s", errmsg, "GetItem", errmsg, err.Error())
	}
	fmt.Println("recipeRSearch: GetItem: Query ConsumedCapacity: \n", result.ConsumedCapacity)
	if len(result.Item) == 0 {
		return nil, fmt.Errorf("%s. No Recipe record found for R-%s-%s", errmsg, s.reqBkId, s.reqRId)
	}
	rec := &RecipeT{}
	err = dynamodbattribute.UnmarshalMap(result.Item, rec)
	if err != nil {
		return nil, fmt.Errorf("%s. UnmarshalMaps:  %s", errmsg, s.reqRId, err.Error())
	}
	// populate session context fields
	s.reqRName = rec.RName
	//s.index = rec.Index //TODO: is this required?
	if len(s.authors) == 0 {
		fmt.Println()
		err = s.bookNameLookup()
		if err != nil {
			s.reqBkName = ""
			return nil, err
		}
	}
	s.dmsg = s.reqRName + " in " + s.reqBkName + " by " + s.authors
	s.vmsg = s.reqRName
	s.recipe = rec
	//fmt.Printf("assign Recipe Parts: %d, %#v\n\n", len(rec.Part), rec.Part)
	s.parts = rec.Part
	// add division if any to parts
	for _, v := range s.parts {
		v.Type_ = "Pt"
	}
	for _, v := range rec.Division {
		s.parts = append(s.parts, PartT{Title: v.Title, Type_: "Div", Index: v.Index})
	}
	for _, v := range rec.Thread {
		s.parts = append(s.parts, PartT{Title: v.Title, Type_: "Thrd", Index: v.Index})
	}
	s.rsearch = true
	fmt.Println("Exit recipeRSearch ")
	return rec, nil
}

func (s *sessCtx) keywordSearch(srch string) error {
	//
	// search for recipe by specifying ingredient and a category or sub-category.
	// data must exist in this table for each recipe. Data is populated as part of the base activity processig.
	//
	type searchRecT struct {
		PKey     string
		SortK    string `json:"SortK"`
		RName    string `json:"RName"`
		BkName   string `json:"BkName"`
		Authors  string `json:"Authors"`
		Quantity string `json:"Quantity"`
		Serves   string `json:"Srv"`
	}

	var (
		recS   []searchRecT
		result *dynamodb.QueryOutput
	)
	// zero recipeList list
	//
	fmt.Printf("entered keywordSearch [%s]\n", srch)
	if len(s.reqOpenBk) > 0 {
		// look for recipes in current book only
		kcond := expression.KeyEqual(expression.Key("PKey"), expression.Value(srch))
		kcond = kcond.And(expression.KeyBeginsWith(expression.Key("SortK"), s.reqBkId+"-"))
		expr, err := expression.NewBuilder().WithKeyCondition(kcond).Build()
		if err != nil {
			return fmt.Errorf("Error: %s [%s] %s", "in NewBuilder in keywordSearch of ", s.reqSearch, err.Error())
		}
		input := &dynamodb.QueryInput{
			KeyConditionExpression:    expr.KeyCondition(),
			FilterExpression:          expr.Filter(),
			ExpressionAttributeNames:  expr.Names(),
			ExpressionAttributeValues: expr.Values(),
		}
		input = input.SetTableName("Ingredient").SetReturnConsumedCapacity("TOTAL").SetConsistentRead(false)
		//
		result, err = s.dynamodbSvc.Query(input)
		if err != nil {
			return fmt.Errorf("Error: %s [%s] %s", "in Query in keywordSearch of ", s.reqSearch, err.Error())
		}
		fmt.Println("keywordSearch: Query ConsumedCapacity: \n", result.ConsumedCapacity)
		recS = make([]searchRecT, int(*result.Count))
		err = dynamodbattribute.UnmarshalListOfMaps(result.Items, &recS)
		if err != nil {
			return fmt.Errorf("Error: %s [%s] err", "in UnmarshalListMaps of keywordSearch ", s.reqRName, err.Error())
		}
	} else {
		//
		// allBooks
		//
		//
		// loop through registered books index by userid.  Need to associate userId with email - in external system.
		// 1. external system contains: email as registered by user and book names.
		// 2. on first use of App by user, the App passes [email,userid] to external system  - using index.js to source email & userid. (see github example)
		// 3. this triggers external to pass [userid,[]bkidExt] via Gateway API or insert directly to dynamo
		// 4. Every 5 mins external system loads newly registered books into dynamo passing [userId,bkidExt]. Use Gateway API (http request) or load directly using dynamo call.
		//		4a. check is book is registered, if not inserts [bookName, Authors].  Gets back [bkid].
		//		4b. pass [bkid, BookName], back to external system. External system has following Ids:  [bkid, userId] which maybe stored in Dynamo or external system db.
		//		4c. for each userid inserts [userId,bkid] into dynamo.
		// 5. on each recipe search, App consults dynamo to see what [bkid] are registered to [userId]
		//
		// for each book id for userId loop..(table: Ingredient: PKey: uId, SortK: BkId)
		bkids := s.state[0].Welcome.Bkids
		//
		kcond := expression.KeyEqual(expression.Key("PKey"), expression.Value(srch))
		expr, err := expression.NewBuilder().WithKeyCondition(kcond).Build()
		if err != nil {
			return fmt.Errorf("Error: %s [%s] %s", "in NewBuilder in keywordSearch of ", s.reqSearch, err.Error())
		}
		input := &dynamodb.QueryInput{
			KeyConditionExpression:    expr.KeyCondition(),
			FilterExpression:          expr.Filter(),
			ExpressionAttributeNames:  expr.Names(),
			ExpressionAttributeValues: expr.Values(),
		}
		input = input.SetTableName("Ingredient").SetReturnConsumedCapacity("TOTAL").SetConsistentRead(false)
		//
		result, err = s.dynamodbSvc.Query(input)
		if err != nil {
			return fmt.Errorf("Error: %s [%s] %s", "in Query in keywordSearch of ", s.reqBkId, err.Error())
		}
		fmt.Println("keywordSearch: Query ConsumedCapacity: \n", result.ConsumedCapacity)
		if int(*(result.Count)) > 0 {
			// found one or more recipes.
			recS_ := make([]searchRecT, int(*result.Count))
			err = dynamodbattribute.UnmarshalListOfMaps(result.Items, &recS_)
			if err != nil {
				return fmt.Errorf("Error: %s [%s] err", "in UnmarshalListMaps of keywordSearch ", s.reqRName, err.Error())
			}
			// Now check if recipes are in the list of registered books
			for _, v := range bkids {
				for _, r := range recS_ {
					if v == r.SortK[:strings.IndexByte(r.SortK, '-')] {
						recS = append(recS, r)
					}
				}
			}
		}
	}
	//
	// append results to recipe list
	//
	for i, v := range recS {
		sortk := strings.Split(v.SortK, "-")
		s.ddata += strconv.Itoa(i+1) + ": " + v.BkName + " by " + v.Authors + ". Quantity: " + v.Quantity + "\n"
		rec := mRecipeT{Id: i + 1, RName: v.RName, RId: sortk[1], BkName: v.BkName, BkId: sortk[0], Authors: v.Authors, Quantity: v.Quantity, Serves: v.Serves}
		s.recipeList = append(s.recipeList, rec)
	}
	return nil
}

func (s *sessCtx) recipeNameSearch() error {
	//
	// user "opens <book>". Alexa provides associated slot-type-id BkId value.
	//
	// used in recipe name
	type RecipeT_ struct {
		PKey  string
		SortK int
		RName string
	}
	var (
		expr expression.Expression
		err  error
	)

	kcond := expression.KeyEqual(expression.Key("RName"), expression.Value(s.reqRName))
	if len(s.reqBkId) > 0 {
		filter := expression.Equal(expression.Name("PKey"), expression.Value("R-"+s.reqBkId))
		expr, err = expression.NewBuilder().WithKeyCondition(kcond).WithFilter(filter).Build()
	} else {
		expr, err = expression.NewBuilder().WithKeyCondition(kcond).Build()
	}
	if err != nil {
		panic(err)
	}
	input := &dynamodb.QueryInput{
		KeyConditionExpression:    expr.KeyCondition(),
		IndexName:                 aws.String("RName-Key"),
		FilterExpression:          expr.Filter(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
	}
	input = input.SetTableName("Recipe").SetReturnConsumedCapacity("TOTAL").SetConsistentRead(false)
	// while BkId is unique we are using a GSI so must use Query (I presume)
	result, err := s.dynamodbSvc.Query(input)
	if err != nil {
		return fmt.Errorf("Error: %s [%s] - %s", "in Query in recipeNameSearch of ", s.reqRName, err.Error())
	}
	fmt.Println("recipeNameSearch: Query ConsumedCapacity: \n", result.ConsumedCapacity)
	if int(*result.Count) == 0 {
		return fmt.Errorf("No recipe found in recipeNameSearch, for rname [%s]", s.reqRName)
	}
	// define a slice of struct as Query expects to return 1 or more rows so the slice represents a row
	// and we ue unmarshallistofmaps to handle a batch like select
	recS := make([]RecipeT_, int(*result.Count))
	err = dynamodbattribute.UnmarshalListOfMaps(result.Items, &recS)
	if err != nil {
		return fmt.Errorf("Error: %s [%s] %s", "in UnmarshalMaps in recipeNameSearch ", s.reqRName, err.Error())
	}
	switch len(recS) {
	case 1:
		// single recipe-book found
		s.reqBkId = recS[0].PKey[2:] // trim prefix "R-"
		s.reqRId = strconv.Itoa(recS[0].SortK)
		s.reqRName = recS[0].RName
		//
		// populate Session Context Part data for a recipe
		//
		_, err = s.recipeRSearch()
		if err != nil {
			return fmt.Errorf("Error: %s [%s] %s", "in recipeNameSearch of recipeRSearch ", s.reqRName, err.Error())
		}
		//
		// populate session context Book related data
		//
		err = s.bookNameLookup()
		if err != nil {
			return fmt.Errorf("Error: %s [%s] %s", "in UnmarrecipeNameSearch of shalMaps bookNameLookup ", s.reqRName, err.Error())
		}
		//
		s.dmsg = s.reqRName + " in " + s.reqBkName + " by " + s.authors
		s.vmsg = "yFound " + s.reqRName + " in " + s.reqBkName + " by " + s.authors
		s.vmsg += `What would you like to list?. Say "list container" or "List Ingredient" or "List Prep tasks" or "start Cooking" or "cancel"`
	default:
		// more than one recipe-book found
		//s.makeSelect = true
		s.dmsg = `Recipe appears in more than one book. Please make a selection from the list below. Say "select number\n" `
		s.vmsg = `the recipe appears in more than one book. I will recite the first 6. Please say "next" to hear each one and "select" to choose or "cancel" to exit\n" `
		s.recipeList = nil
		for i := 0; i < len(recS); i++ {
			s.reqBkId = recS[i].PKey[2:] // trim prefix "R-"
			s.reqRId = strconv.Itoa(recS[i].SortK)
			s.reqRName = recS[i].RName
			err = s.bookNameLookup()
			s.ddata += strconv.Itoa(i+1) + ". " + s.reqRName + " in " + s.reqBkName + " by " + s.authors + "\n"
			rec := mRecipeT{Id: i + 1, RName: s.reqRName, RId: s.reqRId, BkName: s.reqBkName, BkId: s.reqBkId, Authors: s.authors, Serves: s.serves}
			s.recipeList = append(s.recipeList, rec)
		}
		// clear session context because mutli records means no-one record is active until user selects one.
		s.reqBkId, s.reqRName, s.reqBkName, s.reqRId, s.authors, s.serves = "", "", "", "", "", ""
	}
	//
	return nil
}

func (s *sessCtx) bookNameLookup() error {
	//
	// user "opens <book>". Alexa provides associated slot-type-id BkId value.
	//
	type recT struct {
		PKey    string
		Authors []string `json:"Authors"`
	}
	flatten := func(w []string) string {
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

	kcond := expression.KeyEqual(expression.Key("BkId"), expression.Value(s.reqBkId)) // must internally converts bookid string to int
	proj := expression.NamesList(expression.Name("Authors"), expression.Name("PKey"))
	expr, err := expression.NewBuilder().WithKeyCondition(kcond).WithProjection(proj).Build()
	if err != nil {
		return fmt.Errorf("Error: %s [%s] - %s", "in NewBuilder in bookNameLookup, bookId ", s.reqBkId, err.Error())
	}
	input := &dynamodb.QueryInput{
		KeyConditionExpression:    expr.KeyCondition(),
		IndexName:                 aws.String("BkId-BkName"),
		FilterExpression:          expr.Filter(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		ProjectionExpression:      expr.Projection(),
	}
	input = input.SetTableName("Recipe").SetReturnConsumedCapacity("TOTAL").SetConsistentRead(false)
	// while BkId is unique we are using a GSI so must use Query (I presume)
	result, err := s.dynamodbSvc.Query(input)
	if err != nil {
		return fmt.Errorf("Error: %s [%s] %s", "in Query in bookNameLookup of ", s.reqBkId, err.Error())
	}
	fmt.Println("bookNameLookup: Query ConsumedCapacity: \n", result.ConsumedCapacity)
	if int(*result.Count) == 0 {
		// Alexa respository means all requests should be for its registered book so we shouldn't get 0 found.
		return fmt.Errorf("Internal error: Book Id [%s] not found", s.reqBkId)
	}
	if int(*result.Count) > 1 {
		return fmt.Errorf("Internal error in bookNameLookup. %s [%s]", "More than one book found for bookId ", s.reqBkId)
	}
	// define a slice of struct as Query expects to return 1 or more rows so the slice represents a row
	// and we ue unmarshallistofmaps to handle a batch like select

	rec := make([]recT, int(*result.Count))
	err = dynamodbattribute.UnmarshalListOfMaps(result.Items, &rec)
	if err != nil {
		return fmt.Errorf("Error: %s [%s] %s", "in UnmarshalMaps in bookNameLookup ", s.reqRName, err.Error())
	}
	s.authors = flatten(rec[0].Authors)
	s.authorS = rec[0].Authors
	s.reqBkName = rec[0].PKey[3:] // trim "BK-" prefix
	fmt.Println("in bookNameLookup: Opened book ", s.reqBkName)
	return nil
}
