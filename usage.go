package main

import (
	_ "encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

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
	Type string
}
type clsort []mkey

func (cs clsort) Len() int           { return len(cs) }
func (cs clsort) Less(i, j int) bool { return cs[i].size < cs[j].size }
func (cs clsort) Swap(i, j int)      { cs[i], cs[j] = cs[j], cs[i] }

func (cm ContainerMap) generateContainerUsage(svc *dynamodb.DynamoDB) []string {
	type ctCount struct {
		C   []*Container
		num int
	}
	var b strings.Builder
	var output_ []string

	if len(cm) == 0 {
		return nil
	}
	// use map to group-by-container-type-and-size - map value contains list of identical containers and the number of them
	identicalC := make(map[mkey]*ctCount)
	//
	done := make(map[string]bool)
	for k, v := range cm {
		var size_ string
		// for each container aggregate based on type and size
		if v.Measure == nil {
			continue
		}
		if len(v.Measure.Size) > 0 {
			size_ = strings.ToLower(v.Measure.Size)
		} else {
			// where no size defined give each container its own size - order at the top
			size_ = "AAA" + k
		}
		z := mkey{size: size_, Type: strings.ToLower(v.Type)}
		// identical based on {size,Type}
		if y, ok := identicalC[z]; !ok {
			// {size,Type} does not exist - create first one
			y := new(ctCount)
			y.num = 1
			y.C = append(y.C, v)
			identicalC[z] = y
		} else {
			// check if the container can be reused by examining other containers in the identical list
			var reuse bool
			if !done[v.Cid] {
				// for containers not already matched as ok to reuse
				for _, oc := range y.C {
					// fmt.Printf("loop check for %s  %s\n", v.Cid, oc.Cid)
					// fmt.Printf("oc.last %d  < %d v.start,  v.last  %d  < %d oc.start \n", oc.last, v.start, v.last, oc.start)
					if oc.last <= v.start || v.last <= oc.start {
						done[oc.Cid] = true // don't check for this Container again.
						reuse = true
						// all containers that represent the same physical container have the same reused value
						oc.reused, v.reused = y.num, y.num
						// bind these containers which means that are the same physical container.
						fmt.Printf("reuse true for %s\n", oc.Cid)
						break
					}
				}
				if !reuse {
					y.num += 1
				}
			}
			y.C = append(y.C, v)
		}
	}
	// Populate slice, which satisfies sort interface, with the map key.
	// Objective - sort the may key which we then use to access the map in sorted order.
	clsorted := clsort{}
	for k, _ := range identicalC {
		clsorted = append(clsorted, k)
	}
	// use sorted key to index into container map - sorted by size attribute in container.measure.
	sort.Sort(clsorted)
	for _, v := range clsorted {
		//
		// containers belonging to same {size,type}
		//
		if len(identicalC[v].C) > 1 {
			// use Type as this is the attribute that is used to aggregated the containers
			// and each container may have a different label. Not so if were dealing with just one container of course
			fmt.Printf("v = %#v\n", v)
			b.WriteString(fmt.Sprintf(" %d %s %s", identicalC[v].num, strings.Title(v.size), v.Type))

			if len(identicalC[v].C) != identicalC[v].num {
				if identicalC[v].num > 1 {
					b.WriteString(fmt.Sprintf("%s ", "s"))
				}
				// some containers are reused
				for i := identicalC[v].num; i > 0; i-- {
					b.WriteString(fmt.Sprintf(" ( "))
					var newLine bool
					for _, c := range identicalC[v].C {
						if c.reused == i {
							if !newLine {
								b.WriteString(fmt.Sprintf(" %s ", strings.ToLower(c.Contains)))
								newLine = true
							} else {
								b.WriteString(fmt.Sprintf(" then %s ", strings.ToLower(c.Contains)))
							}
						}
					}
					b.WriteString(fmt.Sprintf(" ) "))
				}
			} else {
				// no containers are reused.
				if identicalC[v].num > 1 {
					b.WriteString(fmt.Sprintf("%s ", "s"))
				}
				for i, d := range identicalC[v].C {
					switch i {
					case 0:
						if len(d.Contains) > 0 {
							b.WriteString(fmt.Sprintf(" one for %s ", strings.ToLower(d.Contains)))
						}
					default:
						if len(d.Contains) > 0 {
							b.WriteString(fmt.Sprintf(" another for %s ", strings.ToLower(d.Contains)))
						}
					}
				}
			}
		} else {
			//
			// only one logical container or only one physical container in the identical grouping
			//
			c := identicalC[v].C[0]
			if v.size[:3] == "AAA" {
				b.WriteString(" 1 ")
				b.WriteString(c.String())
			} else {
				b.WriteString(fmt.Sprintf(" 1 %s %s", strings.Title(v.size), c.Label))
			}
			if len(c.Purpose) > 0 {
				if c.Purpose[0] == '_' {
					b.WriteString(fmt.Sprintf(" for %s ", strings.ToLower(c.Contains+"  "+c.Purpose[1:]+" ")))
				} else {
					b.WriteString(fmt.Sprintf(" for %s ", strings.ToLower(c.Purpose+" "+c.Contains+"  ")))
				}
			}
		}
		output_ = append(output_, b.String())
		b.Reset()
	}

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
	// sort parallelisable prep tasks
	//
	for p := prepctl.start; p != nil; p = p.nextPrep {
		var add bool
		for ia, pp := range p.Prep { // slice of prep tasks
			if pp.UseDevice != nil {
				if strings.ToLower(pp.UseDevice.Type) == "oven" {
					add = true
				}
			}
			if pp.Parallel && pp.WaitOn == 0 || add {
				add = false
				processed[atvTask{p.AId, ia}] = true
				pt := prepTaskRec{PKey: pKey, AId: p.AId, Type: 'P', time: pp.Time, Text: pp.text, Verbal: pp.verbal, Part: p.Part, taskp: pp}
				ptS = append(ptS, &pt)
			}
		}
	}
	sort.Sort(ptS)
	//
	// generate SortK Ids
	//
	var i int = 1 // start at one as works better with Dynamodb UpateItem ADD semantics.
	for j := 0; j < len(ptS); i++ {
		ptS[j].SortK = i
		j++
	}
	//
	// append remaining prep tasks - these are serial tasks so order unimportant
	//
	for p := prepctl.start; p != nil; p = p.nextPrep {
		for ia, pp := range p.Prep {
			if pp.WaitOn > 0 {
				continue
			}
			if _, ok := processed[atvTask{p.AId, ia}]; ok {
				continue
			}
			processed[atvTask{p.AId, ia}] = true
			pt := prepTaskRec{PKey: pKey, SortK: i, AId: p.AId, Type: 'P', time: pp.Time, Text: pp.text, Verbal: pp.verbal, Part: p.Part, taskp: pp}
			ptS = append(ptS, &pt)
			i++
		}
	}
	// now for all WaitOn prep tasks
	for p := prepctl.start; p != nil; p = p.nextPrep {
		for ia, pp := range p.Prep {
			if _, ok := processed[atvTask{p.AId, ia}]; ok {
				continue
			}
			pt := prepTaskRec{PKey: pKey, SortK: i, AId: p.AId, Type: 'P', time: pp.Time, Text: pp.text, Verbal: pp.verbal, Part: p.Part, taskp: pp}
			ptS = append(ptS, &pt)
			i++
		}
	}
	//
	// append tasks
	//
	for p := taskctl.start; p != nil; p = p.nextTask {
		for _, pp := range p.Task {
			pt := prepTaskRec{PKey: pKey, SortK: i, AId: p.AId, Type: 'T', time: pp.Time, Text: pp.text, Verbal: pp.verbal, Part: p.Part, taskp: pp}
			ptS = append(ptS, &pt)
			i++
		}
	}
	// now that we know the size of the list assign End-Of-List field. This approach replaces MaxId[] set stored in Recipe table
	// this mean each record knows how long the list is - helpful in a stateless Lambda app.
	eol := len(ptS)
	//TODO : replace 5
	pcnt := make(map[string]int, 5)
	// number of instructions per part
	for _, v := range ptS {
		pcnt[v.Part] += 1
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
	// Link first instruction for each partition of recipe to recipe part data.
	//
	partM := make(map[string]struct{})
	rPart := make(map[string]bool)
	// find if there are any parts to recipe
	for a := &a[0]; a != nil; a = a.next {
		if len(a.Part) > 0 {
			partM[a.Part] = struct{}{}
		} else {
			partM["nopart_"] = struct{}{}
		}
	}
	fmt.Printf("%#v\n", partM)
	//
	// assign first instruction record id (SortK) for each partition or non-partition Recipes in Recipe data
	//
	for k, _ := range partM {
		rPart[k] = false
		for _, v := range ptS {
			// scan thru instructions looking for first instruct for part
			if len(v.Part) == 0 && !rPart["nopart_"] {
				r.Start = v.SortK
				rPart["nopart_"] = true
				break
			}
			// found first instruction for part in ptS list
			if v.Part == k {
				// find recipe part entry and update Start
				for _, rp := range r.Part {
					if rp.Index == k {
						rPart[k] = true
						rp.Start = v.SortK
						break
					}
				}
			}
		}
	}
	//
	// Error if part in Activity is not represented in Recipe data
	//
	for k, v := range rPart {
		if !v {
			panic(fmt.Errorf("Error: no Recipe entry for recipe part [%s]", k))
		}
	}
	//
	s.updateRecipe(r)
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

func expandLiteralTags(str string) string {
	var (
		b      strings.Builder // supports io.Write write expanded text/verbal text to this buffer before saving to Task or Verbal fields
		tclose int
		topen  int
		nm     *MeasureT
	)

	resetScale := func() func() {
		savedScale := pIngrdScale
		pIngrdScale = 1
		return func() { pIngrdScale = savedScale }

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
		tag := strings.Split(strings.ToLower(str[topen+1:tclose]), ":")
		switch tag[0] {
		case "m":
			// measure literal
			pt := strings.Split(strings.ToLower(tag[1]), "|")
			nm = &MeasureT{Num: pt[3], Quantity: pt[0], Size: pt[2], Unit: pt[1]}
			b.WriteString(nm.String())
		case "t":
			// time literal
			pt := strings.Split(strings.ToLower(tag[1]), "|")
			b.WriteString(pt[0] + unitMap[pt[1]].String())
		default:
			// not a literal tag - pass through unchanged
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
