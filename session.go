package main

import (
	"fmt"
	"time"

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
type sessRecipeT struct {
	Obj     string // Object - to which operation (listing) apply
	BkId    string // Book Id
	BkName  string // Book name - saves a lookup under some circumstances
	RName   string // Recipe name - saves a lookup under some circumstances
	SwpBkNm string
	SwpBkId string
	RId     string // Recipe Id
	Oper    string // Operation (next, prev, repeat, modify)
	Qid     int    // Question id
	RecId   []int  // current record in object list. (SortK in Recipe table)
	Ver     string
	EOL     int // last RecId of current list. Used to determine when last record is reached or exceeded in the case of goto operation
	Dmsg    string
	Vmsg    string
	DData   string
	//SrchLst []mRecipeT
	RnLst  []mRecipeT
	Select bool
	//
	Part    []*PartT // PartT.Idx - short name for Recipe Part
	CurPart string
	Next    int `json:"NextR"`
	Prev    int `json:"PrevR"`
	PEOL    int `json:"PEOL"`
}

func (s *sessCtx) GetSession() (lastSess *sessRecipeT, err error) {
	//
	// Table:  Sessions
	//
	type pKey struct {
		Sid string
	}

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
		// *** no session data then ignore validating the session and insert it
		// session with what we've got in the session context
		// if s.curreq != bookrecipe_ {
		// 	s.dmsg = `You must specify a book and recipe from that book. To get started, please say "open", followed by the name of the book`
		// 	s.vmsg = `You must specify a book and recipe from that book. To get started, please say "open", followed by the name of the book`
		// 	s.abort = true
		// 	return nil, nil
		//
		return &sessRecipeT{}, nil
	}
	lastSess = &sessRecipeT{}
	err = dynamodbattribute.UnmarshalMap(result.Item, lastSess)
	if err != nil {
		return nil, err
	}
	return lastSess, nil
}

