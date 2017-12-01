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
	"github.com/broamski/cooper/csrf"
	"github.com/broamski/cooper/templating"

	"github.com/crewjam/saml"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/gwatts/gin-adapter"
)

var config struct {
	Setup          bool
	InitialAdmin   string
	Encrypt        bool
	EncryptPayload string
	KMSKey         string
	Region         string
}

var ddb *dynamodb.DynamoDB

func init() {
	flag.BoolVar(&config.Setup, "setup", false, "perform initial app setup")
	flag.BoolVar(&config.Encrypt, "encrypt", false, "encrypts a payload (typically for storing federated credentials")
	flag.StringVar(&config.InitialAdmin, "initial-admin", "", "Username of and admin you'd like to set on setup")
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

	ddb = dynamodb.New(sess)
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
		if config.InitialAdmin != "" {
			newAdmin := Admin{config.InitialAdmin}
			err = AddAdmin(ddb, newAdmin)
			if err != nil {
				log.Fatal(fmt.Sprintf("Could not add admin %s - error %s", config.InitialAdmin, err))
			}
		}
		return
	}

	baseTemplate := "templates/base.html"
	templates := templating.New()
	templates.AddFromFiles("index", baseTemplate, "templates/index.html")
	templates.AddFromFiles("admins", baseTemplate, "templates/admins.html")
	templates.AddFromFiles("admins-details", baseTemplate, "templates/admins-details.html")
	templates.AddFromFiles("targets", baseTemplate, "templates/targets.html")
	templates.AddFromFiles("targets-search", baseTemplate, "templates/targets-search.html")
	templates.AddFromFiles("targets-details", baseTemplate, "templates/targets-details.html")
	templates.AddFromFiles("login", baseTemplate, "templates/login.html")
	templates.AddFromFiles("errors", baseTemplate, "templates/errors.html")

	gob.Register(Flash{})

	log.Println(fmt.Sprintf("%+v", templates))

	r := gin.Default()
	r.Static("/assets", "./assets")
	r.HTMLRender = templates
	// option to provide this is env var
	var secret = []byte("TkQzrflu3SNitU3M3toyoGh9P4r0yxVfpXn8v921")
	store := sessions.NewCookieStore(secret)
	r.Use(sessions.Sessions("session", store))
	r.Use(csrf.Middleware())

	keyPair, err := tls.LoadX509KeyPair("myservice.cert", "myservice.key")
	if err != nil {
		panic(err) // TODO handle error
	}
	keyPair.Leaf, err = x509.ParseCertificate(keyPair.Certificate[0])
	if err != nil {
		panic(err) // TODO handle error
	}

	idpMetadataURL, err := url.Parse("https://www.testshib.org/metadata/testshib-providers.xml")
	if err != nil {
		panic(err) // TODO handle error
	}

	rootURL, err := url.Parse("http://localhost:8000")
	if err != nil {
		panic(err) // TODO handle error
	}

	samlSP, _ := samlsp.New(samlsp.Options{
		IDPMetadataURL: idpMetadataURL,
		URL:            *rootURL,
		Key:            keyPair.PrivateKey.(*rsa.PrivateKey),
		Certificate:    keyPair.Leaf,
	})
	r.GET("/saml", adapter.Wrap(samlSP))

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
	r.GET("/test", func(c *gin.Context) {
		resp, _ := ddb.GetItem(&dynamodb.GetItemInput{
			TableName: aws.String(portalAdmins.TableName),
			Key: map[string]*dynamodb.AttributeValue{
				"username": {
					S: aws.String("brian@test.com"),
				},
			},
		})
		fmt.Println(fmt.Sprintf("%T", resp))
		if len(resp.Item) == 0 {
			fmt.Println("no results returned")
		} else {
			fmt.Println(len(resp.Item), "results returned")
		}
		fmt.Println(resp.Item)
		c.String(200, "ok")
	})
	r.Run()
}
