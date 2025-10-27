// handler/oauth2_client_handler.go
package handler

import (
	"auth-service/internal/domain"
	"encoding/json"
	"net/http"
	"x/shared/response"
)

// ================================
// CLIENT MANAGEMENT ENDPOINTS
// ================================

// RegisterClient allows users to register a new OAuth2 application
// POST /api/v1/oauth2/clients
func (h *OAuth2Handler) RegisterClient(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	userID, authenticated := h.getUserFromRequest(r)
	if !authenticated {
		response.Error(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	var req domain.CreateOAuth2ClientRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request")
		return
	}

	// Set the owner
	req.OwnerUserID = userID

	client, plainSecret, err := h.oauth2Svc.RegisterClient(ctx, &req)
	if err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	// Return client details with plain secret (only shown once)
	response.JSON(w, http.StatusCreated, map[string]interface{}{
		"client_id":     client.ClientID,
		"client_secret": plainSecret, // IMPORTANT: Only shown once
		"client_name":   client.ClientName,
		"redirect_uris": client.RedirectURIs,
		"grant_types":   client.GrantTypes,
		"scope":         client.Scope,
		"created_at":    client.CreatedAt,
	})
}

// ListMyClients returns all OAuth2 clients owned by the authenticated user
// GET /api/v1/oauth2/clients
func (h *OAuth2Handler) ListMyClients(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	userID, authenticated := h.getUserFromRequest(r)
	if !authenticated {
		response.Error(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	clients, err := h.oauth2Svc.GetClientsByOwner(ctx, userID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Don't return client secrets
	clientsResp := make([]map[string]interface{}, len(clients))
	for i, client := range clients {
		clientsResp[i] = map[string]interface{}{
			"client_id":     client.ClientID,
			"client_name":   client.ClientName,
			"client_uri":    client.ClientURI,
			"logo_uri":      client.LogoURI,
			"redirect_uris": client.RedirectURIs,
			"grant_types":   client.GrantTypes,
			"scope":         client.Scope,
			"is_active":     client.IsActive,
			"created_at":    client.CreatedAt,
			"updated_at":    client.UpdatedAt,
		}
	}

	response.JSON(w, http.StatusOK, map[string]interface{}{
		"clients": clientsResp,
	})
}

// GetClient returns details of a specific OAuth2 client
// GET /api/v1/oauth2/clients/:client_id
func (h *OAuth2Handler) GetClient(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	userID, authenticated := h.getUserFromRequest(r)
	if !authenticated {
		response.Error(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	clientID := r.URL.Query().Get("client_id")
	if clientID == "" {
		response.Error(w, http.StatusBadRequest, "client_id required")
		return
	}

	client, err := h.oauth2Svc.GetClientByID(ctx, clientID)
	if err != nil {
		response.Error(w, http.StatusNotFound, "Client not found")
		return
	}

	// Verify ownership
	if client.OwnerUserID != userID {
		response.Error(w, http.StatusForbidden, "Access denied")
		return
	}

	response.JSON(w, http.StatusOK, map[string]interface{}{
		"client_id":     client.ClientID,
		"client_name":   client.ClientName,
		"client_uri":    client.ClientURI,
		"logo_uri":      client.LogoURI,
		"redirect_uris": client.RedirectURIs,
		"grant_types":   client.GrantTypes,
		"scope":         client.Scope,
		"is_active":     client.IsActive,
		"created_at":    client.CreatedAt,
		"updated_at":    client.UpdatedAt,
	})
}

// UpdateClient updates an OAuth2 client
// PUT /api/v1/oauth2/clients/:client_id
func (h *OAuth2Handler) UpdateClient(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	userID, authenticated := h.getUserFromRequest(r)
	_ = userID
	if !authenticated {
		response.Error(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	clientID := r.URL.Query().Get("client_id")
	if clientID == "" {
		response.Error(w, http.StatusBadRequest, "client_id required")
		return
	}

	var req domain.UpdateOAuth2ClientRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request")
		return
	}

	client, err := h.oauth2Svc.UpdateClient(ctx, clientID, &req)
	if err != nil {
		if err == domain.ErrAccessDenied {
			response.Error(w, http.StatusForbidden, "Access denied")
			return
		}
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	response.JSON(w, http.StatusOK, map[string]interface{}{
		"client_id":     client.ClientID,
		"client_name":   client.ClientName,
		"client_uri":    client.ClientURI,
		"logo_uri":      client.LogoURI,
		"redirect_uris": client.RedirectURIs,
		"is_active":     client.IsActive,
		"updated_at":    client.UpdatedAt,
	})
}

// DeleteClient deletes (deactivates) an OAuth2 client
// DELETE /api/v1/oauth2/clients/:client_id
func (h *OAuth2Handler) DeleteClient(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	userID, authenticated := h.getUserFromRequest(r)
	if !authenticated {
		response.Error(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	clientID := r.URL.Query().Get("client_id")
	if clientID == "" {
		response.Error(w, http.StatusBadRequest, "client_id required")
		return
	}

	if err := h.oauth2Svc.DeleteClient(ctx, clientID, userID); err != nil {
		if err == domain.ErrAccessDenied {
			response.Error(w, http.StatusForbidden, "Access denied")
			return
		}
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	response.JSON(w, http.StatusOK, map[string]interface{}{
		"message": "Client deleted successfully",
	})
}

// RegenerateClientSecret generates a new client secret
// POST /api/v1/oauth2/clients/:client_id/regenerate-secret
func (h *OAuth2Handler) RegenerateClientSecret(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	userID, authenticated := h.getUserFromRequest(r)
	if !authenticated {
		response.Error(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	clientID := r.URL.Query().Get("client_id")
	if clientID == "" {
		response.Error(w, http.StatusBadRequest, "client_id required")
		return
	}

	newSecret, err := h.oauth2Svc.RegenerateClientSecret(ctx, clientID, userID)
	if err != nil {
		if err == domain.ErrAccessDenied {
			response.Error(w, http.StatusForbidden, "Access denied")
			return
		}
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	response.JSON(w, http.StatusOK, map[string]interface{}{
		"client_secret": newSecret, // IMPORTANT: Only shown once
		"message":       "Client secret regenerated successfully",
	})
}

// ================================
// USER CONSENT MANAGEMENT
// ================================

// ListMyConsents returns all OAuth2 consents granted by the user
// GET /api/v1/oauth2/consents
func (h *OAuth2Handler) ListMyConsents(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	userID, authenticated := h.getUserFromRequest(r)
	if !authenticated {
		response.Error(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	consents, err := h.oauth2Svc.GetUserConsents(ctx, userID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	response.JSON(w, http.StatusOK, map[string]interface{}{
		"consents": consents,
	})
}

// RevokeConsent revokes consent for a specific client
// DELETE /api/v1/oauth2/consents/:client_id
func (h *OAuth2Handler) RevokeConsent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	userID, authenticated := h.getUserFromRequest(r)
	if !authenticated {
		response.Error(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	clientID := r.URL.Query().Get("client_id")
	if clientID == "" {
		response.Error(w, http.StatusBadRequest, "client_id required")
		return
	}

	if err := h.oauth2Svc.RevokeUserConsent(ctx, userID, clientID); err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	response.JSON(w, http.StatusOK, map[string]interface{}{
		"message": "Consent revoked successfully",
	})
}

// RevokeAllConsents revokes all consents granted by the user
// DELETE /api/v1/oauth2/consents
func (h *OAuth2Handler) RevokeAllConsents(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	userID, authenticated := h.getUserFromRequest(r)
	if !authenticated {
		response.Error(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	if err := h.oauth2Svc.RevokeAllUserConsents(ctx, userID); err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	response.JSON(w, http.StatusOK, map[string]interface{}{
		"message": "All consents revoked successfully",
	})
}

func (h *OAuth2Handler) UserInfo(w http.ResponseWriter, r *http.Request) {
    // Extract user info from context (populated by your middleware)
    userID, user, authenticated := h.AuthHandler.getUserFromContext(r)
	if !authenticated {
		response.Error(w, http.StatusUnauthorized, "Authentication required")
		return
	}

    // Example user info response (customize fields)
    info := map[string]interface{}{
        "sub":   userID,           // Standard OIDC field for subject ID
        "email": user.Email,
        "name":  user.FirstName + " " + user.LastName,
		"username": user.Username,
        "phone": user.Phone,
		"photo_url": user.ProfileImageUrl,
        "verified": true,
    }

    response.JSON(w, http.StatusOK, info)
}