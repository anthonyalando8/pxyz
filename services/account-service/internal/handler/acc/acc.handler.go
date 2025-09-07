package acchandler

import (
	//"context"
	"context"
	"encoding/json"
	"fmt"
	"time"
	emailclient "x/shared/email"
	"x/shared/genproto/accountpb"

	//"x/shared/genproto/accountpb"

	"account-service/internal/service/acc"
)

type AccountHandler struct {
	accService *accservice.AccountService
	emailClient *emailclient.EmailClient
}

func NewAccountHandler(svc *accservice.AccountService, emailClient *emailclient.EmailClient) *AccountHandler {
	return &AccountHandler{accService: svc, emailClient: emailClient}
}


func (h *AccountHandler) GetUserProfile(ctx context.Context, req *accountpb.GetUserProfileRequest) (*accountpb.GetUserProfileResponse, error) {
	profile, err := h.accService.GetOrCreateProfile(ctx, req.UserId, "", "")
	if err != nil {
		return nil, err
	}

	// Convert DOB
	var dob string
	if profile.DateOfBirth != nil {
		dob = profile.DateOfBirth.Format("2006-01-02") // ISO format (YYYY-MM-DD)
	}

	// Convert Address map to JSON string
	var addressJSON string
	if profile.Address != nil {
		addrBytes, err := json.Marshal(profile.Address)
		if err != nil {
			return nil, err
		}
		addressJSON = string(addrBytes)
	}
	var nationality string
	if profile.Nationality != nil {
		nationality = *profile.Nationality
	}

	return &accountpb.GetUserProfileResponse{
		Profile: &accountpb.UserProfile{
			UserId:        profile.UserID,
			FirstName:     profile.FirstName,
			LastName:      profile.LastName,
			Bio:           profile.Bio,
			Gender:        profile.Gender,
			DateOfBirth:   dob,
			AddressJson:   addressJSON,
			ProfileImageUrl: profile.ProfileImageURL,
			Nationality: nationality,
			Username: profile.SysUsername,
			CreatedAt:     profile.CreatedAt.Format(time.RFC3339),
			UpdatedAt:     profile.UpdatedAt.Format(time.RFC3339),
			// If you later add email/phone fields, map them here
		},
	}, nil
}

func (h *AccountHandler) GetUserNationalityStatus(
    ctx context.Context,
    req *accountpb.GetUserNationalityRequest,
) (*accountpb.GetUserNationalityResponse, error) {

    // Ensure profile exists (create if missing)
    profile, err := h.accService.GetOrCreateProfile(ctx, req.UserId, "", "")
    if err != nil {
        return nil, err
    }

    resp := &accountpb.GetUserNationalityResponse{
        HasNationality: profile.Nationality != nil && *profile.Nationality != "",
    }

    if profile.Nationality != nil {
        resp.Nationality = *profile.Nationality
    }

    return resp, nil
}


func (h *AccountHandler) UpdateAccountHandler(ctx context.Context, req *accountpb.UpdateProfileRequest) (*accountpb.UpdateProfileResponse, error) {
	// Fetch existing profile
	profile, err := h.accService.GetOrCreateProfile(ctx, req.UserId, "", "")
	if err != nil {
		return nil, err
	}

	// Update fields if provided
	if req.FirstName != "" {
		profile.FirstName = req.FirstName
	}
	if req.LastName != "" {
		profile.LastName = req.LastName
	}
	if req.Surname != "" {
		profile.Surname = req.Surname
	}
	if req.SysUsername != "" {
		profile.SysUsername = req.SysUsername
	}
	if req.Bio != "" {
		profile.Bio = req.Bio
	}
	if req.Gender != "" {
		profile.Gender = req.Gender
	}
	if req.DateOfBirth != "" {
		// Parse ISO date string → time.Time
		if dob, err := time.Parse("2006-01-02", req.DateOfBirth); err == nil {
			profile.DateOfBirth = &dob
		} else {
			return &accountpb.UpdateProfileResponse{Success: false}, fmt.Errorf("invalid date_of_birth format: %v", err)
		}
	}
	if req.AddressJson != "" {
		var addr map[string]interface{}
		if err := json.Unmarshal([]byte(req.AddressJson), &addr); err == nil {
			profile.Address = addr
		} else {
			return &accountpb.UpdateProfileResponse{Success: false}, fmt.Errorf("invalid address_json: %v", err)
		}
	}

	// Persist changes
	if err := h.accService.UpdateProfile(ctx, profile); err != nil {
		return &accountpb.UpdateProfileResponse{Success: false}, err
	}

	return &accountpb.UpdateProfileResponse{Success: true}, nil
}

func (h *AccountHandler) UpdateProfilePicture(ctx context.Context, req *accountpb.UpdateProfilePictureRequest) (*accountpb.UpdateProfilePictureResponse,error) {
	if err := h.accService.UpdateProfileImage(ctx, req.UserId, req.ImageUrl); err != nil {
		return &accountpb.UpdateProfilePictureResponse{Success: false}, err
	}
	return &accountpb.UpdateProfilePictureResponse{Success: true, ProfileImageUrl: req.ImageUrl}, nil
}

func (h *AccountHandler) UpdateUserNationality(
	ctx context.Context,
	req *accountpb.UpdateUserNationalityRequest,
) (*accountpb.UpdateUserNationalityResponse, error) {

	// Allow clearing nationality if empty string provided
	var nationality *string
	if req.Nationality != "" {
		nationality = &req.Nationality
	}

	if err := h.accService.UpdateNationality(ctx, req.UserId, nationality); err != nil {
		return &accountpb.UpdateUserNationalityResponse{Success: false, Error: err.Error()}, err
	}

	return &accountpb.UpdateUserNationalityResponse{
		Success:     true,
	}, nil
}
