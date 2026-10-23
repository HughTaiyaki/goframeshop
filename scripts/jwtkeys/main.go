// Command jwtkeys prints a fresh Ed25519 key pair for local development.
//
// The private key goes into the user service only. The public key goes into
// every process that verifies tokens. Never commit either value.
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
)

func main() {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("JWT_PRIVATE_KEY=%s\n", base64.StdEncoding.EncodeToString(privateKey.Seed()))
	fmt.Printf("JWT_PUBLIC_KEY=%s\n", base64.StdEncoding.EncodeToString(publicKey))
}
