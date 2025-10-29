package handler

import (
	"log"
	"net/http"
	"time"
	"x/shared/auth/middleware"

	authpb "x/shared/genproto/sessionpb"
	"x/shared/response"

	"github.com/go-chi/chi/v5"
)

func (h *AuthHandler) LogoutHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token, ok := r.Context().Value(middleware.ContextToken).(string)
		if !ok || token == "" {
			response.Error(w, http.StatusUnauthorized, "Missing auth token")
			return
		}

		_, err := h.auth.Client.DeleteSession(r.Context(), &authpb.DeleteSessionRequest{
			Token: token,
		})
		if err != nil {
			log.Printf("Logout failed: %v", err)
			response.Error(w, http.StatusInternalServerError, "Failed to log out")
			return
		}

		response.JSON(w, http.StatusOK, "Logged out successfully")
	}
}

func (h *AuthHandler) ListSessionsHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := r.Context().Value(middleware.ContextUserID).(string)
		if !ok || userID == "" {
			response.Error(w, http.StatusUnauthorized, "Unauthorized")
			return
		}

		res, err := h.auth.Client.ListSessions(r.Context(), &authpb.ListSessionsRequest{
			UserId: userID,
		})
		if err != nil {
			log.Printf("Failed to list sessions: %v", err)
			response.Error(w, http.StatusInternalServerError, "Could not fetch sessions")
			return
		}

		response.JSON(w, http.StatusOK, res.Sessions)
	}
}

func (h *AuthHandler) DeleteSessionByIDHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID := chi.URLParam(r, "id")
		if sessionID == "" {
			response.Error(w, http.StatusBadRequest, "Missing session ID")
			return
		}

		// Optional: check user owns this session first if needed

		_, err := h.auth.Client.DeleteSessionByID(r.Context(), &authpb.DeleteSessionByIDRequest{
			SessionId: sessionID,
		})
		if err != nil {
			log.Printf("DeleteSessionByID failed: %v", err)
			response.Error(w, http.StatusInternalServerError, "Failed to delete session")
			return
		}

		response.JSON(w, http.StatusOK, "Session deleted")
	}
}

func (h *AuthHandler) LogoutAllHandler() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        userID, ok := r.Context().Value(middleware.ContextUserID).(string)
        if !ok || userID == "" {
            response.Error(w, http.StatusUnauthorized, "Unauthorized")
            return
        }

        // Step 1: Call gRPC to delete all sessions
        _, err := h.auth.Client.DeleteAllSessions(r.Context(), &authpb.DeleteAllSessionsRequest{
            UserId: userID,
        })
        if err != nil {
            log.Printf("DeleteAllSessions failed: %v", err)
            response.Error(w, http.StatusInternalServerError, "Could not logout from all sessions")
            return
        }

		h.publisher.Publish(r.Context(), "auth.logout", userID, "", map[string]string{
			"message": "You logged out from this device",
			"title": "logout",
			"timestamp": time.Now().Format(time.RFC3339),
		})

        // Step 3: Respond
        response.JSON(w, http.StatusOK, "Logged out from all sessions")
    }
}

