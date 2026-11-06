// Command verify-update-signature checks an Ed25519 signature with the
// app's public update key before a release is published. Build-time only.
//
// Usage: go run ./scripts/verify-update-signature PUBLIC_KEY_BASE64 FILE SIGNATURE_BASE64
package main

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"os"
)

func main() {
	if len(os.Args) != 4 {
		fmt.Fprintln(os.Stderr, "usage: verify-update-signature PUBLIC_KEY FILE SIGNATURE")
		os.Exit(2)
	}
	key, err := base64.StdEncoding.DecodeString(os.Args[1])
	if err != nil || len(key) != ed25519.PublicKeySize {
		fmt.Fprintln(os.Stderr, "invalid public key")
		os.Exit(1)
	}
	data, err := os.ReadFile(os.Args[2])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	sig, err := base64.StdEncoding.DecodeString(string(bytes.TrimSpace([]byte(os.Args[3]))))
	if err != nil || !ed25519.Verify(key, data, sig) {
		fmt.Fprintln(os.Stderr, "signature does not match the app's public key")
		os.Exit(1)
	}
}
