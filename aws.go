package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/kms"
	"github.com/aws/aws-sdk-go/service/sts"
)

func ProcessRoleAssumption(s *sts.STS, t Target, b Become) (*sts.Credentials, error) {
	d, _ := strconv.ParseInt(b.Duration, 10, 64)
	params := &sts.AssumeRoleInput{
		RoleArn:         aws.String(t.ARN),
		RoleSessionName: aws.String(b.UserID),
		DurationSeconds: aws.Int64(d),
	}
	resp, err := s.AssumeRole(params)
	if err != nil {
		return &sts.Credentials{}, fmt.Errorf("could not assume role", err)
	}
	return resp.Credentials, nil
}

func ProcessFederation(km *kms.KMS, t Target, b Become) (*sts.Credentials, error) {
	policy := `{
	  "Version": "2012-10-17",
	  "Statement": [
	    {
	      "Effect": "Allow",
	      "Action": [
	        "*"
	      ],
	      "Resource": "*"
	    }
	  ]
	}`
	// decrypt with KMS
	decoded, err := base64.StdEncoding.DecodeString(t.FederatedCredentials)
	if err != nil {
		return &sts.Credentials{}, err
	}
	kmparams := &kms.DecryptInput{
		CiphertextBlob: []byte(decoded),
	}
	kmresp, err := km.Decrypt(kmparams)
	creds := strings.Split(string(kmresp.Plaintext), "|")
	k, s := creds[0], creds[1]

	c := credentials.NewStaticCredentials(k, s, "")
	sess, err := session.NewSession(&aws.Config{
		Credentials: c,
		Region:      aws.String("us-east-1"),
		MaxRetries:  aws.Int(5),
	})
	if err != nil {
		return &sts.Credentials{}, err
	}

	st := sts.New(sess)
	d, _ := strconv.ParseInt(b.Duration, 10, 64)
	params := &sts.GetFederationTokenInput{
		// Name:            aws.String(b.UserID),
		Name:            aws.String(t.Name),
		DurationSeconds: aws.Int64(d),
		Policy:          aws.String(policy),
	}
	resp, err := st.GetFederationToken(params)
	if err != nil {
		// Print the error, cast err to awserr.Error to get the Code and
		// Message from an error.
		fmt.Println(err.Error())
		return &sts.Credentials{}, fmt.Errorf("could not federate user", err)
	}
	return resp.Credentials, nil
}

type signinToken struct {
	SigninToken string `json:"SigninToken"`
}

func Fileize(aro *sts.Credentials) string {
	cf := fmt.Sprintf("[default]\n"+
		"aws_access_key_id = %s\n"+
		"aws_secret_access_key = %s\n"+
		"aws_seurity_token = %s\n"+
		"aws_session_token = %s\n", *aro.AccessKeyId, *aro.SecretAccessKey,
		*aro.SessionToken, *aro.SessionToken)
	return cf
}

func Portalize(aro *sts.Credentials) string {
	session := struct {
		SessionID   string `json:"sessionId"`
		SessionKey  string `json:"sessionKey"`
		SesionToken string `json:"sessionToken"`
	}{
		*aro.AccessKeyId,
		*aro.SecretAccessKey,
		*aro.SessionToken,
	}
	// check this error eventually
	jcreds, _ := json.Marshal(session)
	rparams := make(url.Values)
	rparams.Set("Action", "getSigninToken")
	rparams.Set("Session", string(jcreds))

	u := url.URL{
		Scheme:   "https",
		Host:     "signin.aws.amazon.com",
		Path:     "federation",
		RawQuery: rparams.Encode(),
	}
	resp, err := http.Get(u.String())
	if err != nil {
		fmt.Println(err)
	}

	var st signinToken
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)
	json.Unmarshal(body, &st)

	rp := make(url.Values)
	rp.Set("Action", "login")
	// make this an environment variable or.. inspect the incoming request
	rp.Set("Issuer", "https://aws.bnuz.co")
	rp.Set("Destination", "https://console.aws.amazon.com/")
	rp.Set("SigninToken", st.SigninToken)

	cu := url.URL{
		Scheme:   "https",
		Host:     "signin.aws.amazon.com",
		Path:     "federation",
		RawQuery: rp.Encode(),
	}
	return cu.String()
}