package handler

import (
	"errors"
	"ptn-auth-service/pkg/utils"
)

func validateSignupRequest(req RegisterInit) error {
	if req.Email != "" && !utils.ValidateEmail(req.Email) {
		return errors.New("invalid email format")
	}
	if !req.AcceptTerms {
		return errors.New("you must accept terms and conditions to register")
	}
	return nil
}
func detectChannel(req RegisterInit) (string, string) {
	if req.Phone != "" {
		return "sms", req.Phone
	}
	return "email", req.Email
}
