package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
)

func main() {
	log.SetOutput(os.Stdout)

	setup := flag.Bool("setup", false, "Perform initial app setup")
	flag.Parse()

	awsregion := os.Getenv("AWS_REGION")
	if awsregion == "" {
		awsregion = "us-east-1"
	}
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(awsregion)},
	)
	if err != nil {
		log.Fatalln("failed to setup the session", err)
	}

	ddb := dynamodb.New(sess)

	if *setup {
		log.Println("running setup..")
		log.Println("creating DynamoDB tables")
		CreateTables(ddb)
		return
	}

	log.Println("eventually will start up an http server")
}
