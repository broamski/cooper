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
	"strconv"

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

var Config struct {
	Setup               bool
	AdminGroupMapping   string
	Encrypt             bool
	EncryptPayload      string
	KMSKey              string
	Region              string    `yaml:"aws_region"`
	Duo                 DuoConfig `yaml:"duo"`
	IssuerURL           string    `yaml:"issuer_url"`
	IDPMetadataURL      string    `yaml:"idp_metadata_url"`
	SAMLCertPath        string    `yaml:"saml_cert_path"`
	SAMLKeyPath         string    `yaml:"saml_key_path"`
	HighSecurity        bool      `yaml:"high_security"`
	AdminIPRestrictions ipslice   `yaml:"admin_ip_restrictions"`
}

type ipslice []string

func (i *ipslice) String() string {
	return fmt.Sprintf("%d", *i)
}

// The second method is Set(value string) error
func (i *ipslice) Set(value string) error {
	*i = append(*i, value)
	return nil
}

type DuoConfig struct {
	SKey    string `yaml:"skey"`
	IKey    string `yaml:"ikey"`
	ApiHost string `yaml:"apihost"`
}

var ddb *dynamodb.DynamoDB
var KMSClient *kms.KMS
var configPath string

// buildDefaultConfigItem uses the following operation: ENV --> arg --> yaml
func buildDefaultConfigItem(envKey string, def string) (val string) {
	val = os.Getenv(envKey)
	if val == "" {
		val = def
	}
	return
}

func init() {
	flag.BoolVar(&Config.Setup, "setup", false, "perform initial app setup")
	flag.BoolVar(&Config.Encrypt, "encrypt", false, "encrypts a payload (typically for storing federated credentials")
	flag.BoolVar(&Config.HighSecurity, "high-security", func() bool {
		b, err := strconv.ParseBool(buildDefaultConfigItem("HIGH_SECURITY", "true"))
		return err == nil && b
	}(), "encrypts a payload (typically for storing federated credentials")
	flag.Var(&Config.AdminIPRestrictions, "admin-ip-restrictions", "restrict admin actions to these supplied CIDRS")
	flag.StringVar(&Config.AdminGroupMapping, "admin-group-mapping", "", "IdP group name that ties membership administrators")
	flag.StringVar(&Config.EncryptPayload, "encrypt-payload", "", "payload to encrypt")
	flag.StringVar(&Config.KMSKey, "kmskey", "", "kms key ID used to encrypt payload")
	flag.StringVar(&Config.Region, "region", buildDefaultConfigItem("AWS_REGION", ""), "the AWS region where services reside that host cooper")
	flag.StringVar(&Config.IssuerURL, "saml-issuer-url", buildDefaultConfigItem("SAML_ISSUER_URL", ""), "SAML Issuer URL")
	flag.StringVar(&Config.IDPMetadataURL, "saml-idp-metadata-url", buildDefaultConfigItem("SAML_IDP_METADATA_URL", ""), "The Identity Provider's metadata URL")
	flag.StringVar(&Config.SAMLCertPath, "saml-cert-path", buildDefaultConfigItem("SAML_CERT_PATH", ""), "Local certificate to sign SAML requests")
	flag.StringVar(&Config.SAMLKeyPath, "saml-key-path", buildDefaultConfigItem("SAML_KEY_PATH", ""), "Local key to sign SAML requests")
	flag.StringVar(&configPath, "config", "", "")
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

	// parse config file
	source, err := ioutil.ReadFile(configPath)
	if err != nil {
		log.Println("error reading config file:", err)
	} else {
		if len(source) > 0 {
			err = yaml.Unmarshal(source, &Config)
			if err != nil {
				log.Println("could not parse yaml:", err)
			}
		}
	}

	if os.Getenv("AWS_REGION") != "" {
		Config.Region = os.Getenv("AWS_REGION")
	}

	sess, err := session.NewSession(&aws.Config{
		Region:     aws.String(Config.Region),
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
	if Config.Encrypt {
		if Config.KMSKey == "" {
			log.Println("must provide -kmskey")
			return
		}
		if Config.EncryptPayload == "" {
			log.Println("must provide -encrypt-payload")
			return
		}
		KMSEncrypt(KMSClient, Config.KMSKey, Config.EncryptPayload)
		return
	}

	if Config.Setup {
		log.Println("running setup..")
		log.Println("creating DynamoDB tables")
		CreateTables(ddb)
		if Config.AdminGroupMapping != "" {
			err = AddAdminMapping(ddb, Config.AdminGroupMapping)
			if err != nil {
				log.Fatal(fmt.Sprintf("Could not add admin mapping %s - error %s", Config.AdminGroupMapping, err))
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
	keyPair, err := tls.LoadX509KeyPair(Config.SAMLCertPath, Config.SAMLKeyPath)
	if err != nil {
		panic(err) // TODO handle error
	}
	keyPair.Leaf, err = x509.ParseCertificate(keyPair.Certificate[0])
	if err != nil {
		panic(err) // TODO handle error
	}

	idpMetadataURL, err := url.Parse(Config.IDPMetadataURL)
	if err != nil {
		panic(err) // TODO handle error
	}

	rootURL, err := url.Parse(Config.IssuerURL)
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
