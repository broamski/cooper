package main

import (
	"encoding/gob"
	"flag"
	"log"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"

	"github.com/gin-contrib/multitemplate"
	"github.com/gin-contrib/sessions"
	"gopkg.in/gin-gonic/gin.v1"
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
		Region:     aws.String(awsregion),
		MaxRetries: aws.Int(3),
	})
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

	templates := multitemplate.New()
	templates.AddFromFiles("index", "templates/base.html", "templates/index.html")
	templates.AddFromFiles("admins", "templates/base.html", "templates/admins.html")
	templates.AddFromFiles("targets", "templates/base.html", "templates/targets.html")
	templates.AddFromFiles("login", "templates/base.html", "templates/login.html")

	gob.Register(Flash{})

	r := gin.Default()
	r.Static("/assets", "./assets")
	r.HTMLRender = templates
	var secret = []byte("TkQzrflu3SNitU3M3toyoGh9P4r0yxVfpXn8v921")
	store := sessions.NewCookieStore(secret)
	r.Use(sessions.Sessions("session", store))
	r.GET("/", SessionAuthMiddlware(), Index(ddb))
	r.GET("/admins", SessionAuthMiddlware(), Admins(ddb))
	r.GET("/login", Login)
	r.GET("/logout", SessionAuthMiddlware(), Logout)
	r.GET("/targets", SessionAuthMiddlware(), Targets(ddb))
	r.POST("/become", SessionAuthMiddlware(), Becomer(ddb))

	// TOTO: remove - used for setting a debug session
	r.GET("/setcookie", func(c *gin.Context) {
		session := sessions.Default(c)
		var username = "brian@test.com"
		session.Set("username", username)
		session.Save()
		c.JSON(200, gin.H{"username": username})
	})
	r.Run()
}
