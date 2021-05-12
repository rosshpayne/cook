package main

import (
	_ "encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/rosshpayne/cook/global"

	"github.com/aws/aws-sdk-go/aws"
	_ "github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/dynamodb/expression"

	_ "github.com/aws/aws-lambda-go/lambdacontext"
)

// use this struct as key into map
type mkey struct {
	size string
	Type string // probably redundant
}
type clsort []mkey

func (cs clsort) Len() int           { return len(cs) }
func (cs clsort) Less(i, j int) bool { return cs[i].size < cs[j].size }
func (cs clsort) Swap(i, j int)      { cs[i], cs[j] = cs[j], cs[i] }

func (cm ContainerMap) generateContainerUsage(svc *dynamodb.DynamoDB) []string {
	type ctCount struct {
		C           []*Container // list of logical containers in the group
		numPhysical int          // number of physicalId containers in the group e.g. 10 logical containers for 2 physicalId means 1 physicalId container has many uses (reuse) across activities
	}
	var b strings.Builder
	var output_ []string

	if len(cm) == 0 {
		return nil
	}
	// use map to group-by-container-type-and-size - map value contains list of identical containers and the number of them
	ctGroup := make(map[mkey]*ctCount)
	//
	for _, v := range cm {
		//var size_ string
		// for each container aggregate based on type (bowl, plate, tray, ebowl) and size (small,medium,large)
		if v.Measure == nil {
			// ingore container if no measurements defined - unlikely
			continue
		}
		size_ := v.String()
		//
		// key is made up of a containers, size & type
		//
		z := mkey{size: size_, Type: strings.ToLower(v.Type)}
		// identical based on {size,Type}
		if y, ok := ctGroup[z]; !ok {
			// {size,Type} does not exist - create first one
			y := new(ctCount)
			y.numPhysical = 1
			v.physicalId = y.numPhysical
			y.C = append(y.C, v)
			ctGroup[z] = y
		} else {
			// assign the logical container (v) to a physical container id.
			var ok bool
			for i := 1; i <= y.numPhysical; i++ {
				ok = true
				for _, oc := range y.C {
					if oc.physicalId == i {
						if !(oc.last < v.start || v.last < oc.start) {
							ok = false
						}
					}
				}
				if ok {
					// logical container can work with this physicalId container
					v.physicalId = y.numPhysical
					break
				}
			}
			if !ok { // logical container requires new physicalId container
				y.numPhysical++
				v.physicalId = y.numPhysical
			}
			// append v reused or not..
			y.C = append(y.C, v)
		}
	}
	// The key in ctGroup can be sorted using clsort type. Once sorted we can access ctGroup in sorted ordered.\
	clsorted := clsort{}
	for k, _ := range ctGroup {
		clsorted = append(clsorted, k)
	}
	// use sorted key to index into container map
	sort.Sort(clsorted)
	var footnote bool
	for _, v := range clsorted {
		//
		// containers belonging to same {size,type}
		//
		cCnt := len(ctGroup[v].C)
		if cCnt != ctGroup[v].numPhysical {
			//	b.WriteString(fmt.Sprintf(" %d-%d* %s %s", ctGroup[v].numPhysical, cCnt, strings.ToLower(v.size), v.Type) + "s")
			b.WriteString(fmt.Sprintf(" %d-%d* %s", ctGroup[v].numPhysical, cCnt, strings.ToLower(v.size)) + "s")
			footnote = true
		} else {
			if cCnt == 1 {
				var (
					t string
					m string
					r string
				)
				c := ctGroup[v].C[0]
				t = v.Type
				if len(c.Label) > 0 {
					t = c.Label
				}
				if len(c.Prelabel) > 0 {
					r = ", " + c.Postlabel
				}
				if c.Measure != nil {
					m = c.Measure.String()
				}
				if len(c.Postlabel) > 0 {
					r = ", " + c.Postlabel
				}
				b.WriteString(fmt.Sprintf(" 1 %s %s %s", m, strings.ToLower(t), r))
			} else {
				b.WriteString(fmt.Sprintf(" %d %s %s", cCnt, strings.ToLower(v.size), v.Type))
				if cCnt > 1 {
					b.WriteString("s")
				}
			}
		}
		output_ = append(output_, b.String())
		b.Reset()
	}
	if footnote {
		b.WriteString(" ")
		b.WriteString("* lower value applies when you wash the container immediately after use, to maximise reuse")
	}
	output_ = append(output_, b.String())
	// store number of records in recipe table
	return output_
}

