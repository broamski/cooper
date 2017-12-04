package main

import (
	"log"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb"
)

// AddAdmin adds an administrator to the system
func AddAdminMapping(ddb *dynamodb.DynamoDB, adminGroup string) error {
	input := &dynamodb.PutItemInput{
		ConditionExpression: aws.String("attribute_not_exists(item)"),
		Item: map[string]*dynamodb.AttributeValue{
			"item": {
				S: aws.String("admins_group"),
			},
			"value": {
				S: aws.String(adminGroup),
			},
		},
		TableName: aws.String(portalValues.TableName),
	}
	_, err := ddb.PutItem(input)

	if err != nil {
		return err
	}
	log.Println("added administrative grou mapping for :", adminGroup)
	return nil
}
