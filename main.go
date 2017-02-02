package main

import (
	"encoding/gob"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/kms"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/broamski/cooper/templating"

	"github.com/gin-contrib/sessions"
	"gopkg.in/gin-gonic/gin.v1"
)

var config struct {
	Setup          bool
	Encrypt        bool
	EncryptPayload string
	KMSKey         string
	Region         string
}

func init() {
	flag.BoolVar(&config.Setup, "setup", false, "perform initial app setup")
	flag.BoolVar(&config.Encrypt, "encrypt", false, "encrypts a payload (typically for storing federated credentials")
	flag.StringVar(&config.EncryptPayload, "encrypt-payload", "", "payload to encrypt")
	flag.StringVar(&config.KMSKey, "kmskey", "", "kms key ID used to encrypt payload")
	flag.StringVar(&config.Region, "region", "us-east-1", "the AWS region where services reside that host cooper")
}

func main() {
	log.SetOutput(os.Stdout)
	flag.Parse()

	if os.Getenv("AWS_REGION") != "" {
		config.Region = os.Getenv("AWS_REGION")
	}

	sess, err := session.NewSession(&aws.Config{
		Region:     aws.String(config.Region),
		MaxRetries: aws.Int(5),
	})
	if err != nil {
		log.Fatalln("failed to setup the aws session", err)
	}

	ddb := dynamodb.New(sess)
	sts := sts.New(sess)
	kms := kms.New(sess)

	// utility for encrypting federated user credentials
	// maybe this could also be provided via an html form?
	if config.Encrypt {
		if config.KMSKey == "" {
			log.Println("must provide -kmskey")
			return
		}
		if config.EncryptPayload == "" {
			log.Println("must provide -encrypt-payload")
			return
		}
		KMSEncrypt(kms, config.KMSKey, config.EncryptPayload)
		return
	}

	if config.Setup {
		log.Println("running setup..")
		log.Println("creating DynamoDB tables")
		CreateTables(ddb)
		return
	}

	templates := templating.New()
	templates.AddFromFiles("index", "templates/base.html", "templates/index.html")
	templates.AddFromFiles("admins", "templates/base.html", "templates/admins.html")
	templates.AddFromFiles("admins-details", "templates/base.html", "templates/admins-details.html")
	templates.AddFromFiles("targets", "templates/base.html", "templates/targets.html")
	templates.AddFromFiles("targets-search", "templates/base.html", "templates/targets-search.html")
	templates.AddFromFiles("targets-details", "templates/base.html", "templates/targets-details.html")
	templates.AddFromFiles("login", "templates/base.html", "templates/login.html")

	gob.Register(Flash{})

	log.Println(fmt.Sprintf("%+v", templates))

	r := gin.Default()
	r.Static("/assets", "./assets")
	r.HTMLRender = templates
	// option to provide this is env var
	var secret = []byte("TkQzrflu3SNitU3M3toyoGh9P4r0yxVfpXn8v921")
	store := sessions.NewCookieStore(secret)
	r.Use(sessions.Sessions("session", store))
	r.GET("/", Authenticated(), Index(ddb))
	r.GET("/admins", AuthenticatedAdmin(ddb), Admins(ddb))
	r.GET("/admins/details/:userid", AuthenticatedAdmin(ddb), AdminsDetails(ddb))
	r.POST("/admins/add", AuthenticatedAdmin(ddb), AdminsAdd(ddb))
	r.POST("/admins/remove", AuthenticatedAdmin(ddb), AdminsRemove(ddb))
	r.GET("/login", Login)
	r.GET("/logout", Authenticated(), Logout)
	r.GET("/targets", AuthenticatedAdmin(ddb), Targets(ddb))
	r.GET("/targets/search", AuthenticatedAdmin(ddb), TargetsSearch(ddb))
	r.POST("/targets/add", AuthenticatedAdmin(ddb), TargetsAdd(ddb))
	r.POST("/targets/remove", AuthenticatedAdmin(ddb), TargetsRemove(ddb))
	r.POST("/targets/update", AuthenticatedAdmin(ddb), TargetsUpdate(ddb))
	r.POST("/targets/associate", AuthenticatedAdmin(ddb), TargetsAssoc(ddb))
	r.POST("/targets/disassociate", AuthenticatedAdmin(ddb), TargetsDisassoc(ddb))
	r.GET("/targets/details/:targetid", AuthenticatedAdmin(ddb), TargetsDetails(ddb))
	r.POST("/become", Authenticated(), Becomer(ddb, sts, kms))

	// TOTO: remove - used for setting a debug session
	r.GET("/setcookie", func(c *gin.Context) {
		session := sessions.Default(c)
		var username = "brian@test.com"
		session.Set("username", username)
		session.Save()
		c.JSON(200, gin.H{"username": username})
	})
	// set cookie for second testing user
	r.GET("/setcookie2", func(c *gin.Context) {
		session := sessions.Default(c)
		var username = "brian@example.com"
		session.Set("username", username)
		session.Save()
		c.JSON(200, gin.H{"username": username})
	})
	r.Run()
}