func (a Activities) GenerateTasks(pKey string, r *RecipeT, s *sessCtx) prepTaskS {
	// Merge and Populate prepTask and then sort.
	//  1. first load parrellelisable tasks identified by words or prep property "parallel" or device (=oven)
	//  2. sort
	//  3. add other tasks in order
	//
	type atvTask struct {
		AId int
		TId int
	}
	var ptS prepTaskS // this type satisfies sort interface.
	processed := make(map[atvTask]bool, prepctl.cnt)

	firstInstructon := func(idx string) int {
		// no-part part
		fmt.Println("Looking for ", idx)
		if idx == "nopart_" {
			for n := 0; n < len(ptS); n++ {
				if len(ptS[n].Part) == 0 {
					return ptS[n].SortK
				}
			}

		}
		// part
		for n := 0; n < len(ptS); n++ {
			if ptS[n].Part == idx {
				return ptS[n].SortK
			}
		}
		return -1
	}

	nextPartInstructon := func(i int) int {
		for n := i + 1; n < len(ptS); n++ {
			if ptS[n].Part == ptS[i].Part {
				return ptS[n].SortK
			}
		}
		return -1
	}

	prevPartInstructon := func(i int) int {
		for n := i - 1; n >= 0; n-- {
			if ptS[n].Part == ptS[i].Part {
				return ptS[n].SortK
			}
		}
		return -1
	}
	//
	// process Div
	//
	for pa := &a[0]; pa != nil; pa = pa.next {
		var adiv string
		if len(pa.Division) > 0 {
			adiv = pa.Division
		}
		for i := 0; i < len(pa.Prep); i++ {
			if len(pa.Prep[i].Division) == 0 && len(adiv) > 0 {
				pa.Prep[i].Division = adiv
			}
			if len(pa.Prep[i].Div_) > 0 {
				pa.Prep[i].divOnly = true
				pa.Prep[i].Division = pa.Prep[i].Div_
			}
		}
		for i := 0; i < len(pa.Task); i++ {
			if len(pa.Task[i].Division) == 0 && len(adiv) > 0 {
				pa.Task[i].Division = adiv
			}
			if len(pa.Task[i].Div_) > 0 {
				pa.Task[i].divOnly = true
				pa.Task[i].Division = pa.Task[i].Div_
			}
		}

	}
	//
	// Are threads used..
	//
	var (
		thrdBased bool
	)
	for pa := &a[0]; pa != nil; pa = pa.next {

		if len(pa.Thread) > 0 {
			thrdBased = true
			break
		}
		if !thrdBased {
			for i := 0; i < len(pa.Prep); i++ {
				if len(pa.Prep[i].Thread) > 0 {
					thrdBased = true
					break
				}
			}
			if !thrdBased {
				for i := 0; i < len(pa.Task); i++ {
					if len(pa.Task[i].Thread) > 0 {
						thrdBased = true
						break
					}
				}
			}
		}
	}
	fmt.Println("thread based: ", thrdBased)
	//
	// check all tasks have thread value if activity is assigned to a thread
	//
	if thrdBased {
		// Assign thread values to tasks.
		for pa := &a[0]; pa != nil; pa = pa.next {
			for i := 0; i < len(pa.Prep); i++ {
				if len(pa.Prep[i].Thread) == 0 && len(pa.Thread) > 0 {
					pa.Prep[i].Thread = pa.Thread
				}
			}
			for i := 0; i < len(pa.Task); i++ {
				if len(pa.Task[i].Thread) == 0 && len(pa.Thread) > 0 {
					pa.Task[i].Thread = pa.Thread
				}
			}
		}
	}
	//
	// sort parallelisable prep tasks
	//
	for pa := prepctl.start; pa != nil; pa = pa.nextPrep {
		var add bool
		for ia, pp := range pa.Prep { // slice of prep tasks
			if pp.UseDevice != nil {
				if strings.ToLower(pp.UseDevice.Type) == "oven" {
					add = true
				}
			}
			if pp.Parallel && pp.WaitOn == 0 || add {
				var thrd int
				var err error
				add = false
				processed[atvTask{pa.AId, ia}] = true
				if len(pp.Thread) > 0 {
					thrd, err = strconv.Atoi(pp.Thread)
					if err != nil {
						panic(fmt.Errorf("Error: cannot convert to int for Thread in ptR %s", pp.Thread))
					}
				}
				pt := taskRecT{PKey: pKey, AId: pa.AId, Type: 'P', time: pp.Time, Text: pp.text, Verbal: pp.verbal, Thread: thrd, MergeThrd: pp.MergeThrd, Division: pp.Division, DivOnly: pp.divOnly, Part: pa.Part, taskp: pp}
				ptS = append(ptS, &pt)
			}
		}
	}
	sort.Sort(ptS)
	//
	// generate SortK Ids
	//
	var i int = 1 // start at one as works better with Dynamodb UpateItem (nolonger used) ADD semantics.
	for j := 0; j < len(ptS); i++ {
		ptS[j].SortK = i
		j++
	}
	//
	// append remaining prep tasks - these are serial tasks so order unimportant
	//
	for pa := prepctl.start; pa != nil; pa = pa.nextPrep {
		for ia, pp := range pa.Prep {
			var thrd int
			var err error
			if pp.WaitOn > 0 {
				continue
			}
			if _, ok := processed[atvTask{pa.AId, ia}]; ok {
				continue
			}
			processed[atvTask{pa.AId, ia}] = true
			if len(pp.Thread) > 0 {
				thrd, err = strconv.Atoi(pp.Thread)
				if err != nil {
					panic(fmt.Errorf("Error: cannot convert to int for Thread in ptR %s", pp.Thread))
				}
			}
			pt := taskRecT{PKey: pKey, SortK: i, AId: pa.AId, Type: 'P', time: pp.Time, Text: pp.text, Verbal: pp.verbal, Thread: thrd, MergeThrd: pp.MergeThrd, Division: pp.Division, DivOnly: pp.divOnly, Part: pa.Part, taskp: pp}
			ptS = append(ptS, &pt)
			i++
		}
	}
	// now for all WaitOn prep tasks
	for pa := prepctl.start; pa != nil; pa = pa.nextPrep {
		for ia, pp := range pa.Prep {
			var thrd int
			var err error
			if _, ok := processed[atvTask{pa.AId, ia}]; ok {
				continue
			}
			if len(pp.Thread) > 0 {
				thrd, err = strconv.Atoi(pp.Thread)
				if err != nil {
					panic(fmt.Errorf("Error: cannot convert to int for Thread in ptR %s", pp.Thread))
				}
			}
			pt := taskRecT{PKey: pKey, SortK: i, AId: pa.AId, Type: 'P', time: pp.Time, Text: pp.text, Verbal: pp.verbal, Thread: thrd, MergeThrd: pp.MergeThrd, Division: pp.Division, DivOnly: pp.divOnly, Part: pa.Part, taskp: pp}
			ptS = append(ptS, &pt)
			i++
		}
	}
	//
	// append tasks
	//
	//timerMsg := "Set a timer for %s, and get back to me at that time."
	for pa := taskctl.start; pa != nil; pa = pa.nextTask {
		for _, pp := range pa.Task {
			var thrd int
			var err error
			if len(pp.Thread) > 0 {
				thrd, err = strconv.Atoi(pp.Thread)
				if err != nil {
					panic(fmt.Errorf("Error: cannot convert to int for Thread in ptR %s", pp.Thread))
				}
			}
			pt := taskRecT{PKey: pKey, SortK: i, AId: pa.AId, Type: 'T', time: pp.Time, Text: pp.text, Verbal: pp.verbal, Thread: thrd, MergeThrd: pp.MergeThrd, Division: pp.Division, DivOnly: pp.divOnly, Part: pa.Part, taskp: pp}
			ptS = append(ptS, &pt)
			i++
			// if pp.Timer != nil && pp.Timer.Set {
			// 	//
			// 	// create a timer instruction - this may trigger a Alexa Reminder in future.
			// 	//
			// 	var ts string
			// 	var msg string
			// 	if pp.Timer.Time == 0 {
			// 		ts = fmt.Sprintf("%d%s", pp.Time, UnitMap[pp.Unit].String(pp))
			// 	} else {
			// 		ts = fmt.Sprintf("%d%s", pp.Timer.Time, UnitMap[pp.Timer.Unit].String(pp.Timer))
			// 	}
			// 	if len(pp.Timer.Msg) > 0 {
			// 		msg = fmt.Sprintf(pp.Timer.Msg, ts)
			// 	} else {
			// 		msg = fmt.Sprintf(timerMsg, ts)
			// 	}
			// 	pt := taskRecT{PKey: pKey, SortK: i, AId: pa.AId, Type: 'T', Text: msg, Verbal: msg, Thread: thrd, Division: pp.Division, Part: pa.Part, taskp: pp}
			// 	ptS = append(ptS, &pt)
			// 	i++
			// }

		}
	}
	// now that we know the size of the list assign End-Of-List field. This approach replaces MaxId[] set stored in Recipe table
	// this mean each record knows how long the list is - helpful in single record processing (which is nolonger used)
	eol := len(ptS)
	//TODO : consider limit of 10 can be replaced with dynamic value
	pcnt := make(map[string]int, 10) // upto ten parts only
	// number of instructions per part
	for _, v := range ptS {
		pcnt[v.Part] += 1
	}
	//
	// if parts not used assign EOL and return
	//
	if len(pcnt) == 1 {
		for _, v := range ptS {
			v.EOL = eol
		}
		return ptS
	}
	//
	//	order of instruction in part or no-part recipe, is driven by recipe.SortK which inturn is defined by activity.AId
	//
	for i, v := range ptS {
		v.EOL = eol
		v.Next = nextPartInstructon(i)
		v.PEOL = pcnt[v.Part]
	}
	for i := len(ptS) - 1; i >= 0; i-- {
		ptS[i].Prev = prevPartInstructon(i)
	}
	//
	// Link first instruction for each partition of recipe to Recipe data.
	//
	partM := make(map[string]bool)
	// find if there are any parts to recipe
	for a := &a[0]; a != nil; a = a.next {
		if len(a.Part) > 0 {
			partM[a.Part] = false
		} else {
			partM["nopart_"] = false
		}
	}
	//
	// check there are no unregistered (in recipe) parts/div/threads
	//
	for k, _ := range partM {
		// for each part in Map - which was just populated by scanning across all activities
		var found bool
		if k == "nopart_" {
			continue
		}
		// loop through all entries in recipe parts
		for _, p := range s.parts {
			if k == p.Index {
				found = true
				break
			}
		}
		if !found {
			panic(fmt.Errorf("Part [%s] in Activity but not incuded in recipe part description", k))
		}
	}
	//
	// check to see if recipe not already assigned part start values
	//  this fuction has two entry points, one from loadBaseRecipe and the other loadBaseContainers
	//
	for i := 0; i < len(s.parts); i++ {
		if s.parts[i].Index == "nopart_" {
			return ptS
		}
	}
	// assign recipe.Start of first SortK value for a part.
	//
	Parts := s.parts
	for i := 0; i < len(Parts); i++ {
		Parts[i].Start = firstInstructon(Parts[i].Index)
	}
	// prepend no-part part to Parts
	if _, ok := partM["nopart_"]; ok {
		var rpart []PartT
		rpart = []PartT{PartT{Index: "nopart_", Title: "Main", Start: firstInstructon("nopart_")}}
		rpart = append(rpart, Parts...)
		s.parts = rpart
	}
	Parts = s.parts
	if r != nil {
		r.Part = s.parts
	}
	//
	// assign PId, record (instruction) id within a part. For no part this is the SortK value.
	//
	for k, _ := range partM {
		var (
			start int
			peol  int
		)
		for _, r := range Parts {
			if r.Index == k {
				start = r.Start
			}
		}
		peol = ptS[start-1].PEOL
		for i, p := 1, ptS[start-1]; i <= peol; i++ {
			p.PId = i
			if p.Next > 0 {
				p = ptS[p.Next-1]
			}
		}
	}
	//
	// Add (or update) part data to Recipe record (R-)
	//
	if r != nil {
		s.updateRecipe(r)
	}
	//
	return ptS
}

