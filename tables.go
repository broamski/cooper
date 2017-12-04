package main

import (
	"log"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb"
)

// DDBTable may be referenced externally at some point
type DDBTable struct {
	TableName             string
	KeySchema             []*dynamodb.KeySchemaElement
	AttributeDefinitions  []*dynamodb.AttributeDefinition
	ProvisionedThroughput *dynamodb.ProvisionedThroughput
}

var portalValues = DDBTable{
	"cooper_portal",
	[]*dynamodb.KeySchemaElement{
		{
			AttributeName: aws.String("item"),
			KeyType:       aws.String("HASH"),
		},
	},
	[]*dynamodb.AttributeDefinition{
		{
			AttributeName: aws.String("item"),
			AttributeType: aws.String("S"),
		},
	},
	&dynamodb.ProvisionedThroughput{
		ReadCapacityUnits:  aws.Int64(1),
		WriteCapacityUnits: aws.Int64(1),
	},
}

var portalTargets = DDBTable{
	"cooper_portal_targets",
	[]*dynamodb.KeySchemaElement{
		{
			AttributeName: aws.String("target_id"),
			KeyType:       aws.String("HASH"),
		},
	},
	[]*dynamodb.AttributeDefinition{
		{
			AttributeName: aws.String("target_id"),
			AttributeType: aws.String("S"),
		},
	},
	&dynamodb.ProvisionedThroughput{
		ReadCapacityUnits:  aws.Int64(1),
		WriteCapacityUnits: aws.Int64(1),
	},
}

var ddbTables = []DDBTable{portalTargets, portalValues}

// CreateTables creates all of the required tables to support the application
func CreateTables(svc *dynamodb.DynamoDB) {
	for _, v := range ddbTables {
		log.Println("creating table:", v.TableName)
		params := &dynamodb.CreateTableInput{
			TableName:             aws.String(v.TableName),
			AttributeDefinitions:  v.AttributeDefinitions,
			KeySchema:             v.KeySchema,
			ProvisionedThroughput: v.ProvisionedThroughput,
		}
		_, err := svc.CreateTable(params)
		if err != nil {
			log.Println("failed to create table", v.TableName, err)
			continue
		}

		// Use a waiter function to wait until the table has been created before proceeding
		log.Println("waiting for table", v.TableName, "to be created")
		describeTableInput := &dynamodb.DescribeTableInput{
			TableName: aws.String(v.TableName),
		}
		if err := svc.WaitUntilTableExists(describeTableInput); err != nil {
			log.Println("failed to create table", v.TableName, ":", err)
		}
	}
}
