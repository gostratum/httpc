package auth

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"strings"
)

// LoadRSAPrivateKey loads an RSA private key from either PEM content or a file
// path prefixed with file:.
func LoadRSAPrivateKey(source string) (*rsa.PrivateKey, error) {
	data, err := loadBytes(source)
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err == nil {
		return key, nil
	}

	pkcs8Key, err2 := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err2 != nil {
		return nil, fmt.Errorf("parse private key: %v %v", err, err2)
	}

	rsaKey, ok := pkcs8Key.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("parsed key is not RSA")
	}
	return rsaKey, nil
}

// LoadHMACSecret loads an HMAC secret from either a literal string or file.
func LoadHMACSecret(source string) ([]byte, error) {
	return loadBytes(source)
}

func loadBytes(source string) ([]byte, error) {
	if strings.HasPrefix(source, "file:") {
		return os.ReadFile(strings.TrimPrefix(source, "file:"))
	}
	return []byte(source), nil
}
