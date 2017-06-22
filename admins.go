package main

import (
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
)

// Admin defines what an administrative user looks like
type Admin struct {
	Username string `form:"username" json:"username" binding:"required"`
}

// GetAdmin takes a user's ID and returns a boolean
// stating whether the user is an admin, along with their Admin struct
func GetAdmin(svc *dynamodb.DynamoDB, uid string) (bool, Admin, error) {
	fmt.Println("getting admin", uid, "from", portalAdmins.TableName)
	admin := Admin{}
	resp, err := svc.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String(portalAdmins.TableName),
		Key: map[string]*dynamodb.AttributeValue{
			"username": {
				S: aws.String(uid),
			},
		},
	})
	if err != nil {
		log.Println(err)
		return false, admin, err
	}

	if len(resp.Item) == 0 {
		return false, admin, nil
	}

	err = dynamodbattribute.UnmarshalMap(resp.Item, &admin)
	if err != nil {
		return false, admin, err
	}
	return true, admin, nil
}

// GetAdmins returns a slice of all administrators, it implements
// pagination in the event the result set is too large to return
// with a single scan
func GetAdmins(svc *dynamodb.DynamoDB) ([]Admin, error) {
	items := []Admin{}
	ai := make([]map[string]*dynamodb.AttributeValue, 0)

	params := &dynamodb.ScanInput{
		TableName: aws.String(portalAdmins.TableName),
	}
	resp, err := svc.Scan(params)
	if err != nil {
		return []Admin{}, err
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
			return []Admin{}, err
		}
		ai = append(ai, resp.Items...)
	}
	err = dynamodbattribute.UnmarshalListOfMaps(ai, &items)
	if err != nil {
		return []Admin{}, err
	}

	return items, nil
}

// AddAdmin adds an administrator to the system
func AddAdmin(svc *dynamodb.DynamoDB, adminuser Admin) error {
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
func RemoveAdmin(ddb *dynamodb.DynamoDB, a string) error {
	_, _, err := GetAdmin(ddb, a)
	if err != nil {
		return nil
	}

	_, err = ddb.DeleteItem(&dynamodb.DeleteItemInput{
		ConditionExpression: aws.String("attribute_exists(username)"),
		Key: map[string]*dynamodb.AttributeValue{
			"username": {
				S: aws.String(a),
			},
		},
		TableName: aws.String(portalAdmins.TableName),
	})
	if err != nil {
		return err
	}
	log.Println("removed administrative user:", a)
	return nil
}