// Recipe table
type PkeysT1 struct {
	PKey  string `json:"PKey"`
	SortK int    `json='SortK"`
}

// Ingredient table
type PkeysT2 struct {
	PKey  string `json:"PKey"`
	SortK string `json:"SortK"`
}

// contains meta-data that defines what is purged
type purge struct {
	prefix string
	table  string
}

func (s *sessCtx) purgeRecipe() error {
	//
	items := []purge{
		{prefix: "A-", table: "Recipe"},     // explicitly defined activities
		{prefix: "T-", table: "Recipe"},     // task list
		{prefix: "D-", table: "Recipe"},     // device list
		{prefix: "C-", table: "Recipe"},     // container list
		{prefix: "R-", table: "Recipe"},     // recipe name
		{prefix: "C-", table: "Ingredient"}, // explicitly defined containers that span activities
	}
	var kcond expression.KeyConditionBuilder
	for _, p := range items {
		if p.prefix == "R-" {
			rid, _ := strconv.Atoi(s.reqRId)
			kcond = expression.KeyAnd(expression.Key("PKey").Equal(expression.Value(p.prefix+s.reqBkId)), expression.Key("SortK").Equal(expression.Value(rid)))
		} else {
			kcond = expression.KeyEqual(expression.Key("PKey"), expression.Value(p.prefix+s.pkey))
		}
		proj := expression.NamesList(expression.Name("PKey"), expression.Name("SortK"))
		expr, err := expression.NewBuilder().WithKeyCondition(kcond).WithProjection(proj).Build()
		if err != nil {
			panic(err)
		}
		input := &dynamodb.QueryInput{
			KeyConditionExpression:    expr.KeyCondition(),
			FilterExpression:          expr.Filter(),
			ExpressionAttributeNames:  expr.Names(),
			ExpressionAttributeValues: expr.Values(),
			ProjectionExpression:      expr.Projection(),
		}
		input = input.SetTableName(p.table).SetReturnConsumedCapacity("TOTAL").SetConsistentRead(false)
		//*dynamodb.DynamoDB,
		result, err := s.dynamodbSvc.Query(input)
		if err != nil {
			return fmt.Errorf("Error: in purgeRecipe Query - %s", err.Error())
		}
		switch p.table {
		case "Recipe":
			purgeKeyS := make([]PkeysT1, int(*result.Count))
			err = dynamodbattribute.UnmarshalListOfMaps(result.Items, &purgeKeyS)
			if err != nil {
				return fmt.Errorf("** Error during UnmarshalListOfMaps in purgeRecipe - %s", err.Error())
			}
			for _, v := range purgeKeyS {
				pk := PkeysT1{PKey: v.PKey, SortK: v.SortK}
				av, err := dynamodbattribute.MarshalMap(pk)
				if err != nil {
					return fmt.Errorf("%s: %s", "Error: failed to marshal Record in purgeRecipe", err.Error())
				}
				_, err = s.dynamodbSvc.DeleteItem(&dynamodb.DeleteItemInput{
					TableName: aws.String(p.table),
					Key:       av,
				})
				if err != nil {
					return fmt.Errorf("%s: %s", "Error: failed to DeleteItem in purgeRecipe", err.Error())
				}
			}
			//
		case "Ingredient":
			purgeKeyS := make([]PkeysT2, int(*result.Count))
			err = dynamodbattribute.UnmarshalListOfMaps(result.Items, &purgeKeyS)
			if err != nil {
				return fmt.Errorf("** Error during UnmarshalListOfMaps in purgeRecipe - %s", err.Error())
			}
			for _, v := range purgeKeyS {
				pk := PkeysT2{PKey: v.PKey, SortK: v.SortK}
				av, err := dynamodbattribute.MarshalMap(pk)
				if err != nil {
					return fmt.Errorf("%s: %s", "Error: failed to marshal Record in purgeRecipe", err.Error())
				}
				_, err = s.dynamodbSvc.DeleteItem(&dynamodb.DeleteItemInput{
					TableName: aws.String(p.table),
					Key:       av,
				})
				if err != nil {
					return fmt.Errorf("%s: %s", "Error: failed to DeleteItem in purgeRecipe", err.Error())
				}
			}
		}
	}
	//
	// purge indexed entries
	//
	fcond := expression.Equal(expression.Name("SortK"), expression.Value(s.pkey))
	proj := expression.NamesList(expression.Name("PKey"), expression.Name("SortK"))
	expr, err := expression.NewBuilder().WithProjection(proj).WithFilter(fcond).Build()
	if err != nil {
		return fmt.Errorf("%s", "Error: failed to NewBuilder for ingredient purge in purgeRecipe "+err.Error())
	}
	//
	// purge recipe search entries (as defined by Index attribute in Attributes)
	//
	params := &dynamodb.ScanInput{
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		ProjectionExpression:      expr.Projection(),
		FilterExpression:          expr.Filter(),
		TableName:                 aws.String("Ingredient"),
	}
	result, err := s.dynamodbSvc.Scan(params)
	if err != nil {
		return fmt.Errorf("%s", "Error in scan of unit table: "+err.Error())
	}
	purgeKeyS := make([]PkeysT2, int(*result.Count))
	err = dynamodbattribute.UnmarshalListOfMaps(result.Items, &purgeKeyS)
	if err != nil {
		return fmt.Errorf("Error during UnmarshalListOfMaps of Ingredient in purgeRecipe - %s", err.Error())
	}
	for _, v := range purgeKeyS {
		pk := PkeysT2{PKey: v.PKey, SortK: v.SortK}
		av, err := dynamodbattribute.MarshalMap(pk)
		if err != nil {
			return fmt.Errorf("%s: %s", "Error: failed to MarshalMap  of Ingredient in purgeRecipe", err.Error())
		}
		_, err = s.dynamodbSvc.DeleteItem(&dynamodb.DeleteItemInput{
			TableName: aws.String("Ingredient"),
			Key:       av,
		})
		if err != nil {
			return fmt.Errorf("%s: %s", "Error: failed to DeleteItem of Ingredient in purgeRecipe", err.Error())
		}
	}
	return nil
}

