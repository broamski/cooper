package main

import (
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/gob"
	"flag"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/url"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/kms"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/broamski/cooper/csrf"
	"github.com/gin-contrib/multitemplate"

	"github.com/crewjam/saml/samlsp"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/gwatts/gin-adapter"

	"gopkg.in/yaml.v2"
)

var config struct {
	Setup             bool
	AdminGroupMapping string
	Encrypt           bool
	EncryptPayload    string
	KMSKey            string
	Region            string
	CFile             string
}

type DuoConfig struct {
	SKey    string `yaml:"skey"`
	IKey    string `yaml:"ikey"`
	ApiHost string `yaml:"apihost"`
}
type ConfigFile struct {
	Duo            DuoConfig `yaml:"duo"`
	IssuerURL      string    `yaml:"issuer_url"`
	IDPMetadataURL string    `yaml:"idp_metadata_url"`
	SAMLCertPath   string    `yaml:"saml_cert_path"`
	SAMLKeyPath    string    `yaml:"saml_key_path"`
}

var ddb *dynamodb.DynamoDB
var KMSClient *kms.KMS
var ParsedConfigFile ConfigFile

func init() {
	flag.BoolVar(&config.Setup, "setup", false, "perform initial app setup")
	flag.BoolVar(&config.Encrypt, "encrypt", false, "encrypts a payload (typically for storing federated credentials")
	flag.StringVar(&config.AdminGroupMapping, "admin-group-mapping", "", "IdP group name that ties membership administrators")
	flag.StringVar(&config.EncryptPayload, "encrypt-payload", "", "payload to encrypt")
	flag.StringVar(&config.KMSKey, "kmskey", "", "kms key ID used to encrypt payload")
	flag.StringVar(&config.Region, "region", "us-east-1", "the AWS region where services reside that host cooper")
	flag.StringVar(&config.CFile, "config", "config.yaml", "yaml config file")
}

var funcMap = template.FuncMap{
	"IsAdmin": func() bool {
		return false
	},
	"admin": func() bool {
		return false
	},
	"csrf_token": func() string {
		return ""
	},
}

func main() {
	log.SetOutput(os.Stdout)
	flag.Parse()

	// config testing
	source, err := ioutil.ReadFile(config.CFile)
	if err != nil {
		panic(err)
	}
	err = yaml.Unmarshal(source, &ParsedConfigFile)
	if err != nil {
		panic(err)
	}

	// config testing

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
	KMSClient = kms.New(sess)

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
		KMSEncrypt(KMSClient, config.KMSKey, config.EncryptPayload)
		return
	}

	if config.Setup {
		log.Println("running setup..")
		log.Println("creating DynamoDB tables")
		CreateTables(ddb)
		if config.AdminGroupMapping != "" {
			err = AddAdminMapping(ddb, config.AdminGroupMapping)
			if err != nil {
				log.Fatal(fmt.Sprintf("Could not add admin mapping %s - error %s", config.AdminGroupMapping, err))
			}
		}
		return
	}
	log.Println("Cooper, this is no time for caution.")

	baseTemplate := "templates/base.html"
	templates := multitemplate.New()
	templates.AddFromFilesFuncs("index", funcMap, baseTemplate, "templates/index.html")
	templates.AddFromFilesFuncs("targets", funcMap, baseTemplate, "templates/targets.html")
	templates.AddFromFilesFuncs("targets-details", funcMap, baseTemplate, "templates/targets-details.html")
	templates.AddFromFilesFuncs("errors", funcMap, baseTemplate, "templates/errors.html")

	gob.Register(Flash{})

	// saml start
	keyPair, err := tls.LoadX509KeyPair(ParsedConfigFile.SAMLCertPath, ParsedConfigFile.SAMLKeyPath)
	if err != nil {
		panic(err) // TODO handle error
	}
	keyPair.Leaf, err = x509.ParseCertificate(keyPair.Certificate[0])
	if err != nil {
		panic(err) // TODO handle error
	}

	idpMetadataURL, err := url.Parse(ParsedConfigFile.IDPMetadataURL)
	if err != nil {
		panic(err) // TODO handle error
	}

	rootURL, err := url.Parse(ParsedConfigFile.IssuerURL)
	if err != nil {
		panic(err) // TODO handle error
	}

	samlSP, _ := samlsp.New(samlsp.Options{
		IDPMetadataURL: idpMetadataURL,
		URL:            *rootURL,
		Key:            keyPair.PrivateKey.(*rsa.PrivateKey),
		Certificate:    keyPair.Leaf,
	})

	// saml end

	r := gin.Default()
	r.Static("/assets", "./assets")
	r.HTMLRender = templates

	// option to provide this is env var
	var secret = []byte("TkQzrflu3SNitU3M3toyoGh9P4r0yxVfpXn8v921")
	store := sessions.NewCookieStore(secret)

	r.Use(sessions.Sessions("session", store))
	r.Use(csrf.Middleware())

	r.POST("/saml/acs", gin.WrapH(samlSP))
	r.GET("/saml/metadata", gin.WrapH(samlSP))

	authorized := r.Group("/")
	authorized.Use(adapter.Wrap(samlSP.RequireAccount))
	{
		authorized.GET("/", Index(ddb))
		authorized.POST("/become", Becomer(ddb, sts, KMSClient))
		authorized.GET("/logout", Logout)

		authorized.GET("/targets", AuthAdmin(ddb), Targets(ddb))
		authorized.GET("/targets/details/:targetid", AuthAdmin(ddb), TargetsDetails(ddb))
		authorized.POST("/targets/add", AuthAdmin(ddb), TargetsAdd(ddb))
		authorized.POST("/targets/remove", AuthAdmin(ddb), TargetsRemove(ddb))
		authorized.POST("/targets/update", AuthAdmin(ddb), TargetsUpdate(ddb))
	}

	r.Run()
}
