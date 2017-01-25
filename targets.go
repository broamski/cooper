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
	ID         string `dynamodbav:"target_id" form:"target_id" json:"target_id"`
	Name       string `form:"target_name" json:"target_name" binding:"required"`
	ARN        string `form:"target_arn" json:"target_arn" binding:"required"`
	Type       string `form:"target_type" json:"target_type" binding:"required"`
	ExternalID string `form:"target_external_id" json:"target_external_id"`
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
	// arn:aws:sts::123456789012:federated-user/Bobo
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

	// ------

	// scan for assocations ties to this target, to eventually delete
	ai := make([]map[string]*dynamodb.AttributeValue, 0)
	params := &dynamodb.ScanInput{
		TableName:        aws.String(portalUserAssc.TableName),
		FilterExpression: aws.String("assoc_id = :tid"),
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":tid": {
				S: aws.String(tid),
			},
		},
	}
	resp, err := svc.Scan(params)
	ai = append(ai, resp.Items...)

	for len(resp.LastEvaluatedKey) > 0 {
		fmt.Println("max number of results returned, processing next batch")
		params := &dynamodb.ScanInput{
			TableName:        aws.String(portalUserAssc.TableName),
			FilterExpression: aws.String("assoc_id = :tid"),
			ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
				":tid": {
					S: aws.String(tid),
				},
			},
		}
		resp, err = svc.Scan(params)
		ai = append(ai, resp.Items...)
	}
	fmt.Println(ai)
	for _, v := range ai {
		_, err = svc.DeleteItem(&dynamodb.DeleteItemInput{
			ConditionExpression: aws.String("attribute_exists(username)"),
			Key:                 v,
			TableName:           aws.String(portalUserAssc.TableName),
		})
	}

	log.Println(fmt.Sprintf("removed target %+v", z))
	return nil
}

// An Association type represents a user associated to a target
type Association struct {
	Username      string `dynamodbav:"username"`
	AssociationID string `dynamodbav:"assoc_id"`
}

type AssociationDetailed struct {
	Assoc         Association
	TargetName    string
	AccountNumber string
}

// GetAssociations returns all of the associations for a particular user
func GetAssociations(svc *dynamodb.DynamoDB, uid string) ([]AssociationDetailed, error) {
	t := []Association{}
	newnew := make([]AssociationDetailed, 0, len(t))
	ai := make([]map[string]*dynamodb.AttributeValue, 0)
	resp, err := svc.Query(&dynamodb.QueryInput{
		TableName:              aws.String(portalUserAssc.TableName),
		KeyConditionExpression: aws.String("username = :uid"),
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":uid": {
				S: aws.String(uid),
			},
		},
	})
	if err != nil {
		return newnew, err
	}
	ai = append(ai, resp.Items...)

	// fetch additional items if the scan return limit is met
	for len(resp.LastEvaluatedKey) > 0 {
		fmt.Println("max number of results returned, processing next batch")
		resp, err = svc.Query(&dynamodb.QueryInput{
			TableName:              aws.String(portalUserAssc.TableName),
			KeyConditionExpression: aws.String("username = :uid"),
			ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
				":uid": {
					S: aws.String(uid),
				},
			},
			ExclusiveStartKey: resp.LastEvaluatedKey,
		})
		if err != nil {
			return newnew, err
		}
		ai = append(ai, resp.Items...)
	}
	err = dynamodbattribute.UnmarshalListOfMaps(ai, &t)
	if err != nil {
		return newnew, err
	}

	// populate the association with target details
	//newnew := make([]AssociationDetailed, 0, len(t))
	for _, v := range t {
		// should we catch this error?
		t, _ := GetTarget(svc, v.AssociationID)
		newnew = append(newnew, AssociationDetailed{
			Assoc:         v,
			TargetName:    t.Name,
			AccountNumber: t.getAccountNumber(),
		})
	}
	fmt.Println(newnew)
	return newnew, nil
}

