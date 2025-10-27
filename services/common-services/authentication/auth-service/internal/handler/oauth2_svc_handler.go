// handler/oauth2_handler.go
package handler

import (
	"auth-service/internal/domain"
	service "auth-service/internal/service/app_oauth2_client"
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/url"
	"path/filepath"
	"x/shared/auth/middleware"
	"x/shared/response"
)

type OAuth2Handler struct {
	oauth2Svc *service.OAuth2Service
	AuthHandler *AuthHandler
	authUC    interface{} // Your existing auth usecase
}

func NewOAuth2Handler(oauth2Svc *service.OAuth2Service, authUC interface{}, AuthHandler *AuthHandler) *OAuth2Handler {
	return &OAuth2Handler{
		oauth2Svc: oauth2Svc,
		authUC:    authUC,
		AuthHandler: AuthHandler,
	}
}

// ================================
// AUTHORIZATION ENDPOINT
// ================================

// Authorize initiates the OAuth2 authorization flow
// GET /oauth2/authorize?response_type=code&client_id=xxx&redirect_uri=xxx&scope=read&state=xyz
func (h *OAuth2Handler) Authorize(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse query parameters
	req := &domain.OAuth2AuthorizationRequest{
		ResponseType:        r.URL.Query().Get("response_type"),
		ClientID:            r.URL.Query().Get("client_id"),
		RedirectURI:         r.URL.Query().Get("redirect_uri"),
		Scope:               r.URL.Query().Get("scope"),
		State:               r.URL.Query().Get("state"),
		CodeChallenge:       stringPtrOrNil(r.URL.Query().Get("code_challenge")),
		CodeChallengeMethod: stringPtrOrNil(r.URL.Query().Get("code_challenge_method")),
	}

	// Validate the authorization request
	if err := h.oauth2Svc.ValidateAuthorizationRequest(ctx, req); err != nil {
		h.redirectError(w, r, req.RedirectURI, "invalid_request", err.Error(), req.State)
		return
	}

	// Check if user is authenticated
	userID, authenticated := h.getUserFromRequest(r)

	if !authenticated {
		// Redirect to login with OAuth2 context
		h.redirectToLogin(w, r, req)
		return
	}

	// Check if user has already granted consent
	consent, err := h.oauth2Svc.GetConsentInfo(ctx, req.ClientID, userID, req.Scope)
	if err != nil {
		h.redirectError(w, r, req.RedirectURI, "server_error", "Failed to check consent", req.State)
		return
	}

	if !consent.HasExistingConsent {
		// Show consent screen
		h.showConsentScreen(w, r, req, consent)
		return
	}

	// Generate authorization code and redirect back
	h.completeAuthorization(w, r, req, userID)
}

// ================================
// CONSENT ENDPOINT
// ================================

// ShowConsent displays the consent screen
// GET /oauth2/consent
func (h *OAuth2Handler) ShowConsent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get OAuth2 request from session/state
	clientID := r.URL.Query().Get("client_id")
	scope := r.URL.Query().Get("scope")
	access_token := r.URL.Query().Get("access_token")
	if access_token == "" {
		http.Error(w, "access_token required", http.StatusBadRequest)
		return
	}
	userID, err := h.oauth2Svc.ValidateTemporaryCode(r.Context(),access_token, true)
	if err != nil {
		http.Error(w, "Invalid or expired access token", http.StatusUnauthorized)
		return
	}

	// userID, authenticated := h.getUserFromRequest(r)
	// if !authenticated {
	// 	response.Error(w, http.StatusUnauthorized, "User not authenticated")
	// 	return
	// }

	consentInfo, err := h.oauth2Svc.GetConsentInfo(ctx, clientID, userID, scope)
	if err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	session, err := h.AuthHandler.createSessionHelper(
		r.Context(), userID, true, false, "oauth2_consent", nil, nil, nil, nil, r,
	)
	if err != nil {
		log.Printf("[Consent Token]  Failed to create main session user=%s err=%v", userID, err)
		response.Error(w, http.StatusInternalServerError, "Session creation failed")
		return
	}

	response.JSON(w, http.StatusOK, map[string]interface{}{
		"consent_info": consentInfo,
		"token": session.AuthToken,
	})
}
func (h *OAuth2Handler) ServeConsentUI(w http.ResponseWriter, r *http.Request) {
	access_token := r.URL.Query().Get("access_token")
	if access_token == "" {
		http.Error(w, "access_token required", http.StatusBadRequest)
		return
	}
	_, err := h.oauth2Svc.ValidateTemporaryCode(r.Context(),access_token, false)
	if err != nil {
		http.Error(w, "Invalid or expired access token", http.StatusUnauthorized)
		return
	}
	// Build the path to your UI folder
	uiDir := "/app/ui" // adjust if needed; relative to where binary runs
	file := filepath.Join(uiDir, "screen/oauth2_consent.html")
	log.Printf("Serving consent UI from: %s", file)

	http.ServeFile(w, r, file)
}
func (h *OAuth2Handler) ServeTestUI(w http.ResponseWriter, r *http.Request) {
	// Build the path to your UI folder
	uiDir := "/app/ui" // adjust if needed; relative to where binary runs
	file := filepath.Join(uiDir, "screen/index.html")
	log.Printf("Serving redirect UI from: %s", file)

	http.ServeFile(w, r, file)
}

