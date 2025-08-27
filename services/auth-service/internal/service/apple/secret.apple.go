package apple

import (
	"crypto/x509"
	"encoding/pem"
	"errors"
	"time"

	"github.com/lestrrat-go/jwx/v2/jwa"
	"github.com/lestrrat-go/jwx/v2/jws"
	"github.com/lestrrat-go/jwx/v2/jwt"
)

type ClientSecretParams struct {
	TeamID        string
	KeyID         string
	ServiceID     string // client_id (Service ID) you created in Apple developer
	PrivateKeyPEM string
	TTL           time.Duration // max 6 months
}

func GenerateClientSecret(p ClientSecretParams) (string, error) {
	block, _ := pem.Decode([]byte(p.PrivateKeyPEM))
	if block == nil {
		return "", errors.New("invalid PEM for Apple private key")
	}

	priv, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		// Some keys are PKCS1
		if k, err2 := x509.ParsePKCS1PrivateKey(block.Bytes); err2 == nil {
			priv = k
		} else {
			return "", err
		}
	}

	now := time.Now()
	tok, err := jwt.NewBuilder().
		Issuer(p.TeamID).
		IssuedAt(now).
		Expiration(now.Add(p.TTL)).
		Audience([]string{"https://appleid.apple.com"}).
		Subject(p.ServiceID).
		Build()
	if err != nil {
		return "", err
	}

	// Set the `kid` in protected headers
	hdrs := jws.NewHeaders()
	if err := hdrs.Set(jws.KeyIDKey, p.KeyID); err != nil {
		return "", err
	}

	signed, err := jwt.Sign(tok,
		jwt.WithKey(jwa.ES256, priv, jws.WithProtectedHeaders(hdrs)),
	)
	if err != nil {
		return "", err
	}
	return string(signed), nil
}
