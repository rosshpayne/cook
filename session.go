package main

import (
	"fmt"
	_ "time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	_ "github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/dynamodb/expression"

	_ "github.com/aws/aws-lambda-go/lambdacontext"
)

// Session table record
// only items from session context that need to be preserved between sessions are persisted.
type stateRec struct {
	// InputRequest fields
	Path    string // inputEvent.Path
	Param   string
	Request string // PathItem[0]:
	//
	Obj     string // Object - to which operation (listing) apply
	BkId    string // Book Id
	BkName  string // Book name - saves a lookup under some circumstances
	RName   string // Recipe name - saves a lookup under some circumstances
	Serves  string
	SwpBkNm string
	SwpBkId string
	RId     string // Recipe Id
	Qid     int    // Question id
	RecId   []int  // current record in object list. (SortK in Recipe table)
	Ver     string
	EOL     int // last RecId of current list. Used to determine when last record is reached or exceeded in the case of goto operation
	Dmsg    string
	Vmsg    string
	DData   string
	//
	// search
	//
	Search  string
	MChoice []mRecipeT
	//
	// select
	//
	SelCtx selectCtxT // select a recipe or other which is (ingred, task, container, utensil)
	SelId  int        // select choice
	//
	// Recipe Part related data
	//
	Parts []*PartT `json:"Parts"` // PartT.Idx - short name for Recipe Part
	Part  string   `json:"Part"`
	Next  int      `json:"Next"`
	Prev  int      `json:"Prev"`
	PEOL  int      `json:"PEOL"`
	PId   int      `json:"PId"`
	//
	// Display header for APL - NB: this is probably not required as display will persist last assigned value.
	//
	//dispHdr  string `json:"DHdr"`  // Display header
	//dispSHdr string `json:"DsHdr"` // Display subheader
}

type stateStack []stateRec

func (s stateStack) pop() *stateRec {
	st := s[len(s)-1]
	return &st
}

func (s *sessCtx) getState() (*stateRec, error) {
	//
	// Table:  Sessions
	//
	type pKey struct {
		Sid string
	}
	fmt.Println("entered getState..")
	pkey := pKey{s.sessionId}
	av, err := dynamodbattribute.MarshalMap(&pkey)
	input := &dynamodb.GetItemInput{
		Key:       av,
		TableName: aws.String("Sessions"),
	}
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
		return nil, err
	}
	if len(result.Item) == 0 {
		//
		s.recId = []int{0, 0, 0, 0} // initial record ids. This data will be retrieved, updated and saved on each request involing navigation across a object list.
		sr, err := s.saveState(true)
		return sr, err
	}
	//
	type stateItemT struct {
		Sid   string     `json:"Sid"`
		State stateStack `json:"state"`
	}
	stateItem := stateItemT{}
	err = dynamodbattribute.UnmarshalMap(result.Item, &stateItem)
	if err != nil {
		return nil, err
	}
	//return staterow.state.pop(), nil
	s.state = stateItem.State
	return stateItem.State.pop(), nil
}

func (s *sessCtx) saveState(new ...bool) (*stateRec, error) {

	type pKey struct {
		Sid string
	}

	var (
		sr      stateRec
		updateC expression.UpdateBuilder
	)
	fmt.Println("Entered saveState..", new)
	sr.Path = s.path
	sr.Param = s.param

	sr.RId = s.reqRId       // Recipe Id
	sr.BkId = s.reqBkId     // Book Id
	sr.BkName = s.reqBkName // Book name - saves a lookup under some circumstances
	sr.RName = s.reqRName   // Recipe name - saves a lookup under some circumstances
	sr.SwpBkNm = s.swapBkName
	sr.SwpBkId = s.swapBkId
	sr.Request = s.request // Request e.g.next, prev, repeat, modify)
	sr.Serves = s.serves
	sr.Qid = s.questionId // Question id	for k,v:=range objectMap {
	sr.Obj = s.object     // Object - to which operation (listing) apply

	if s.reset {
		sr.RecId = []int{0, 0, 0, 0}
	} else {
		sr.RecId = s.recId //s.recId     // current record in object list. (SortK in Recipe table)
	}
	sr.Ver = s.reqVersion
	sr.EOL = s.eol // last RecId of current list. Used to determine when last record is reached or exceeded in the case of goto operation
	sr.Dmsg = s.dmsg
	sr.Vmsg = s.vmsg
	sr.DData = s.ddata
	//
	// Record id across objects
	//
	sr.RecId = s.recId
	// search
	//
	sr.Search = s.reqSearch
	sr.MChoice = s.mChoice
	//
	// select
	//
	sr.SelCtx = s.selCtx // select a recipe or other which is (ingred, task, container, utensil)
	sr.SelId = s.selId   //
	//
	// Recipe Part related data
	//
	sr.Parts = s.parts
	sr.Part = s.part
	sr.Next = s.next
	sr.Prev = s.prev
	sr.PEOL = s.peol
	sr.PId = s.pid
	//
	State := make(stateStack, 1)
	State[0] = sr
	if new != nil {
		if new[0] {
			updateC = expression.Set(expression.Name("state"), expression.Value(State))
		}
	} else {
		updateC = expression.Set(expression.Name("state"), expression.ListAppend(expression.Name("state"), expression.Value(State)))
	}
	expr, err := expression.NewBuilder().WithUpdate(updateC).Build()
	if err != nil {
		return nil, err
	}

	pkey := pKey{Sid: s.sessionId}
	av, err := dynamodbattribute.MarshalMap(&pkey)

	input := &dynamodb.UpdateItemInput{
		TableName:                 aws.String("Sessions"),
		Key:                       av, // accets []map[]*attributeValues so must use marshal not expression
		UpdateExpression:          expr.Update(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
	}
	_, err = s.dynamodbSvc.UpdateItem(input)
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
		return nil, err
	}
	fmt.Println("Saved. Exiting saveState..", new)
	return &sr, nil
}

