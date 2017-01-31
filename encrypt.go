package main

import (
	"encoding/base64"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/kms"
)

func EncryptKeys(k *kms.KMS, keyid string, payload string) {
	params := &kms.EncryptInput{
		KeyId:     aws.String(keyid), // Required
		Plaintext: []byte(payload),   // Required
	}
	resp, err := k.Encrypt(params)
	if err != nil {
		fmt.Println(fmt.Sprintf("there was an error:", err))
	}
	zz := base64.StdEncoding.EncodeToString(resp.CiphertextBlob)
	fmt.Print("here is your encrypted payload:\n")
	fmt.Println(zz)
}
