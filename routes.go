package main

import (
	"fmt"

	"github.com/aws/aws-sdk-go/service/dynamodb"

	"github.com/gin-contrib/sessions"
	"gopkg.in/gin-gonic/gin.v1"
)

func SessionAuthMiddlware() gin.HandlerFunc {
	return func(c *gin.Context) {
		fmt.Println("middlewarez")
		session := sessions.Default(c)
		u := session.Get("username")
        if u == nil {
            c.Redirect(307,"/login")
            c.Abort()
            return
        }
		c.Next()
	}
}

// Admins ok?
func Admins(ddb *dynamodb.DynamoDB) gin.HandlerFunc {
	fn := func(c *gin.Context) {
		session := sessions.Default(c)
		u := session.Get("username")
		c.HTML(200, "admins", gin.H{
			"title":    "admins",
			"header":   "cooper admins",
			"username": u,
		})
	}
	return gin.HandlerFunc(fn)
}

func Index(ddb *dynamodb.DynamoDB) gin.HandlerFunc {
	return func(c *gin.Context) {
		session := sessions.Default(c)
		flashes := session.Flashes()
		u := session.Get("username")
		as, err := GetAssociations(ddb, u.(string))
		if err != nil {
			// hack a new, non-session flash into the flashes map so that 
			// an error can be displayed on the current loading page
            gf := Flash{
				Type:    "danger",
				Message: fmt.Sprintf("Problem: '%s'", err),
			}
			flashes = append(flashes, gf)
		}
		session.Save()
		c.HTML(200, "index", gin.H{
			"title":    "aws portal",
			"header":   "Target Selection",
			"username": u,
			"flashes":  flashes,
			"assoc":    as,
		})
	}
}

func Login(c *gin.Context) {
        c.HTML(200, "login", gin.H{
            "title": "Login",
        })
}

func Logout(c *gin.Context) {
    fmt.Println("at logout")
	session := sessions.Default(c)
	session.Delete("username")
	session.Save()
    c.Redirect(307,"/")
    return
}

// Targets ok?
func Targets(ddb *dynamodb.DynamoDB) gin.HandlerFunc {
	fn := func(c *gin.Context) {
		session := sessions.Default(c)
		u := session.Get("username")
		c.HTML(200, "targets", gin.H{
			"title":    "targets",
			"header":   "cooper targets",
			"username": u,
		})
	}
	return gin.HandlerFunc(fn)
}

type Flash struct {
	Type    string
	Message string
}

type Become struct {
	UserID   string `form:"-" json:"-"`
	TargetID string `form:"target_id" json:"target_id"`
	Duration string `form:"duration" json:"duration"`
	Format   string `form:"format" json:"format" binding:"required"`
}

func (b Become) ValidateFormat() bool {
	switch b.Format {
	case
		"console",
		"credentials":
		return true
	}
	return false
}

func Becomer(ddb *dynamodb.DynamoDB) gin.HandlerFunc {
	fn := func(c *gin.Context) {
		session := sessions.Default(c)
		who := session.Get("username")
		fmt.Println("Hi", who, "!")
		var form Become
		err := c.Bind(&form)
		fmt.Println(form)
		if err != nil {
			fmt.Println(err)
			c.String(400, "bad payload")
		}

        // format validation, this should be merged into a larger validator
		if !form.ValidateFormat() {
			florsh := Flash{
				Type:    "warning",
				Message: fmt.Sprintf("Sorry, '%s' is not a vaild format", form.Format),
			}
			session.AddFlash(florsh)
			c.Redirect(301, "/")
			err = session.Save()
			if err != nil {
				fmt.Println(err)
			}
			return
		}
		form.UserID = who.(string)
		fmt.Println(fmt.Sprintf("%+v", form))
		ia, err := IsAssociated(ddb, form.UserID, form.TargetID)
		if err != nil {
			c.String(500, "something bad happened", err)
			return
		}
		if ia {
			c.String(200, "yes, it worked!")
			return
		}
		session.AddFlash(Flash{
			Type:    "danger",
			Message: "uhm, you aren't allowed to become this target",
		})
		session.Save()
		c.Redirect(301, "/")
	}
	return gin.HandlerFunc(fn)
}

func BecomerOld(svc *dynamodb.DynamoDB, b Become) error {
	// create a becoming object and validate its format
	v, err := IsAssociated(svc, b.UserID, b.TargetID)
	if err != nil {
		return fmt.Errorf("error checking association", err)
	}
	if v {
		t, err := GetTarget(svc, b.TargetID)
		if err != nil {
			return fmt.Errorf("becomer->GetTarget error", err)
		}
		switch t.Type {
		case "role":
			fmt.Println("getting credentials by assming role")
		case "user":
			fmt.Println("getting credentials by GetFederationToken")
		}

		switch b.Format {
		case "console":
			Portalize()
		case "file":
			Fileize()
		}
	} else {
		return fmt.Errorf("cannot become this role as you arent associated to it")
	}
	return nil
}

func Portalize() {
	fmt.Println("portalize!")
}

func Fileize() {
	fmt.Println("fileize")
}