// literal tags are tags in the form : {type:q|u|s|n}
// where type is "m" for measure or "t" for time or "l" for length, "c" for container
// note type "m" is salable all others are not scaled.
// literal tags are provided to add flexibility to embed a measurement anywhere beyond what the data
// model is designed to handle. All tags except m type are non-scalable

func expandLiteralTags(str string, s ...*sessCtx) string {
	var (
		b          strings.Builder // supports io.Write write expanded text/verbal text to this buffer before saving to Task or Verbal fields
		tclose     int
		topen      int
		nm         *MeasureT
		savedScale float64
	)

	resetScale := func() func() {
		savedScale = global.GetScale()
		global.SetScale(1)
		return func() { global.SetScale(savedScale) }
	}
	// literal tags are not scalable, set scale to 1 for duration of function.
	defer resetScale()()
	for tclose, topen = 0, strings.IndexByte(str, '{'); topen != -1; {
		b.WriteString(str[tclose:topen])
		nextclose := strings.IndexByte(str[topen:], '}')
		if nextclose == -1 {
			panic(fmt.Errorf("Error: closing } not found in expandIngrd() [%s]", str))
		}
		nextopen := strings.IndexByte(str[topen+1:], '{')
		if nextopen != -1 {
			if nextclose > nextopen {
				panic(fmt.Errorf("Error: closing } not found in expandIngrd() [%s]", str))
			}
		}
		tclose += strings.IndexByte(str[tclose:], '}')
		//
		tag := strings.Split(str[topen+1:tclose], ":")
		switch tag[0] {
		case "m":
			// weight measure literal - needs to be scaled
			pt := strings.Split(strings.ToLower(tag[1]), "|")
			nm = &MeasureT{Num: pt[3], Quantity: pt[0], Size: pt[2], Unit: pt[1]}
			global.SetScale(savedScale)
			b.WriteString(nm.String())
			global.SetScale(1.0)
		case "t":
			// time literal
			fmt.Println("time literal ", tag[1])
			pt := strings.Split(strings.ToLower(tag[1]), "|")
			b.WriteString(pt[0] + UnitMap[pt[1]].String())
		case "l":
			// length literal
			pt := strings.Split(strings.ToLower(tag[1]), "|")
			b.WriteString(pt[0] + UnitMap[pt[1]].String())
		case "T":
			// temp literal
			pt := strings.Split(tag[1], "|")
			d := &DeviceT{Unit: pt[0], Temp: pt[1], Set: pt[2]}
			fmt.Printf("usage: for T: %#v", *d)
			b.WriteString(d.String())
		case "c":
			// {addtoC} tag where container is scalable
			pt := strings.Split(strings.ToLower(tag[1]), "|")
			fmt.Println("expand container tag..", pt)
			c := &Container{Label: pt[0], Measure: &MeasureCT{Quantity: pt[1], Size: pt[2], Shape: pt[3], Dimension: pt[4], Height: pt[5], Unit: pt[6]}}
			if len(s) > 0 {
				fmt.Println("expand container: s passed in")
				if s[0].dispCtr != nil {
					fmt.Printf("user defined container: %#v\n", s[0].dispCtr)
					if len(s[0].dispCtr.UDimension) > 0 {
						if s[0].dispCtr.UDimension != s[0].dispCtr.Dimension {
							b.WriteString(" your " + c.label())
						} else {
							b.WriteString(" a " + c.String())
						}
					} else {
						b.WriteString(" a " + c.String())
					}
				} else {
					fmt.Println("NO user defined container: dispCtr is nil")
					b.WriteString(" a " + c.String())
				}
			} else {
				fmt.Println("expand container: s not passed in")
				b.WriteString(" a " + c.String())
			}
		default:
			// not a literal tag, pass through
			b.WriteString(str[topen : tclose+1])
		}
		//
		tclose += 1
		topen = strings.IndexByte(str[tclose:], '{')
		if topen == -1 {
			b.WriteString(str[tclose:])
		} else {
			topen += tclose
		}
	}
	if tclose == 0 {
		// no {} found
		return str
	}
	return b.String()
}
