package main

import (
	"fmt"

	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/kms"
	"github.com/aws/aws-sdk-go/service/sts"

	"github.com/broamski/cooper/csrf"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/satori/go.uuid"
)

type Flash struct {
	// success, info, warning or danger
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

func Index(ddb *dynamodb.DynamoDB) gin.HandlerFunc {
	return func(c *gin.Context) {
		session := sessions.Default(c)
		flashes := session.Flashes()
		//u := session.Get("username")
		u := c.Request.Header.Get("X-Saml-Firstname")
		groups := c.Request.Header["X-Saml-Awsgroups"]
		fmt.Println(c.Request.Header["X-Saml-Awsgroups"])
		as, err := GetAssociations(ddb, groups)
		if err != nil {
			// hack a new, non-session flash into the flashes map so that
			// an error can be displayed on the current loading page
			gf := Flash{
				Type:    "danger",
				Message: fmt.Sprintf("Problem: '%s'", err),
			}
			flashes = append(flashes, gf)
		}
		//zz := csrf.GetToken(c)
		//fmt.Println(zz)
		session.Save()
		c.HTML(200, "index", gin.H{
			"IsAdmin":    IsAdmin(ddb, groups),
			"title":      "aws portal",
			"username":   u,
			"flashes":    flashes,
			"assoc":      as,
			"csrf_token": csrf.GetToken(c),
		})
	}
}

func Logout(c *gin.Context) {
	c.SetCookie("token", "", 1, "/", "", false, false)
	session := sessions.Default(c)
	session.Delete("token")
	session.Delete("session")
	session.Save()
	c.Redirect(307, "/")
	return
}

func Targets(ddb *dynamodb.DynamoDB) gin.HandlerFunc {
	fn := func(c *gin.Context) {
		session := sessions.Default(c)
		flashes := session.Flashes()
		u := c.Request.Header.Get("X-Saml-Email")
		groups := c.Request.Header["X-Saml-Awsgroups"]
		targets, err := GetTargets(ddb)
		if err != nil {
			flasher(session, "danger", "you are not allowed to become this target")
			c.Redirect(302, "/targets")
			return
		}
		session.Save()
		c.HTML(200, "targets", gin.H{
			"IsAdmin":    IsAdmin(ddb, groups),
			"title":      "targets",
			"header":     "Target Management",
			"username":   u,
			"flashes":    flashes,
			"targets":    targets,
			"csrf_token": csrf.GetToken(c),
		})
	}
	return gin.HandlerFunc(fn)
}

func TargetsDetails(ddb *dynamodb.DynamoDB) gin.HandlerFunc {
	fn := func(c *gin.Context) {
		session := sessions.Default(c)
		flashes := session.Flashes()
		groups := c.Request.Header["X-Saml-Awsgroups"]
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
		session.Save()
		c.HTML(200, "targets-details", gin.H{
			"IsAdmin":    IsAdmin(ddb, groups),
			"title":      fmt.Sprintf("target details - %s", t.Name),
			"flashes":    flashes,
			"target":     t,
			"csrf_token": csrf.GetToken(c),
		})
	}
	return gin.HandlerFunc(fn)
}

type Become struct {
	UserID       string `form:"-" json:"-"`
	TargetID     string `form:"target_id" json:"target_id"`
	Duration     string `form:"duration" json:"duration"`
	Format       string `form:"format" json:"format" binding:"required"`
	SecondFactor string `form:"second_factor" json:"second_factor" binding:"required"`
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
		username := c.Request.Header.Get("X-Saml-Email")
		actual_username := c.Request.Header.Get("X-Saml-Username")
		fmt.Println(actual_username)
		groups := c.Request.Header["X-Saml-Awsgroups"]
		fmt.Println("Hi", username, "!")
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
		form.UserID = username
		allowed, err := AllowedToBecome(ddb, groups, form.TargetID)
		if err != nil {
			fmt.Println(err)
			flasher(session, "danger", fmt.Sprintf("%s", err))
			c.Redirect(301, "/")
			c.Abort()
			return
		}
		if allowed {
			if Config.HighSecurity {
				duoAuthClient := NewDuoAuthClient(Config.Duo)

				result, err := processSecondFactorDuo(duoAuthClient, username, form.SecondFactor)
				if !result || err != nil {
					flasher(session, "danger", err.Error())
					c.Redirect(301, "/")
					c.Abort()
					return
				}
			}

			t, err := GetTarget(ddb, form.TargetID)
			if err != nil {
				fmt.Println(err)
				flasher(session, "danger", fmt.Sprintf("coult not get target: %s", err))
				c.Redirect(301, "/")
				c.Abort()
				return
			}

			var creds *sts.Credentials
			if t.Type == "role" {
				fmt.Println("getting credentials by assming role")
				creds, err = ProcessRoleAssumption(s, t, form)
				if err != nil {
					fmt.Println(err)
					flasher(session, "danger", fmt.Sprint(err))
					c.Redirect(301, "/")
					c.Abort()
					return
				}
			} else if t.Type == "user" {
				fmt.Println("getting credentials by GetFederationToken")
				creds, err = ProcessFederation(k, t, form)
				if err != nil {
					fmt.Println(err)
					flasher(session, "danger", fmt.Sprintf("there was a problem: %s", err))
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
		username := c.Request.Header.Get("X-Saml-Email")
		var form Target
		err := c.Bind(&form)
		if err != nil {
			flasher(session, "danger", "there was a problem processing form data")
			c.Redirect(302, "/targets")
			return
		}

		if Config.HighSecurity {
			secondFactor := c.DefaultPostForm("second_factor", "auto")
			duoAuthClient := NewDuoAuthClient(Config.Duo)

			result, err := processSecondFactorDuo(duoAuthClient, username, secondFactor)
			if !result || err != nil {
				flasher(session, "danger", err.Error())
				c.Redirect(301, "/targets")
				c.Abort()
				return
			} else {
				fmt.Println("2fa successfully completed")
			}
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
		username := c.Request.Header.Get("X-Saml-Email")

		var form Target
		err := c.Bind(&form)
		if err != nil {
			flasher(session, "danger", fmt.Sprintf("incoming payload is invalid: %s", err))
			c.Redirect(302, "/targets")
			return
		}

		if Config.HighSecurity {
			secondFactor := c.DefaultPostForm("second_factor", "auto")
			duoAuthClient := NewDuoAuthClient(Config.Duo)

			result, err := processSecondFactorDuo(duoAuthClient, username, secondFactor)
			if !result || err != nil {
				flasher(session, "danger", err.Error())
				c.Redirect(302, fmt.Sprintf("/targets/details/%s", form.ID))
				c.Abort()
				return
			} else {
				fmt.Println("2fa successfully completed")
			}
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
		username := c.Request.Header.Get("X-Saml-Email")

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

		if Config.HighSecurity {
			secondFactor := c.DefaultPostForm("second_factor", "auto")
			duoAuthClient := NewDuoAuthClient(Config.Duo)

			result, err := processSecondFactorDuo(duoAuthClient, username, secondFactor)
			if !result || err != nil {
				flasher(session, "danger", err.Error())
				c.Redirect(302, fmt.Sprintf("/targets/details/%s", form.ID))
				c.Abort()
				return
			} else {
				fmt.Println("2fa successfully completed")
			}
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
