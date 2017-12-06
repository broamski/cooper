package main

import (
	"fmt"
	"log"
	"net"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

type AdminGroupRecord struct {
	AdminsGroup string `dynamodbav:"value"`
}

// AuthAdmin is Middeleware the checks if an incoming user request
// has administrative level privileges
func AuthAdmin(ddb *dynamodb.DynamoDB) gin.HandlerFunc {
	return func(c *gin.Context) {
		fmt.Println("DEBUG: insecting user for admin privledes")
		session := sessions.Default(c)
		groups := c.Request.Header["X-Saml-Awsgroups"]
		if groups == nil {
			flasher(session, "danger", "there was a problem checking your authorization")
			c.Redirect(307, "/")
			c.Abort()
			return
		}

		// check for ip rescrictions
		if len(Config.AdminIPRestrictions) > 0 {
			restricted := true
			for _, v := range Config.AdminIPRestrictions {
				fmt.Println("this is a provided rescrited admin IP:", v)
				_, ipnet, err := net.ParseCIDR(v)
				if err != nil {
					fmt.Println("An improper CIDR was supplied for admin restrictions:", v)
					flasher(
						session, "danger",
						fmt.Sprintf("An error occurred!"),
					)
					c.Redirect(307, "/")
					c.Abort()
					return
				}
				requestIP := net.ParseIP(c.ClientIP())
				fmt.Println(requestIP)
				if ipnet.Contains(requestIP) {
					restricted = false
				}
			}
			if restricted {
				flasher(
					session, "danger",
					fmt.Sprintf("Your IP not permitted to perform administrative activities"),
				)
				c.Redirect(307, "/")
				c.Abort()
				return
			}
		}

		result := IsAdmin(ddb, groups)
		if !result {
			user := c.Request.Header["X-Saml-Email"]
			fmt.Println(fmt.Sprintf("requested user %s is not an administrator", user))
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

// IsAdmin returns true is the user has admin permissions
func IsAdmin(ddb *dynamodb.DynamoDB, groups []string) bool {
	resp, err := ddb.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String(portalValues.TableName),
		Key: map[string]*dynamodb.AttributeValue{
			"item": {
				S: aws.String("admins_group"),
			},
		},
	})
	if err != nil {
		log.Println(err)
		return false
	}

	if len(resp.Item) == 0 {
		return false
	}
	item := AdminGroupRecord{}
	err = dynamodbattribute.UnmarshalMap(resp.Item, &item)
	if err != nil {
		return false
	}
	for _, v := range groups {
		if v == item.AdminsGroup {
			return true
		}
	}
	return false
}
