package main

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"

	"github.com/duosecurity/duo_api_golang"
	"github.com/duosecurity/duo_api_golang/authapi"
)

func NewDuoAuthClient(config DuoConfig) *authapi.AuthApi {
	// if the skey is base64 encoded, assume it is encrypted
	// and use KMS to decrypt it
	decoded, err := base64.StdEncoding.DecodeString(config.SKey)
	if err != nil || len(decoded) == 0 {
		panic("you should use a KMS encrypted Duo skey!")
	}
	config.SKey, err = KMSDecrypt(KMSClient, config.SKey)
	fmt.Println(config)

	duoClient := duoapi.NewDuoApi(
		config.IKey,
		config.SKey,
		config.ApiHost,
		"cooper",
	)
	duoAuthClient := authapi.NewAuthApi(*duoClient)
	return duoAuthClient
}

func processSecondFactorDuo(authclient *authapi.AuthApi, username string, secondFactor string) (bool, error) {
	check, err := authclient.Check()
	if err != nil {
		return false, errors.New("/check to Duo failed")
	}
	if check.StatResult.Stat != "OK" {
		return false, errors.New(fmt.Sprintf("Could not connect to Duo: %s (%s)",
			*check.StatResult.Message, *check.StatResult.Message_Detail))
	}

	preauth, err := authclient.Preauth(
		authapi.PreauthUsername(username),
	)
	if err != nil || preauth == nil {
		return false, errors.New("There was an error performing Duo preauth")
	}

	if preauth.StatResult.Stat == "OK" {
		fmt.Println("everything is all good for doin that 2fa")
	} else {
		return false, errors.New(fmt.Sprintf("preauth failed with this: %s", preauth.StatResult))
	}

	switch preauth.Response.Result {
	case "allow":
		return false, errors.New("sorry, no bypassing auth here...")
	case "auth":
		fmt.Println("preauth states you need to perform a second factor")
		break
	case "deny":
		return false, errors.New(preauth.Response.Status_Msg)
	case "enroll":
		enrollMsg := fmt.Sprintf("%s (%s)", preauth.Response.Status_Msg, preauth.Response.Enroll_Portal_Url)
		return false, errors.New(enrollMsg)
	default:
		errorMsg := fmt.Sprintf("invalid duo preauth response: %s", preauth.Response.Result)
		return false, errors.New(errorMsg)
	}

	if secondFactor == "" {
		secondFactor = "auto"
	}

	options := []func(*url.Values){authapi.AuthUsername(username)}

	fmt.Println("performing second factor authentication with:", secondFactor)
	if secondFactor != "auto" && secondFactor != "push" && secondFactor != "phone" && secondFactor != "sms" {
		// assume it's a passcode, set it up accordingly
		passcode := secondFactor
		secondFactor = "passcode"
		options = append(options, authapi.AuthPasscode(passcode))
	} else {
		options = append(options, authapi.AuthDevice("auto"))
	}

	result, err := authclient.Auth(secondFactor, options...)
	if result.StatResult.Stat != "OK" {
		errorMsg := "Could not authenticate Duo user"
		if result.StatResult.Message != nil {
			errorMsg = errorMsg + ": " + *result.StatResult.Message
		}
		if result.StatResult.Message_Detail != nil {
			errorMsg = errorMsg + " (" + *result.StatResult.Message_Detail + ")"
		}
		return false, errors.New(errorMsg)
	}

	if result.Response.Result != "allow" {
		fmt.Println(result.Response.Status_Msg)
		return false, errors.New(result.Response.Status_Msg)
	} else {
		fmt.Println("2fa successfully completed")
		return true, nil
	}
}
