package main

import (
	"fmt"
	"html/template"
	"net/url"

	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/kms"
	"github.com/aws/aws-sdk-go/service/sts"

	"github.com/gin-contrib/sessions"
	"github.com/satori/go.uuid"
	"gopkg.in/gin-gonic/gin.v1"
)

type Flash struct {
	Type    string
	Message string
}

func flasher(session sessions.Session, ftype, fmsg string) {
	session.AddFlash(Flash{
		Type:    ftype,
		Message: fmsg,
	})
	session.Save()
}

func Authenticated() gin.HandlerFunc {
	return func(c *gin.Context) {
		session := sessions.Default(c)
		u := session.Get("username")
		if u == nil {
			c.Redirect(307, "/login")
			c.Abort()
			return
		}
		c.Next()
	}
}

func AuthenticatedAdmin(ddb *dynamodb.DynamoDB) gin.HandlerFunc {
	return func(c *gin.Context) {
		session := sessions.Default(c)
		u := session.Get("username")
		if u == nil {
			c.Redirect(307, "/login")
			c.Abort()
			return
		}
		a, err := GetAdmin(ddb, u.(string))
		if err != nil {
			flasher(session, "danger", "there was a problem checking your authorization")
			c.Redirect(307, "/login")
			c.Abort()
			return
		}
		if a == (AdminUser{}) {
			flasher(
				session, "danger",
				fmt.Sprintf("you are not authorized to view %s", c.Request.URL.Path),
			)
			c.Redirect(307, "/")
			c.Abort()
			return
		}
		c.Next()
	}
}

func IsAdmin(ddb *dynamodb.DynamoDB, uid string) bool {
	a, err := GetAdmin(ddb, uid)
	if err != nil {
		return false
	}
	if a != (AdminUser{}) {
		return true
	}
	return false
}

// Admins ok?
func Admins(ddb *dynamodb.DynamoDB) gin.HandlerFunc {
	fn := func(c *gin.Context) {
		session := sessions.Default(c)
		flashes := session.Flashes()
		u := session.Get("username")
		admins, err := GetAdmins(ddb)
		if err != nil {
			flasher(session, "danger", fmt.Sprintf("An error occured retrieving admins list: %s", err))
		}
		var funcMap = template.FuncMap{
			"datadmin": func() bool {
				areya := IsAdmin(ddb, u.(string))
				fmt.Println("areyaa:", areya)
				return areya
			},
		}
		session.Save()
		c.HTML(200, "admins", gin.H{
			"title":   "admins",
			"header":  "portal admins",
			"admins":  admins,
			"flashes": flashes,
			"cfunc":   funcMap,
		})
	}
	return gin.HandlerFunc(fn)
}

func AdminsAdd(ddb *dynamodb.DynamoDB) gin.HandlerFunc {
	fn := func(c *gin.Context) {
		session := sessions.Default(c)
		var form AdminUser
		err := c.Bind(&form)
		if err != nil {
			flasher(session, "danger", "payload is invalid or missing")
			c.Redirect(302, "some bad shit happened")
			return
		}
		err = AddAdmin(ddb, form)
		if err != nil {
			flasher(session, "danger", fmt.Sprintf("error adding admin %s: %s", form.Username, err))
			c.Redirect(302, "/admins")
			return
		}
		flasher(session, "success", fmt.Sprintf("added admin: %s", form.Username))
		c.Redirect(302, "/admins")
		return
	}
	return gin.HandlerFunc(fn)
}

func AdminsRemove(ddb *dynamodb.DynamoDB) gin.HandlerFunc {
	fn := func(c *gin.Context) {
		session := sessions.Default(c)
		var form AdminUser
		err := c.Bind(&form)
		if err != nil {
			flasher(session, "danger", "username missing or invalid")
			c.Redirect(302, "some bad shit happened")
			return
		}
		err = RemoveAdmin(ddb, form)
		if err != nil {
			flasher(session, "danger", fmt.Sprintf("error adding admin %s: %s", form.Username, err))
			c.Redirect(302, "/admins")
			return
		}
		flasher(session, "success", fmt.Sprintf("removed admin: %s", form.Username))
		c.Redirect(302, "/admins")
		return
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
		var funcMap = template.FuncMap{
			"datadmin": func() bool {
				areya := IsAdmin(ddb, u.(string))
				fmt.Println("areyaa:", areya)
				return areya
			},
		}
		c.HTML(200, "index", gin.H{
			"title":    "aws portal",
			"header":   fmt.Sprintf("hello, %s", u),
			"username": u,
			"flashes":  flashes,
			"assoc":    as,
			"cfunc":    funcMap,
		})
	}
}