// IsAssociated determines if a particular user is associated with a target
func IsAssociated(svc *dynamodb.DynamoDB, uid string, tid string) (bool, error) {
	resp, err := svc.Query(&dynamodb.QueryInput{
		TableName:              aws.String(portalUserAssc.TableName),
		KeyConditionExpression: aws.String("username = :uid AND assoc_id = :tid"),
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":uid": {
				S: aws.String(uid),
			},
			":tid": {
				S: aws.String(tid),
			},
		},
	})
	if err != nil {
		return false, err
	}

	if *resp.Count == 1 {
		return true, nil
	}
	return false, nil
}

// AssociateTarget associates a user to a target
func AssociateTarget(svc *dynamodb.DynamoDB, uid string, tid string) error {
	// uid and tid will probalby be provided as a json blog, maybe these
	// inputs can change to type Association

	var assoc = Association{
		Username:      uid,
		AssociationID: tid,
	}
	// get target and see if it exists
	z, _ := GetTarget(svc, tid)
	if z == (Target{}) {
		return fmt.Errorf("the target %v does not exist", tid)
	}

	item, err := dynamodbattribute.MarshalMap(assoc)
	if err != nil {
		return err
	}
	_, err = svc.PutItem(&dynamodb.PutItemInput{
		ConditionExpression: aws.String("attribute_not_exists(username)"),
		Item:                item,
		TableName:           aws.String(portalUserAssc.TableName),
	})
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case "ConditionalCheckFailedException":
				return fmt.Errorf("this assocation already exists: %+v", assoc)
			}
		}
		return err
	}
	log.Println(fmt.Sprintf("associated %v to: %+v", uid, z))
	return nil
}

// DisassociateTarget disassociates a user from a target
func DisassociateTarget(svc *dynamodb.DynamoDB, uid string, tid string) error {
	z, _ := GetTarget(svc, tid)
	if z == (Target{}) {
		return fmt.Errorf("the target %v does not exist", tid)
	}
	i, _ := IsAssociated(svc, uid, tid)
	if i {
		var assoc = Association{
			Username:      uid,
			AssociationID: tid,
		}

		item, err := dynamodbattribute.MarshalMap(assoc)
		if err != nil {
			return err
		}
		_, err = svc.DeleteItem(&dynamodb.DeleteItemInput{
			ConditionExpression: aws.String("attribute_exists(username)"),
			Key:                 item,
			TableName:           aws.String(portalUserAssc.TableName),
		})
		if err != nil {
			return err
		}
		log.Println("dissassociated user", uid, "from target", z)
		return nil
	}
	return fmt.Errorf("user %v is not associated with target %v", uid, z)
}

type userlist struct {
	Username string `dynamodbav:"username"`
}

func GetTargetUsers(svc *dynamodb.DynamoDB, tid string) ([]userlist, error) {
	items := []userlist{}
	ai := make([]map[string]*dynamodb.AttributeValue, 0)

	params := &dynamodb.ScanInput{
		TableName:        aws.String(portalUserAssc.TableName),
		FilterExpression: aws.String("assoc_id = :tid"),
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":tid": {
				S: aws.String(tid),
			},
		},
	}
	resp, err := svc.Scan(params)
	ai = append(ai, resp.Items...)
	if err != nil {
		return []userlist{}, err
	}

	for len(resp.LastEvaluatedKey) > 0 {
		fmt.Println("max number of results returned, processing next batch")
		params := &dynamodb.ScanInput{
			TableName:        aws.String(portalUserAssc.TableName),
			FilterExpression: aws.String("assoc_id = :tid"),
			ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
				":tid": {
					S: aws.String(tid),
				},
			},
		}
		resp, err = svc.Scan(params)
		if err != nil {
			return []userlist{}, err
		}
		ai = append(ai, resp.Items...)
	}
	dynamodbattribute.UnmarshalListOfMaps(ai, &items)
	err = dynamodbattribute.UnmarshalListOfMaps(ai, &items)
	if err != nil {
		return []userlist{}, err
	}
	fmt.Println(items)
	return items, nil

}
