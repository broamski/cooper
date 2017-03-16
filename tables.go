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

var portalAdmins = DDBTable{
	"cooper_portal_admins",
	[]*dynamodb.KeySchemaElement{
		{
			AttributeName: aws.String("username"),
			KeyType:       aws.String("HASH"),
		},
	},
	[]*dynamodb.AttributeDefinition{
		{
			AttributeName: aws.String("username"),
			AttributeType: aws.String("S"),
		},
	},
	&dynamodb.ProvisionedThroughput{
		ReadCapacityUnits:  aws.Int64(1),
		WriteCapacityUnits: aws.Int64(1),
	},
}

var portalAdminsAssc = DDBTable{
	"cooper_portal_admins_associations",
	[]*dynamodb.KeySchemaElement{
		{
			AttributeName: aws.String("username"),
			KeyType:       aws.String("HASH"),
		},
		{
			AttributeName: aws.String("account_number"),
			KeyType:       aws.String("RANGE"),
		},
	},
	[]*dynamodb.AttributeDefinition{
		{
			AttributeName: aws.String("username"),
			AttributeType: aws.String("S"),
		},
		{
			AttributeName: aws.String("account_number"),
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

var portalUserAssc = DDBTable{
	"cooper_portal_user_associations",
	[]*dynamodb.KeySchemaElement{
		{
			AttributeName: aws.String("username"),
			KeyType:       aws.String("HASH"),
		},
		{
			AttributeName: aws.String("assoc_id"),
			KeyType:       aws.String("RANGE"),
		},
	},
	[]*dynamodb.AttributeDefinition{
		{
			AttributeName: aws.String("username"),
			AttributeType: aws.String("S"),
		},
		{
			AttributeName: aws.String("assoc_id"),
			AttributeType: aws.String("S"),
		},
	},
	&dynamodb.ProvisionedThroughput{
		ReadCapacityUnits:  aws.Int64(1),
		WriteCapacityUnits: aws.Int64(1),
	},
}

var ddbTables = []DDBTable{portalAdmins, portalTargets, portalUserAssc, portalAdminsAssc}

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
		log.Println("waiting for table", portalAdmins.TableName, "to be created")
		describeTableInput := &dynamodb.DescribeTableInput{
			TableName: aws.String(v.TableName),
		}
		if err := svc.WaitUntilTableExists(describeTableInput); err != nil {
			log.Println("failed to create table", v.TableName, ":", err)
		}
	}
}