func (s *sessCtx) updateState() error {

	type pKey struct {
		Sid string
	}

	var sr stateRec
	var updateC expression.UpdateBuilder
	//
	// update RecId attribute of latest state item
	//
	recid_ := fmt.Sprintf("state[%d].RecId", len(s.state)-1)
	updateC = expression.Set(expression.Name(recid_), expression.Value(sr.RecId))
	expr, err := expression.NewBuilder().WithUpdate(updateC).Build()

	pkey := pKey{Sid: s.sessionId}
	av, err := dynamodbattribute.MarshalMap(&pkey)

	input := &dynamodb.UpdateItemInput{
		TableName:                 aws.String("Sessions"),
		Key:                       av, // accets []map[]*attributeValues so must use marshal not expression
		UpdateExpression:          expr.Update(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		ReturnValues:              aws.String("UPDATED_NEW"),
	}
	_, err = s.dynamodbSvc.UpdateItem(input)
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

func (s *sessCtx) popState() error {
	//
	// removes last entry in state attribute of session item.
	//  populates s (sessCtx) with state data from the new last entry
	//  (which was the second last entry before deletion)
	//
	type pKey struct {
		Sid string
	}
	var updateC expression.UpdateBuilder
	// get current state if not already sourced
	if len(s.state) == 0 {
		s.getState()
	}
	if len(s.state) < 2 {
		return fmt.Errorf("Error: Cannot proceed back any further.")
	}
	// save all but last state entry
	State := s.state[:len(s.state)-1]
	s.state = State[:]
	updateC = expression.Set(expression.Name("state"), expression.Value(State))
	expr, err := expression.NewBuilder().WithUpdate(updateC).Build()
	//
	pkey := pKey{Sid: s.sessionId}
	av, err := dynamodbattribute.MarshalMap(&pkey)

	input := &dynamodb.UpdateItemInput{
		TableName:                 aws.String("Sessions"),
		Key:                       av, // accets []map[]*attributeValues so must use marshal not expression
		UpdateExpression:          expr.Update(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		ReturnValues:              aws.String("UPDATED_NEW"),
	}
	_, err = s.dynamodbSvc.UpdateItem(input)
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
	sr := State.pop()
	// transfer state to session context
	s.path = sr.Path
	s.param = sr.Param

	s.reqRId = sr.RId       // Recipe Id
	s.reqBkId = sr.BkId     // Book Id
	s.reqBkName = sr.BkName // Book name - saves a lookup under some circumstances
	s.reqRName = sr.RName   // Recipe name - saves a lookup under some circumstances
	s.swapBkName = sr.SwpBkNm
	s.swapBkId = sr.SwpBkId
	s.request = sr.Request // Request e.g.next, prev, repeat, modify)
	s.serves = sr.Serves
	s.questionId = sr.Qid // Question id	for k,v:=range objectMap {
	s.object = sr.Obj     // Object - to which operation (listing) apply
	s.recId = sr.RecId    //s.recId     // current record in object list. (SortK in Recipe table)
	s.reqVersion = sr.Ver
	s.eol = sr.EOL // last RecId of current list. Used to determine when last record is reached or exceeded in the case of goto operation
	s.dmsg = sr.Dmsg
	s.vmsg = sr.Vmsg
	s.ddata = sr.DData
	//
	// Record id across objects
	//
	s.recId = sr.RecId
	// search
	//
	s.reqSearch = sr.Search
	s.mChoice = sr.MChoice
	//
	// select
	//
	s.selCtx = sr.SelCtx // select a recipe or other which is (ingred, task, container, utensil)
	s.selId = sr.SelId   //
	//
	// Recipe Part related data
	//
	s.parts = sr.Parts
	s.part = sr.Part
	s.next = sr.Next
	s.prev = sr.Prev
	s.peol = sr.PEOL
	s.pid = sr.PId
	return nil
}
