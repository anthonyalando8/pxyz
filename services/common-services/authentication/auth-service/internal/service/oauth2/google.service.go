package oauth2svc

import (
	"context"
	"log"

	"google.golang.org/api/idtoken"
)

type GoogleUser struct {
	Email     string
	FirstName string
	LastName  string
	Sub       string // Google unique user ID
}

func VerifyGoogleToken(ctx context.Context, token string, clientID string) (*GoogleUser, error) {
	payload, err := idtoken.Validate(ctx, token, clientID)
	if err != nil {
		return nil, err
	}
	log.Printf("Google token payload: %+v\n", payload)
	email, _ := payload.Claims["email"].(string)
	firstName, _ := payload.Claims["given_name"].(string)
	lastName, _ := payload.Claims["family_name"].(string)
	sub, _ := payload.Claims["sub"].(string)

	return &GoogleUser{
		Email:     email,
		FirstName: firstName,
		LastName:  lastName,
		Sub:       sub,
	}, nil
}