func Login(c *gin.Context) {
	c.HTML(200, "login", gin.H{
		"title": "Login",
	})
}

func Logout(c *gin.Context) {
	session := sessions.Default(c)
	session.Delete("username")
	session.Save()
	c.Redirect(307, "/")
	return
}

func Targets(ddb *dynamodb.DynamoDB) gin.HandlerFunc {
	fn := func(c *gin.Context) {
		session := sessions.Default(c)
		flashes := session.Flashes()
		u := session.Get("username")
		targets, err := GetTargets(ddb)
		if err != nil {
			flasher(session, "danger", "you are not allowed to become this target")
			c.Redirect(302, "/targets")
			return
		}
		var funcMap = template.FuncMap{
			"datadmin": func() bool {
				areya := IsAdmin(ddb, u.(string))
				fmt.Println("areyaa:", areya)
				return areya
			},
		}
		session.Save()
		c.HTML(200, "targets", gin.H{
			"title":    "targets",
			"header":   "target management",
			"username": u,
			"flashes":  flashes,
			"targets":  targets,
			"cfunc":    funcMap,
		})
	}
	return gin.HandlerFunc(fn)
}

func TargetsDetails(ddb *dynamodb.DynamoDB) gin.HandlerFunc {
	fn := func(c *gin.Context) {
		session := sessions.Default(c)
		flashes := session.Flashes()
		tid := c.Param("targetid")
		t, err := GetTarget(ddb, tid)
		if err != nil {
			flasher(session, "danger",
				fmt.Sprintf("there was a problem retrieving the target: %s", tid))
			c.Redirect(302, "/targets")
			return
		}
		if t == (Target{}) {
			flasher(session, "info", fmt.Sprintf("cloud not find target: %s", tid))
			c.Redirect(302, "/targets")
			return
		}
		users, _ := GetTargetUsers(ddb, tid)
		var funcMap = template.FuncMap{
			"datadmin": func() bool {
				areya := IsAdmin(ddb, session.Get("username").(string))
				fmt.Println("areyaa:", areya)
				return areya
			},
		}
		session.Save()
		c.HTML(200, "targets-details", gin.H{
			"title":   fmt.Sprintf("target details - %s", t.Name),
			"flashes": flashes,
			"target":  t,
			"users":   users,
			"cfunc":   funcMap,
		})
	}
	return gin.HandlerFunc(fn)
}

