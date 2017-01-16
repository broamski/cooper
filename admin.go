package main

import (
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
)

// AdminUser defines what an administrative user looks like
type AdminUser struct {
	Username string `json:"username"`
}

// GetAdmins returns a slice of all administrators, it implements
// pagination in the event the result set is too large to return
// with a single scan
func GetAdmins(svc *dynamodb.DynamoDB) ([]AdminUser, error) {
	items := []AdminUser{}
	ai := make([]map[string]*dynamodb.AttributeValue, 0)

	params := &dynamodb.ScanInput{
		TableName: aws.String(portalAdmins.TableName),
	}
	resp, err := svc.Scan(params)
	if err != nil {
		return []AdminUser{}, err
	}

	ai = append(ai, resp.Items...)

	// fetch additional items if the scan return limit is met
	for len(resp.LastEvaluatedKey) > 0 {
		fmt.Println("max number of results returned, processing next batch")
		params := &dynamodb.ScanInput{
			TableName:         aws.String(portalAdmins.TableName),
			ExclusiveStartKey: resp.LastEvaluatedKey,
		}
		resp, err = svc.Scan(params)
		if err != nil {
			return []AdminUser{}, err
		}
		ai = append(ai, resp.Items...)
	}
	err = dynamodbattribute.UnmarshalListOfMaps(ai, &items)
	if err != nil {
		return []AdminUser{}, err
	}

	return items, nil
}

// AddAdmin adds an administrator to the system
func AddAdmin(svc *dynamodb.DynamoDB, adminuser AdminUser) error {
	log.Println("adding administrative user:", adminuser)

	item, err := dynamodbattribute.MarshalMap(adminuser)
	if err != nil {
		return err
	}
	_, err = svc.PutItem(&dynamodb.PutItemInput{
		Item:      item,
		TableName: aws.String(portalAdmins.TableName),
	})
	if err != nil {
		return err
	}
	return nil
}

// RemoveAdmin removes an administrator from the system
func RemoveAdmin(svc *dynamodb.DynamoDB, adminuser AdminUser) error {
	log.Println("removing administrative user:", adminuser)

	item, err := dynamodbattribute.MarshalMap(adminuser)
	if err != nil {
		return err
	}
	_, err = svc.DeleteItem(&dynamodb.DeleteItemInput{
		Key:       item,
		TableName: aws.String(portalAdmins.TableName),
	})
	if err != nil {
		return err
	}
	return nil
}