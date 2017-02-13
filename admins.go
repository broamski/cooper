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

func (a AdminUser) globalAdmin() bool {
	return a.GlobalAdmin
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
func RemoveAdmin(ddb *dynamodb.DynamoDB, a string) error {
	admin, err := GetAdmin(ddb, a)
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
	// remove any admin associations for non-global admins
	// as a global admin should never have any associations
	if !admin.GlobalAdmin {
		asc, err := GetAdminAssociations(ddb, a)
		if err != nil {
			return nil
		}
		for _, v := range asc {
			DisassociateAdmin(ddb, v.Username, v.AccountNumber)
		}
	}
	log.Println("removed administrative user:", a)
	return nil
}

func GetAdminAssociations(ddb *dynamodb.DynamoDB, username string) ([]AdminAssociation, error) {
	items := []AdminAssociation{}
	i := make([]map[string]*dynamodb.AttributeValue, 0)
	resp, err := ddb.Query(&dynamodb.QueryInput{
		TableName:              aws.String(portalAdminsAssc.TableName),
		KeyConditionExpression: aws.String("username = :uid"),
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":uid": {
				S: aws.String(username),
			},
		},
	})
	if err != nil {
		return items, err
	}
	i = append(i, resp.Items...)

	// fetch additional items if the scan return limit is met
	for len(resp.LastEvaluatedKey) > 0 {
		fmt.Println("max number of results returned, processing next batch")
		resp, err := ddb.Query(&dynamodb.QueryInput{
			TableName:              aws.String(portalAdminsAssc.TableName),
			KeyConditionExpression: aws.String("username = :uid"),
			ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
				":uid": {
					S: aws.String(username),
				},
			},
		})
		if err != nil {
			return items, err
		}
		i = append(i, resp.Items...)
	}

	// Unmarshal the Items field in the result value to the Item Go type.
	err = dynamodbattribute.UnmarshalListOfMaps(i, &items)
	if err != nil {
		return items, err
	}
	return items, nil
}

type AdminAssociation struct {
	Username      string `dynamodbav:"username"`
	AccountNumber string `dynamodbav:"account_number"`
}

func AssociateAdmin(ddb *dynamodb.DynamoDB, adminuser AdminUser, accountNum string) error {
	a := AdminAssociation{adminuser.Username, accountNum}
	fmt.Println(a)
	item, err := dynamodbattribute.MarshalMap(a)
	if err != nil {
		return err
	}

	_, err = ddb.PutItem(&dynamodb.PutItemInput{
		ConditionExpression: aws.String("attribute_not_exists(username)"),
		Item:                item,
		TableName:           aws.String(portalAdminsAssc.TableName),
	})
	if err != nil {
		return err
	}
	log.Println("associated administrative user:", adminuser, "to", accountNum)
	return nil
}

func DisassociateAdmin(ddb *dynamodb.DynamoDB, admin string, accountNum string) error {
	_, err := ddb.DeleteItem(&dynamodb.DeleteItemInput{
		ConditionExpression: aws.String("attribute_exists(username)"),
		Key: map[string]*dynamodb.AttributeValue{
			"username": {
				S: aws.String(admin),
			},
			"account_number": {
				S: aws.String(accountNum),
			},
		},
		TableName: aws.String(portalAdminsAssc.TableName),
	})
	if err != nil {
		return err
	}
	return nil
}
