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

func (s sessCtx) updateSession() (int, error) {
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
			updateC = expression.Add(expression.Name(recid_), expression.Value(s.updateAdd))
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
		ReturnValues:              aws.String("UPDATED_NEW"), //aws.String("ALL_NEW"),
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
		return 1, err
	}
	//
	// RecId has been updated so copy new value to session context
	//
	lastSess := sessRecT{}
	if len(result.Attributes) > 0 && s.curreq != bookrecipe_ {
		err = dynamodbattribute.UnmarshalMap(result.Attributes, &lastSess)
		if err != nil {
			return 1, err
		}
		// NB: UPDATE_NEW in return values will return only updated elements in a slice/set
		//  In the case of SET all values are returned
		//	In the case of ADD only the changed element in the set is returned.
		switch len(lastSess.RecId) {
		case 1:
			return lastSess.RecId[0], nil
		default:
			return lastSess.RecId[objectMap[s.object]], nil
		}
	}
	return 1, nil
}