func (s sessCtx) updateSession() error {
	// state data that must be maintained across sessions
	//
	type pKey struct {
		Sid string
	}
	var updateC expression.UpdateBuilder
	//book-recipe requests don't need a RecId set.
	if s.curreq != bookrecipe_ || s.reset { // reset on change of book or recipe
		// for the first object request in a session the RecId set will not exist - we need to SET. All other times we will ADD.
		//  we determine the first time using a len(recID) > 0 on the session query in the calling func.
		if s.reset || s.recIdNotExists {
			s.recIdNotExists = false
			// on insert build a prepopulated dynamodb set of int (internally float64 in dynamodb)
			switch len(s.object) {
			case 0:
				updateC = expression.Set(expression.Name("RecId"), expression.Value([]int{0, 0, 0, 0, 0}))
			default:
				switch s.object {
				case ingredient_:
					updateC = expression.Set(expression.Name("RecId"), expression.Value([]int{1, 0, 0, 0, 0}))
				case task_:
					updateC = expression.Set(expression.Name("RecId"), expression.Value([]int{0, 1, 0, 0, 0}))
				case container_:
					updateC = expression.Set(expression.Name("RecId"), expression.Value([]int{0, 0, 1, 0, 0}))
				case utensil_:
					updateC = expression.Set(expression.Name("RecId"), expression.Value([]int{0, 0, 0, 1, 0}))
				case recipe_:
					updateC = expression.Set(expression.Name("RecId"), expression.Value([]int{0, 0, 0, 0, 1}))
				default:
					updateC = expression.Set(expression.Name("RecId"), expression.Value([]int{0, 0, 0, 0, 0}))
				}
			}
			t := time.Now()
			t.Add(time.Hour * 24 * 1)
			updateC = updateC.Set(expression.Name("Epoch"), expression.Value(t.Unix()))
		} else {
			// on update use ADD to increment an object related counter.
			recid_ := fmt.Sprintf("RecId[%d]", objectMap[s.object])
			//updateC = expression.Add(expression.Name(recid_), expression.Value(s.updateAdd))
			updateC = expression.Set(expression.Name(recid_), expression.Value(s.recId))
		}
	}

	updateC = updateC.Set(expression.Name("EOL"), expression.Value(s.eol)) //eol from get-RecId() associated with each Object

	if len(s.reqRName) > 0 {
		updateC = updateC.Set(expression.Name("Rname"), expression.Value(s.reqRName))
	} else {
		updateC = updateC.Set(expression.Name("Rname"), expression.Value(""))
	}
	if len(s.reqRId) > 0 {
		updateC = updateC.Set(expression.Name("RId"), expression.Value(s.reqRId))
	} else {
		updateC = updateC.Set(expression.Name("RId"), expression.Value(""))
	}
	// will clear Book entries provided execution paths bypasses mergeAndValidate func.
	if len(s.reqBkName) > 0 && s.reqBkName != "0" {
		updateC = updateC.Set(expression.Name("BKname"), expression.Value(s.reqBkName))
	} else if s.reqBkName != "0" {
		updateC = updateC.Set(expression.Name("BKname"), expression.Value(""))
	}
	if len(s.reqBkId) > 0 {
		updateC = updateC.Set(expression.Name("BkId"), expression.Value(s.reqBkId))
	} else {
		updateC = updateC.Set(expression.Name("BkId"), expression.Value(""))
	}
	if len(s.swapBkName) > 0 { //TODO - zeor Swp values when question 21 answered
		updateC = updateC.Set(expression.Name("SwpBkNm"), expression.Value(s.swapBkName))
		updateC = updateC.Set(expression.Name("SwpBkId"), expression.Value(s.swapBkId))
	}
	if len(s.operation) > 0 {
		updateC = updateC.Set(expression.Name("Oper"), expression.Value(s.operation)) // next,prev,repeat,modify,goto
	} else {
		updateC = updateC.Set(expression.Name("Oper"), expression.Value(""))
	}
	if len(s.object) > 0 {
		updateC = updateC.Set(expression.Name("Obj"), expression.Value(s.object)) // ingredient,task,container,utensil
	} else {
		updateC = updateC.Set(expression.Name("Obj"), expression.Value(""))
	}
	if s.questionId > 0 {
		updateC = updateC.Set(expression.Name("Qid"), expression.Value(s.questionId))
	} else {
		updateC = updateC.Set(expression.Name("Qid"), expression.Value(0))
	}
	if len(s.dbatchNum) > 0 {
		updateC = updateC.Set(expression.Name("DBat"), expression.Value(s.dbatchNum))
	}
	if len(s.mChoice) > 0 {
		updateC = updateC.Set(expression.Name("RnLst"), expression.Value(s.mChoice)) //recipename
	}
	if len(s.dmsg) > 0 {
		updateC = updateC.Set(expression.Name("Dmsg"), expression.Value(s.dmsg))
		updateC = updateC.Set(expression.Name("DData"), expression.Value(s.ddata))
	} else {
		updateC = updateC.Set(expression.Name("Dmsg"), expression.Value(""))
		updateC = updateC.Set(expression.Name("DData"), expression.Value(""))
	}
	if s.closeBook {
		updateC = updateC.Set(expression.Name("closeB"), expression.Value(true))
	} else {
		updateC = updateC.Set(expression.Name("closeB"), expression.Value(false))
	}
	if len(s.vmsg) > 0 {
		updateC = updateC.Set(expression.Name("Vmsg"), expression.Value(s.vmsg))
	} else {
		updateC = updateC.Set(expression.Name("Vmsg"), expression.Value(""))
	}
	if len(s.reqVersion) > 0 {
		updateC = updateC.Set(expression.Name("Ver"), expression.Value(s.reqVersion))
	} else {
		updateC = updateC.Set(expression.Name("Ver"), expression.Value(""))
	}
	//
	// Data related to Part mode ie. when using a recipe part. Values are blank if no part available or in no-part mode.
	//
	if len(s.parts) > 0 {
		updateC = updateC.Set(expression.Name("Parts"), expression.Value(s.parts))
	} else {
		updateC = updateC.Set(expression.Name("Parts"), expression.Value(""))
	}
	if len(s.part) > 0 {
		updateC = updateC.Set(expression.Name("Part"), expression.Value(s.part))
	} else {
		updateC = updateC.Set(expression.Name("Part"), expression.Value(""))
	}
	if s.peol > 0 {
		updateC = updateC.Set(expression.Name("PEOL"), expression.Value(s.peol))
	} else {
		updateC = updateC.Set(expression.Name("PEOL"), expression.Value(""))
	}
	//
	// NextR (nextRecord) and PrevR (PrevRecord) sourced from Recipe R- data (nxt,prv) for mode Part ie. when listing recipe by part.
	//  value is SortK
	//
	if s.next > 0 {
		updateC = updateC.Set(expression.Name("Next"), expression.Value(s.next))
	} else {
		updateC = updateC.Set(expression.Name("Next"), expression.Value(""))
	}
	if s.prev > 0 {
		updateC = updateC.Set(expression.Name("Prev"), expression.Value(s.prev))
	} else {
		updateC = updateC.Set(expression.Name("Prev"), expression.Value(""))
	}
	//
	//
	//
	updateC = updateC.Set(expression.Name("Select"), expression.Value(s.makeSelect)) // make a selection
	updateC = updateC.Set(expression.Name("ATime"), expression.Value(time.Now().String()))
	expr, err := expression.NewBuilder().WithUpdate(updateC).Build()

	pkey := pKey{Sid: s.sessionId}
	av, err := dynamodbattribute.MarshalMap(&pkey)

	input := &dynamodb.UpdateItemInput{
		TableName:                 aws.String("Sessions"),
		Key:                       av, // accets []map[]*attributeValues so must use marshal not expression
		UpdateExpression:          expr.Update(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		ReturnValues:              aws.String("UPDATED_NEW"), // //aws.String("UPDATED_NEW"), -
	}
	result, err := s.dynamodbSvc.UpdateItem(input) // do an updateitem and return original id value so only one call.
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
	//
	// RecId has been updated so copy new value to session context
	// TODO - maybe we should take control of RecId  (ie. not use ADD but SET instead)
	//
	currentSess := sessRecipeT{}
	if len(result.Attributes) > 0 && s.curreq != bookrecipe_ {
		err = dynamodbattribute.UnmarshalMap(result.Attributes, &currentSess)
		if err != nil {
			return err
		}
		// NB: UPDATE_NEW in return values will return only updated elements in a slice/set
		//  In the case of SET all values are returned
		//	In the case of ADD only the changed element in the set is returned.
		if len(currentSess.Obj) > 0 && s.recId > 0 {
			if currentSess.RecId[0] != s.recId {
				return fmt.Errorf("Error: in UpdateSession. Returned RecId does not match RecId used.")
			}
		}
		// if len(currentSess.CurPart) > 0 {
		// 	//TODO what about previous request from user. Here we presume only going forward
		// 	// TODO what about end ie. NextR = -1
		// 	return currentSess.NextR
		// }
		// switch len(currentSess.RecId) {
		// case 1:
		// 	return currentSess.RecId[0], nil
		// default:
		// 	return currentSess.RecId[objectMap[s.object]], nil
		// }
	}
	//TODO does this return get executed and if so is it the correct value
	if len(result.Attributes) == 0 {
		return fmt.Errorf("Error: zero attributes updated in updateSession")
	}
	return nil
}
