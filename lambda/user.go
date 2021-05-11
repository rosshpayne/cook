package main

import (
	_ "encoding/json"
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

func (s *sessCtx) getUserBooks() ([]string, error) {

	type userT struct {
		Pkey  string
		SortK string
		BkIds []string `json:"bkid"` // book id slice
	}
	type pKey struct {
		PKey  string
		SortK string
	}
	fmt.Println("getUserBooks..")
	pkey := pKey{PKey: "U-" + s.userId, SortK: "1"}
	fmt.Printf("pkey: %3v\n", pkey)
	av, err := dynamodbattribute.MarshalMap(&pkey)
	if err != nil {
		return nil, fmt.Errorf("%s: %s", "Error in MarshalMap of recipeIdLookup", err.Error())
	}
	input := &dynamodb.GetItemInput{
		Key:       av,
		TableName: aws.String("Ingredient"),
	}
	input = input.SetTableName("Ingredient").SetReturnConsumedCapacity("TOTAL").SetConsistentRead(false)
	//
	result, err := s.dynamodbSvc.GetItem(input)
	if err != nil {
		return nil, fmt.Errorf("Error: %s [%s] %s", "in Query in getBookIds of ", s.reqBkId, err.Error())
	}
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
		return nil, fmt.Errorf("%s: %s", "Error in GetItem of recipeRSearch", err.Error())
	}
	if len(result.Item) == 0 {
		return nil, nil
	}
	rec := userT{}
	err = dynamodbattribute.UnmarshalMap(result.Item, &rec)
	if err != nil {
		return nil, fmt.Errorf("Error: in UnmarshalMaps of UserId [%s] err [%s]", s.userId, err.Error())
	}
	return rec.BkIds, nil
}

func (s *sessCtx) addUserBooks() error {
	// external systems has copy of userId along with an email address which must be the same as the email of the Alexa device, to which books are registered.
	//  if the email is different then what. A TODO.
	// passes all books (not just new one) that user has registered
	// alternativels passes any new books user has registered since last refresh. Code below presume first option.
	type pKey struct {
		Uid   string `json:"PKey"`
		SortK string
	}
	fmt.Println("in addUserBooks()")
	fmt.Printf("bkids %#v\n", s.bkids)
	fmt.Printf("uid = %s", s.userId)
	updateC := expression.Set(expression.Name("bkid"), expression.Value(s.bkids))
	updateC = updateC.Set(expression.Name("nbk"), expression.Value(len(s.bkids)))
	expr, err := expression.NewBuilder().WithUpdate(updateC).Build()
	if err != nil {
		return err
	}
	pkey := pKey{Uid: "U-" + s.userId, SortK: "1"}
	av, err := dynamodbattribute.MarshalMap(&pkey)
	input := &dynamodb.UpdateItemInput{
		TableName:                 aws.String("Ingredient"),
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
		return err
	}
	return nil
}