// GrantConsent handles user consent approval
// POST /oauth2/consent
func (h *OAuth2Handler) GrantConsent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req struct {
		ClientID    string `json:"client_id"`
		Scope       string `json:"scope"`
		RedirectURI string `json:"redirect_uri"`
		State       string `json:"state"`
		Approved    bool   `json:"approved"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request")
		return
	}

	userID, authenticated := h.getUserFromRequest(r)
	if !authenticated {
		response.Error(w, http.StatusUnauthorized, "User not authenticated")
		return
	}

	if !req.Approved {
		// User denied consent
		redirectURL := h.oauth2Svc.BuildErrorResponse(req.RedirectURI, "access_denied", "User denied consent", req.State)
		response.JSON(w, http.StatusOK, map[string]string{"redirect_url": redirectURL})
		return
	}

	// Grant consent
	if err := h.oauth2Svc.GrantConsent(ctx, userID, req.ClientID, req.Scope); err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to grant consent")
		return
	}

	// Complete authorization
	authReq := &domain.OAuth2AuthorizationRequest{
		ClientID:    req.ClientID,
		RedirectURI: req.RedirectURI,
		Scope:       req.Scope,
		State:       req.State,
	}

	code, err := h.oauth2Svc.AuthorizeRequest(ctx, authReq, userID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	redirectURL := h.oauth2Svc.BuildAuthorizationResponse(req.RedirectURI, code, req.State)
	response.JSON(w, http.StatusOK, map[string]string{"redirect_url": redirectURL})
}

// ================================
// TOKEN ENDPOINT
// ================================

// Token handles token exchange
// POST /oauth2/token
func (h *OAuth2Handler) Token(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	bodyBytes, _ := io.ReadAll(r.Body)
	log.Println("Raw body:", string(bodyBytes))
	r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	// Parse form data
	if err := r.ParseForm(); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid_request")
		return
	}

	req := &domain.OAuth2TokenRequest{
		GrantType:    r.FormValue("grant_type"),
		ClientID:     r.FormValue("client_id"),
		ClientSecret: stringPtrOrNil(r.FormValue("client_secret")),
		Code:         stringPtrOrNil(r.FormValue("code")),
		RedirectURI:  stringPtrOrNil(r.FormValue("redirect_uri")),
		RefreshToken: stringPtrOrNil(r.FormValue("refresh_token")),
		Scope:        stringPtrOrNil(r.FormValue("scope")),
		CodeVerifier: stringPtrOrNil(r.FormValue("code_verifier")),
	}
	log.Printf("OAuth2 Token Request: grant_type=%s, client_id=%s", req.GrantType, req.ClientID)

	tokenResp, err := h.oauth2Svc.ExchangeToken(ctx, req)
	if err != nil {
		h.handleTokenError(w, err)
		return
	}

	response.JSON(w, http.StatusOK, tokenResp)
}

// ================================
// INTROSPECTION ENDPOINT
// ================================

// Introspect validates and returns token information
// POST /oauth2/introspect
func (h *OAuth2Handler) Introspect(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := r.ParseForm(); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request")
		return
	}

	token := r.FormValue("token")
	clientID := r.FormValue("client_id")

	if token == "" || clientID == "" {
		response.Error(w, http.StatusBadRequest, "token and client_id required")
		return
	}

	introspection, err := h.oauth2Svc.IntrospectToken(ctx, token, clientID)
	if err != nil {
		response.JSON(w, http.StatusOK, map[string]bool{"active": false})
		return
	}

	response.JSON(w, http.StatusOK, introspection)
}

// ================================
// REVOCATION ENDPOINT
// ================================

// Revoke revokes a token
// POST /oauth2/revoke
func (h *OAuth2Handler) Revoke(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := r.ParseForm(); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request")
		return
	}

	token := r.FormValue("token")
	if token == "" {
		response.Error(w, http.StatusBadRequest, "token required")
		return
	}

	if err := h.oauth2Svc.RevokeToken(ctx, token); err != nil {
		// Per RFC 7009, revocation endpoint should return 200 even if token doesn't exist
		log.Printf("Token revocation error: %v", err)
	}

	w.WriteHeader(http.StatusOK)
}

// ================================
// HELPER METHODS
// ================================

func (h *OAuth2Handler) redirectToLogin(w http.ResponseWriter, r *http.Request, authReq *domain.OAuth2AuthorizationRequest) {
	// Store OAuth2 context in session/state
	state := map[string]string{
		"client_id":    authReq.ClientID,
		"redirect_uri": authReq.RedirectURI,
		"scope":        authReq.Scope,
		"state":        authReq.State,
	}

	if authReq.CodeChallenge != nil {
		state["code_challenge"] = *authReq.CodeChallenge
	}
	if authReq.CodeChallengeMethod != nil {
		state["code_challenge_method"] = *authReq.CodeChallengeMethod
	}

	// Encode state as query parameter
	stateJSON, _ := json.Marshal(state)
	loginURL := "/api/v1/auth/login-ui?oauth2_context=" + url.QueryEscape(string(stateJSON))

	http.Redirect(w, r, loginURL, http.StatusFound)
}

func (h *OAuth2Handler) showConsentScreen(w http.ResponseWriter, r *http.Request, authReq *domain.OAuth2AuthorizationRequest, consent *domain.ConsentInfo) {
	// In a real app, this would render an HTML consent screen
	// For API, return JSON with consent information
	consentURL := "/api/v1/oauth2/consent-ui?client_id=" + authReq.ClientID +
		"&scope=" + url.QueryEscape(authReq.Scope) +
		"&redirect_uri=" + url.QueryEscape(authReq.RedirectURI) +
		"&state=" + url.QueryEscape(authReq.State)

	http.Redirect(w, r, consentURL, http.StatusFound)
}

func (h *OAuth2Handler) completeAuthorization(w http.ResponseWriter, r *http.Request, authReq *domain.OAuth2AuthorizationRequest, userID string) {
	ctx := r.Context()

	code, err := h.oauth2Svc.AuthorizeRequest(ctx, authReq, userID)
	if err != nil {
		h.redirectError(w, r, authReq.RedirectURI, "server_error", err.Error(), authReq.State)
		return
	}

	redirectURL := h.oauth2Svc.BuildAuthorizationResponse(authReq.RedirectURI, code, authReq.State)
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

func (h *OAuth2Handler) redirectError(w http.ResponseWriter, r *http.Request, redirectURI, errorCode, description, state string) {
	if redirectURI == "" {
		response.Error(w, http.StatusBadRequest, errorCode /* description*/)
		return
	}

	errorURL := h.oauth2Svc.BuildErrorResponse(redirectURI, errorCode, description, state)
	http.Redirect(w, r, errorURL, http.StatusFound)
}

func (h *OAuth2Handler) handleTokenError(w http.ResponseWriter, err error) {
	switch err {
	case domain.ErrInvalidClient:
		response.Error(w, http.StatusUnauthorized, "invalid_client" /* "Client authentication failed" */)
	case domain.ErrInvalidGrant:
		response.Error(w, http.StatusBadRequest, "invalid_grant" /*"Invalid authorization code or refresh token" */)
	case domain.ErrUnsupportedGrantType:
		response.Error(w, http.StatusBadRequest, "unsupported_grant_type" /* "Grant type not supported" */)
	default:
		response.Error(w, http.StatusInternalServerError, "server_error" /* "Internal server error"*/)
	}
}

func (h *OAuth2Handler) getUserFromRequest(r *http.Request) (userID string, authenticated bool) {
	// Extract from your existing auth middleware
	// This should check for valid session token
	// Return userID and true if authenticated
	userID, ok := r.Context().Value(middleware.ContextUserID).(string)
	if !ok || userID == "" {
		return "", false
	}
	return userID, true
}

func stringPtrOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
