package csrf

import (
	"crypto/rand"
	"crypto/sha1"
	"encoding/hex"
	"log"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

const (
	key_name = "_csrf_token"
)

var bypassMethods = []string{"GET", "HEAD", "OPTIONS", "TRACE"}
var bypassPaths = []string{"/saml/acs"}

func contains(s []string, value string) bool {
	for _, v := range s {
		if v == value {
			return true
		}
	}
	return false
}

func Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		session := sessions.Default(c)
		existing := session.Get(key_name)

		if existing == nil {
			//log.Println("you don't have a token set, setting it")
			b := make([]byte, 40)
			_, err := rand.Read(b)
			if err != nil {
				c.Abort()
				return
			}

			h := sha1.New()
			h.Write(b)
			sha1_hash := hex.EncodeToString(h.Sum(nil))
			session.Set(key_name, sha1_hash)
		}

		// no need to inspect for _csrf_token on safe methods and paths
		if contains(bypassMethods, c.Request.Method) {
			c.Next()
			return
		}

		if contains(bypassPaths, c.Request.URL.Path) {
			c.Next()
			return
		}

		if c.Request.FormValue("_csrf_token") != session.Get(key_name) {
			log.Println("csrf proection triggered for request:", c.Request)
			c.HTML(400, "errors", gin.H{
				"title":         "error!",
				"error_message": "CSRF Protection Triggered",
			})
			c.Abort()
			return
		}
		c.Next()
	}
}

func GetToken(c *gin.Context) string {
	session := sessions.Default(c)
	existing := session.Get(key_name)
	return existing.(string)
}
