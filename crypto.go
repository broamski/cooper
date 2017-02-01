package main

import (
	"encoding/base64"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/kms"
)

// EncryptKeys takes a kms key id, kms service, and string, turning
// it into a base64 encoded string to provide to the portal
func EncryptKeys(k *kms.KMS, keyid string, payload string) {
	params := &kms.EncryptInput{
		KeyId:     aws.String(keyid),
		Plaintext: []byte(payload),
	}
	resp, err := k.Encrypt(params)
	if err != nil {
		fmt.Println("there was an error encrypting the payload:", err)
		return
	}
	ep := base64.StdEncoding.EncodeToString(resp.CiphertextBlob)
	fmt.Print("here is your encrypted payload:\n", ep)
}