func TargetsSearch(ddb *dynamodb.DynamoDB) gin.HandlerFunc {
	fn := func(c *gin.Context) {
		session := sessions.Default(c)
		flashes := session.Flashes()
		u, ok := c.GetQuery("username")
		if !ok {
			flasher(session, "danger", "could not get username from query string")
			c.Redirect(302, "/targets")
			return
		}
		if len(u) == 0 {
			flasher(session, "warning", "you must enter a usename to search for")
			c.Redirect(302, "/targets")
			return
		}
		a, err := GetAssociations(ddb, u)
		if err != nil {
			flasher(session, "danger", fmt.Sprintf("error: %s", err))
			c.Redirect(302, "/targets")
			return
		}
		if len(a) == 0 {
			flasher(session, "warning", fmt.Sprintf("no assocations found for user %s", u))
			c.Redirect(302, "/targets")
			return
		}
		var funcMap = template.FuncMap{
			"datadmin": func() bool {
				areya := IsAdmin(ddb, u)
				fmt.Println("areyaa:", areya)
				return areya
			},
		}
		session.Save()
		c.HTML(200, "targets-search", gin.H{
			"title":      "targets - search",
			"usersearch": u,
			"flashes":    flashes,
			"utargets":   a,
			"cfunc":      funcMap,
		})
	}
	return gin.HandlerFunc(fn)
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

func Becomer(ddb *dynamodb.DynamoDB, s *sts.STS, k *kms.KMS) gin.HandlerFunc {
	fn := func(c *gin.Context) {
		session := sessions.Default(c)
		who := session.Get("username")
		fmt.Println("Hi", who, "!")
		var form Become
		err := c.Bind(&form)
		if err != nil {
			fmt.Println(err)
			c.String(400, "bad payload")
		}

		// format validation, this should be merged into a larger validator
		if !form.ValidateFormat() {
			fmsg := fmt.Sprintf("Sorry, '%s' is not a vaild format", form.Format)
			flasher(session, "warning", fmsg)
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
			t, err := GetTarget(ddb, form.TargetID)
			if err != nil {
				c.String(500, "becomer->GetTarget error", err)
				return
			}

			var creds *sts.Credentials
			var e error
			if t.Type == "role" {
				fmt.Println("getting credentials by assming role")
				creds, e = ProcessRoleAssumption(s, t, form)
				if e != nil {
					fmt.Println(err)
					flasher(session, "danger", fmt.Sprint("%s", err))
					c.Redirect(301, "/")
					c.Abort()
					return
				}
			} else if t.Type == "user" {
				fmt.Println("getting credentials by GetFederationToken")
				// ProcessFederation(t, form)
				creds, e = ProcessFederation(k, t, form)
				if e != nil {
					fmt.Println(err)
					flasher(session, "danger", fmt.Sprint("%s", err))
					c.Redirect(301, "/")
					c.Abort()
					return
				}
			} else {
				flasher(session, "danger", "unknown type")
				c.Redirect(301, "/")
				c.Abort()
				return
			}

			switch form.Format {
			case "console":
				consoleURL := Portalize(creds)
				c.Redirect(302, consoleURL)
				return
			case "credentials":
				qq := Fileize(creds)
				c.Writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
				c.Writer.Header().Set("Content-Disposition", "attachment; filename=credentials")
				c.Writer.WriteString(qq)
				c.Writer.WriteHeader(200)
				return
			}
			c.String(200, "yes, it worked!")
			return
		}
		flasher(session, "danger", "you are not allowed to become this target")
		c.Redirect(301, "/")
	}
	return gin.HandlerFunc(fn)
}

func TargetsAdd(ddb *dynamodb.DynamoDB) gin.HandlerFunc {
	fn := func(c *gin.Context) {
		session := sessions.Default(c)
		var form Target
		err := c.Bind(&form)
		if err != nil {
			flasher(session, "danger", "username missing or invalid")
			c.Redirect(302, "/targets")
			return
		}
		form.ID = uuid.NewV4().String()
		err = AddTarget(ddb, form)
		if err != nil {
			flasher(session, "danger", fmt.Sprintf("error adding new target %s: %s", form, err))
			c.Redirect(302, "/targets")
			return
		}
		flasher(session, "success", fmt.Sprintf("successfully added target: %s", form))
		c.Redirect(302, "/targets")
		return
	}
	return gin.HandlerFunc(fn)
}

func TargetsRemove(ddb *dynamodb.DynamoDB) gin.HandlerFunc {
	fn := func(c *gin.Context) {
		session := sessions.Default(c)
		var form Target
		err := c.Bind(&form)
		if err != nil {
			flasher(session, "danger", fmt.Sprintf("incoming payload is invalid: %s", err))
			c.Redirect(302, "/targets")
			return
		}
		err = RemoveTarget(ddb, form.ID)
		if err != nil {
			flasher(session, "danger", fmt.Sprintf("error removing target %s: %s", form, err))
			c.Redirect(302, "/targets")
			return
		}
		flasher(session, "success", fmt.Sprintf("removed target: %s and all of its associations", form))
		c.Redirect(302, "/targets")
		return
	}
	return gin.HandlerFunc(fn)
}

func TargetsUpdate(ddb *dynamodb.DynamoDB) gin.HandlerFunc {
	fn := func(c *gin.Context) {
		session := sessions.Default(c)
		var form Target
		err := c.Bind(&form)
		if err != nil {
			flasher(session, "danger", fmt.Sprintf("incoming payload is invalid: %s", err))
			if len(form.ID) == 0 {
				c.Redirect(302, "/targets")
				return
			}
			c.Redirect(302, fmt.Sprintf("/targets/details/%s", form.ID))
			return
		}
		err = UpdateTarget(ddb, form)
		if err != nil {
			flasher(session, "danger", fmt.Sprintf("could not update target: %+v, %s", form, err))
			if len(form.ID) == 0 {
				c.Redirect(302, "/targets")
				return
			}
			c.Redirect(302, fmt.Sprintf("/targets/details/%s", form.ID))
			return
		}
		flasher(session, "success", fmt.Sprintf("succssfully updated target: %-v", form))
		c.Redirect(302, fmt.Sprintf("/targets/details/%s", form.ID))
	}
	return gin.HandlerFunc(fn)
}

type TargetAction struct {
	Username string `form:"username"`
	AssocID  string `form:"assoc_id"`
}

func TargetsAssoc(ddb *dynamodb.DynamoDB) gin.HandlerFunc {
	fn := func(c *gin.Context) {
		session := sessions.Default(c)
		var form TargetAction
		err := c.Bind(&form)
		fmt.Println(fmt.Sprintf("%+v", form))
		if err != nil {
			flasher(session, "danger", fmt.Sprintf("problem with incoming payload: %s", err))
			if len(form.AssocID) == 0 {
				c.Redirect(302, "/targets")
				return
			}
			c.Redirect(302, fmt.Sprintf("/targets/details/%s", form.AssocID))
			return
		}
		if len(form.Username) == 0 {
			flasher(session, "danger", "username input cannot be empty")
			c.Redirect(302, "/targets")
			return
		}
		err = AssociateTarget(ddb, form.Username, form.AssocID)
		if err != nil {
			fmt.Println("AssociateTarget err")
			flasher(
				session,
				"danger",
				fmt.Sprintf("error associating target %s from %s: %s", form.AssocID, form.Username, err),
			)
			if len(form.AssocID) == 0 {
				c.Redirect(302, "/targets")
				return
			}
			c.Redirect(302, fmt.Sprintf("/targets/details/%s", form.AssocID))
			return
		}
		fmt.Println("okay i am good")
		flasher(session, "success", fmt.Sprintf("associated target %s to %s", form.AssocID, form.Username))
		c.Redirect(302, fmt.Sprintf("/targets/details/%s", form.AssocID))
	}
	return gin.HandlerFunc(fn)
}

func TargetsDisassoc(ddb *dynamodb.DynamoDB) gin.HandlerFunc {
	fn := func(c *gin.Context) {
		session := sessions.Default(c)
		var form TargetAction
		err := c.Bind(&form)
		if err != nil {
			flasher(session, "danger", fmt.Sprintf("payload invalid or missing: %s", err))
			if len(form.Username) == 0 {
				c.Redirect(302, "/targets")
				return
			}
			c.Redirect(302, fmt.Sprintf("/targets/search?username=%s", url.QueryEscape(form.Username)))
			return
		}
		err = DisassociateTarget(ddb, form.Username, form.AssocID)
		if err != nil {
			flasher(session, "danger", fmt.Sprintf("error disassocating target %s from %s: %s", form.AssocID, form.Username, err))
			if len(form.Username) == 0 {
				c.Redirect(302, "/targets")
				return
			}
			c.Redirect(302, fmt.Sprintf("/targets/search?username=%s", url.QueryEscape(form.Username)))
			return
		}
		flasher(session, "success", fmt.Sprintf("disassocated target %s from %s", form.AssocID, form.Username))
		c.Redirect(302, fmt.Sprintf("/targets/search?username=%s", url.QueryEscape(form.Username)))
		return
	}
	return gin.HandlerFunc(fn)
}
