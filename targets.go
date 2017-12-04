package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
)

// Target defines what can be assumed in an AWS account
type Target struct {
	ID                   string `dynamodbav:"target_id" form:"target_id" json:"target_id"`
	Name                 string `form:"target_name" json:"target_name" binding:"required"`
	ARN                  string `form:"target_arn" json:"target_arn" binding:"required"`
	Type                 string `form:"target_type" json:"target_type" binding:"required"`
	ExternalID           string `form:"target_external_id" json:"target_external_id"`
	FederatedCredentials string `form:"target_fed_creds" json:"target_fed_creds"`
	GroupMapping         string `form:"target_group_mapping" json:"target_group_mapping"`
}

// TargetInvalid checks to see if a proper target type is being provided
func (t Target) TargetInvalid() bool {
	switch t.Type {
	case
		"role",
		"user":
		return false
	}
	return true
}

func (t Target) getAccountNumber() string {
	// example ARNs
	// arn:aws:iam::123456789012:role/S3Access
	// arn:aws:sts::123456789012:user/Bobo
	splitARN := strings.Split(t.ARN, ":")
	if len(splitARN) != 6 {
		return ""
	}
	return splitARN[4]
}

// GetTargets provides a full list of available targets
func GetTargets(svc *dynamodb.DynamoDB) ([]Target, error) {
	items := []Target{}
	ai := make([]map[string]*dynamodb.AttributeValue, 0)

	params := &dynamodb.ScanInput{
		TableName: aws.String(portalTargets.TableName),
	}
	resp, err := svc.Scan(params)
	if err != nil {
		return []Target{}, err
	}

	ai = append(ai, resp.Items...)

	// fetch additional items if the scan return limit is met
	for len(resp.LastEvaluatedKey) > 0 {
		fmt.Println("max number of results returned, processing next batch")
		params := &dynamodb.ScanInput{
			TableName:         aws.String(portalTargets.TableName),
			ExclusiveStartKey: resp.LastEvaluatedKey,
		}
		resp, err = svc.Scan(params)
		if err != nil {
			return []Target{}, err
		}
		ai = append(ai, resp.Items...)
	}
	err = dynamodbattribute.UnmarshalListOfMaps(ai, &items)
	if err != nil {
		return []Target{}, err
	}

	return items, nil
}

// GetTarget returns a target using a supplid target id
func GetTarget(svc *dynamodb.DynamoDB, tid string) (Target, error) {
	resp, err := svc.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String(portalTargets.TableName),
		Key: map[string]*dynamodb.AttributeValue{
			"target_id": {
				S: aws.String(tid),
			},
		},
	})
	if err != nil {
		return Target{}, err
	}
	t := Target{}
	err = dynamodbattribute.UnmarshalMap(resp.Item, &t)
	if err != nil {
		return Target{}, err
	}
	return t, nil
}

// AddTarget adds a new user provided target
func AddTarget(svc *dynamodb.DynamoDB, target Target) error {
	if target.TargetInvalid() {
		return fmt.Errorf("target type of %v is invalid", target.Type)
	}

	item, err := dynamodbattribute.MarshalMap(target)
	if err != nil {
		return err
	}
	_, err = svc.PutItem(&dynamodb.PutItemInput{
		ConditionExpression: aws.String("attribute_not_exists(target_id)"),
		Item:                item,
		TableName:           aws.String(portalTargets.TableName),
	})
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case "ConditionalCheckFailedException":
				return fmt.Errorf("the target %+v already exists", target)
			}
		}
		return err
	}
	log.Println(fmt.Sprintf("added target: %+v", target))
	return nil
}

func UpdateTarget(svc *dynamodb.DynamoDB, target Target) error {
	if target.TargetInvalid() {
		return fmt.Errorf("target type of %v is invalid", target.Type)
	}

	item, err := dynamodbattribute.MarshalMap(target)
	if err != nil {
		return err
	}
	key := make(map[string]*dynamodb.AttributeValue)
	key["target_id"] = item["target_id"]
	delete(item, "target_id")
	// convert to dynamodb.AttributeValueUpdate because there is no unmarshal function
	updates := make(map[string]*dynamodb.AttributeValueUpdate)
	for k, v := range item {
		updates[k] = &dynamodb.AttributeValueUpdate{
			Action: aws.String("PUT"),
			Value:  v,
		}
	}
	fmt.Println(updates)
	//ConditionExpression: aws.String("attribute_exists(target_id)"),
	_, err = svc.UpdateItem(&dynamodb.UpdateItemInput{
		Key:              key,
		AttributeUpdates: updates,
		TableName:        aws.String(portalTargets.TableName),
	})
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case "ConditionalCheckFailedException":
				return fmt.Errorf("the target %+v doesn't exist, so i can't update it", target)
			}
		}
		return err
	}
	log.Println(fmt.Sprintf("updated target: %+v", target))
	return nil
}

