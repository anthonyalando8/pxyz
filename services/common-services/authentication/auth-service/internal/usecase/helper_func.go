package usecase

import (
	"context"
	"errors"
	"strconv"
	"x/shared/genproto/otppb"
)

func toPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func (h *UserUsecase) VerifyOtpHelper(ctx context.Context, userId, otpCode, purpose string)(bool, error) {
	idInt, err := strconv.ParseInt(userId, 10, 64)
	if err != nil {
		return false, errors.New("invalid user ID")
	}

	resp, err := h.otp.Client.VerifyOTP(ctx, &otppb.VerifyOTPRequest{
		UserId:  idInt,
		Purpose: purpose,
		Code:    otpCode,
	})
	if err != nil {
		return false, err
	}
	if !resp.Valid {
		return false, errors.New("invalid or expired OTP")
	}
	return true, nil
}