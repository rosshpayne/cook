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
	for _, v := range cm {
		// for each container aggregate based on type and size
		z := mkey{strings.ToLower(v.Measure.Size), strings.ToLower(v.Type)}
		if y, ok := identicalC[z]; !ok {
			// does not exist - create first one
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
					fmt.Printf("loop check for %s  %s\n", v.Cid, oc.Cid)
					fmt.Printf("oc.last %d  < %d v.start,  v.last  %d  < %d oc.start \n", oc.last, v.start, v.last, oc.start)
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
		if len(identicalC[v].C) > 1 {
			// use typE as this is the attribute that is used to aggregated the containers
			// and each container may have a different label. Not so if were dealing with just one container of course
			b.WriteString(fmt.Sprintf(" %d %s %s", identicalC[v].num, strings.Title(v.size), v.typE))
			if identicalC[v].num > 1 {
				b.WriteString(fmt.Sprintf("%s", "s"))
			}
			if len(identicalC[v].C) != identicalC[v].num {
				if v.typE == "measuring cup" {
					output_ = append(output_, b.String())
					b.Reset()
					continue
				}
				// some reuse
				for i := identicalC[v].num; i > 0; i-- {
					b.WriteString(fmt.Sprintf(" ( "))
					for _, c := range identicalC[v].C {
						if c.reused == i {
							b.WriteString(fmt.Sprintf(" %s ", strings.ToLower(c.Contains)))
						}
					}
					b.WriteString(fmt.Sprintf(" ) "))
				}
			} else {
				for i, d := range identicalC[v].C {
					if d.Type == "Measuring Cup" {
						continue
					}
					switch i {
					case 0:
						if len(d.Contains) > 0 {
							b.WriteString(fmt.Sprintf(" one for %s ", strings.ToLower(d.Contains)))
						}
					default:
						//TODO: what about container reuse.
						var written bool
						for _, oc := range identicalC[v].C {
							if (oc.last <= d.start || d.last <= oc.start) && len(oc.Contains) > 0 {
								b.WriteString(fmt.Sprintf("%s ", " and later "+strings.ToLower(oc.Contains)))
								written = true
							}
						}
						if !written && len(d.Contains) > 0 {
							b.WriteString(fmt.Sprintf(" another for %s ", strings.ToLower(d.Contains)))
							written = false
						}
					}
				}
			}
		} else {
			// single container of this type and size
			c := identicalC[v].C[0]
			if len(v.size) > 0 {
				b.WriteString(fmt.Sprintf(" %d %s %s", identicalC[v].num, strings.Title(v.size), strings.ToLower(c.Label)))
			} else {
				if len(c.Measure.Height) > 0 {
					b.WriteString(fmt.Sprintf(" %d %sx%s%s %s ", identicalC[v].num, c.Measure.Height, c.Measure.Dimension, c.Measure.Unit, strings.ToLower(c.Label)))
				} else {
					b.WriteString(fmt.Sprintf(" %d %s%s %s ", identicalC[v].num, c.Measure.Dimension, c.Measure.Unit, strings.ToLower(c.Label)))
				}
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

func (a Activities) GenerateTasks(pKey string) prepTaskS {
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
				pt := prepTaskRec{PKey: pKey, AId: p.AId, Type: 'P', time: pp.Time, Text: pp.text, Verbal: pp.verbal, taskp: pp}
				ptS = append(ptS, pt)
			}
		}
	}
	sort.Sort(ptS)
	//
	// generate Task Ids
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
			pt := prepTaskRec{PKey: pKey, SortK: i, AId: p.AId, Type: 'P', time: pp.Time, Text: pp.text, Verbal: pp.verbal, taskp: pp}
			ptS = append(ptS, pt)
			i++
		}
	}
	// now for all WaitOn prep tasks
	for p := prepctl.start; p != nil; p = p.nextPrep {
		for ia, pp := range p.Prep {
			if _, ok := processed[atvTask{p.AId, ia}]; ok {
				continue
			}
			pt := prepTaskRec{PKey: pKey, SortK: i, AId: p.AId, Type: 'P', time: pp.Time, Text: pp.text, Verbal: pp.verbal, taskp: pp}
			ptS = append(ptS, pt)
			i++
		}
	}
	//
	// append tasks
	//
	for p := taskctl.start; p != nil; p = p.nextTask {
		for _, pp := range p.Task {
			pt := prepTaskRec{PKey: pKey, SortK: i, AId: p.AId, Type: 'T', time: pp.Time, Text: pp.text, Verbal: pp.verbal, taskp: pp}
			ptS = append(ptS, pt)
			i++
		}
	}
	// now that we know the size of the list assign End-Of-List field. This approach replaces MaxId[] set stored in Recipe table
	// this mean each record knows how long the list is - helpful in a stateless Lambda app.
	eol := len(ptS)
	for i := range ptS {
		ptS[i].EOL = eol
	}
	// store number of records in recipe table
	return ptS
}

func (a Activities) PrintRecipe(rId string) (prepTaskS, string) {
	//
	var ptS prepTaskS
	pid := 0                                     // index in prepOrder
	processed := make(map[int]bool, prepctl.cnt) // set of tasks
	//
	// sort parallelisable prep tasks
	//
	for p := prepctl.start; p != nil; p = p.nextPrep {
		var add bool
		for _, pp := range p.Prep {
			if pp.UseDevice != nil {
				if strings.ToLower(pp.UseDevice.Type) == "oven" {
					add = true
				}

				if pp.Parallel && !pp.Link || add {
					if p.prev != nil && len(p.prev.Prep) != 0 {
						if p.prev.Prep[len(p.prev.Prep)-1].Link {
							continue // exclude if part of linked activity in last prep task of previous activity
						}
					}
					processed[p.AId] = true
					pt := prepTaskRec{time: pp.Time, Text: pp.text}
					ptS = append(ptS, pt)
				}
			}
		}
	}
	sort.Sort(ptS)
	//
	// append remaining prep tasks - these are serial tasks so order unimportant
	//
	for p := prepctl.start; p != nil; p = p.nextPrep {
		if _, ok := processed[p.AId]; ok {
			continue
		}
		for _, pp := range p.Prep {
			var txt string
			var stime float32
			var count int
			if pp.Link {
				for ; pp.Link; p = p.nextPrep {
					//handle Link prep tasks. Link tasks can only have a single prep task per activity
					txt += p.Prep[0].text + " and "
					stime += p.Prep[0].Time
					count++
				}
				txt += pp.text
				stime += pp.Time
				//
				pt := prepTaskRec{time: stime, Text: txt}
				ptS = append(ptS, pt)
			} else {
				pt := prepTaskRec{time: pp.Time, Text: pp.text}
				ptS = append(ptS, pt)
			}
			pid++
		}
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("{ %q : [", jsonKey))
	for i, pt := range ptS {
		b.WriteString(fmt.Sprintf("%q", pt.Text))
		if i < len(ptS)-1 {
			b.WriteString(",")
		}
	}
	b.WriteString("] } ")
	return ptS, b.String()
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