// RemoveTarget removes a target using a provided target id
func RemoveTarget(svc *dynamodb.DynamoDB, tid string) error {
	z, _ := GetTarget(svc, tid)

	_, err := svc.DeleteItem(&dynamodb.DeleteItemInput{
		ConditionExpression: aws.String("attribute_exists(target_id)"),
		Key: map[string]*dynamodb.AttributeValue{
			"target_id": {
				S: aws.String(tid),
			},
		},
		TableName: aws.String(portalTargets.TableName),
	})
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case "ConditionalCheckFailedException":
				return fmt.Errorf("the target %v does not exist", tid)
			}
		}
		return err
	}

	log.Println(fmt.Sprintf("removed target %+v", z))
	return nil
}

// An Association type represents a user associated to a target
type Association struct {
	Username      string `dynamodbav:"username"`
	AssociationID string `dynamodbav:"assoc_id"`
}

// type TargetsMapping struct {
// 	TargetId string `dynamodbav:"target_id"`
// 	GroupId  string `dynamodbav:"group_id"`
// }

type TargetsDetailed struct {
	Assoc         Target
	AccountNumber string
}

type AssociationDetailed struct {
	Assoc         Association
	TargetName    string
	AccountNumber string
}

// GetAssociations returns all of the associations for a particular user
func GetAssociations(svc *dynamodb.DynamoDB, groups []string) ([]TargetsDetailed, error) {
	i := []Target{}
	ai := make([]map[string]*dynamodb.AttributeValue, 0)

	// this is a bad name
	newnew := make([]TargetsDetailed, 0, 0)

	// for every group, get the target_id
	for _, group := range groups {
		params := &dynamodb.ScanInput{
			TableName:        aws.String(portalTargets.TableName),
			FilterExpression: aws.String("target_group_mapping = :gid"),
			ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
				":gid": {
					S: aws.String(group),
				},
			},
		}
		resp, err := svc.Scan(params)
		if err != nil {
			return newnew, err
		}
		if len(resp.Items) > 0 {
			ai = append(ai, resp.Items...)
		}

		// if the scan is truncated, continue the scan
		for len(resp.LastEvaluatedKey) > 0 {
			fmt.Println("max number of results returned, processing next batch")
			params := &dynamodb.ScanInput{
				TableName:        aws.String(portalTargets.TableName),
				FilterExpression: aws.String("target_group_mapping = :gid"),
				ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
					":tid": {
						S: aws.String(group),
					},
				},
			}
			resp, err = svc.Scan(params)
			if err != nil {
				return newnew, err
			}
			if len(resp.Items) > 0 {
				ai = append(ai, resp.Items...)
			}
		}
		err = dynamodbattribute.UnmarshalListOfMaps(ai, &i)
		if err != nil {
			return newnew, err
		}
	}
	for _, v := range i {
		tgt, _ := GetTarget(svc, v.ID)
		newnew = append(newnew, TargetsDetailed{
			Assoc:         v,
			AccountNumber: tgt.getAccountNumber(),
		})
	}
	return newnew, nil
}

// AllowedToBecome determines if the requesting user is in a group that is associated
// with the requested target
func AllowedToBecome(svc *dynamodb.DynamoDB, groups []string, targetId string) (bool, error) {
	// take the provided target_id, get its associated group_id
	resp, err := ddb.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String(portalTargets.TableName),
		Key: map[string]*dynamodb.AttributeValue{
			"target_id": {
				S: aws.String(targetId),
			},
		},
	})
	if err != nil {
		log.Println(err)
		return false, err
	}
	if len(resp.Item) == 0 {
		return false, nil
	}
	i := Target{}
	err = dynamodbattribute.UnmarshalMap(resp.Item, &i)
	if err != nil {
		return false, err
	}

	if Contains(groups, i.GroupMapping) {
		return true, nil
	}
	return false, nil
}
