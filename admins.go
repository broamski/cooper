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
	Username    string `form:"username" json:"username" binding:"required"`
	GlobalAdmin bool   `form:"global_admin" json:"global_admin"`
}

// GetAdmin returns an AdminUser when provided a user ID
func GetAdmin(svc *dynamodb.DynamoDB, uid string) (AdminUser, error) {
	resp, err := svc.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String(portalAdmins.TableName),
		Key: map[string]*dynamodb.AttributeValue{
			"username": {
				S: aws.String(uid),
			},
		},
	})
	if err != nil {
		return AdminUser{}, err
	}
	t := AdminUser{}
	err = dynamodbattribute.UnmarshalMap(resp.Item, &t)
	if err != nil {
		return AdminUser{}, err
	}
	return t, nil
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
	item, err := dynamodbattribute.MarshalMap(adminuser)
	if err != nil {
		return err
	}
	_, err = svc.PutItem(&dynamodb.PutItemInput{
		ConditionExpression: aws.String("attribute_not_exists(username)"),
		Item:                item,
		TableName:           aws.String(portalAdmins.TableName),
	})
	if err != nil {
		return err
	}
	log.Println("added administrative user:", adminuser)
	return nil
}

// RemoveAdmin removes an administrator from the system
func RemoveAdmin(svc *dynamodb.DynamoDB, adminuser AdminUser) error {
	item, err := dynamodbattribute.MarshalMap(adminuser)
	if err != nil {
		return err
	}
	// remove GlobalAdmin from map, so it can match the key schema and be deleted
	delete(item, "global_admin")

	_, err = svc.DeleteItem(&dynamodb.DeleteItemInput{
		ConditionExpression: aws.String("attribute_exists(username)"),
		Key:                 item,
		TableName:           aws.String(portalAdmins.TableName),
	})
	if err != nil {
		return err
	}
	log.Println("removed administrative user:", adminuser)
	return nil
}
