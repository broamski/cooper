package main

import (
	"flag"
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

	// add a bulk # of users for testing
	// for i := 0; i < 325; i++ {
	//     log.Println(i)
	//     AddAdmin(ddb, AdminUser{fmt.Sprintf("admin-%v", i)})
	// }
	// err = AddAdmin(ddb, AdminUser{"tester"})
	// if err != nil {
	// 	log.Println("failed to add administrative user")
	// }
	// admins, err := GetAdmins(ddb)
	// if err != nil {
	// 	log.Println("failed to get admin list", err)
	// }
	// log.Println(len(admins))
	// for _, v := range admins {
	// 	log.Println("here is", v.Username)
	// }
	// err = RemoveAdmin(ddb, AdminUser{"brian@test.comzz"})
	// if err != nil {
	// 	log.Println("failed to remove user")
	// }
	log.Println("eventually will start up an http server")
}
