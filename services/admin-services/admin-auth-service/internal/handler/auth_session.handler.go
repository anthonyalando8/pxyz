package handler

import (
	"admin-auth-service/internal/ws"
	"encoding/json"
	"log"
	"net/http"
	"x/shared/auth/middleware"

	authpb "x/shared/genproto/admin/sessionpb"
	"x/shared/response"

	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"
)

func (h *AuthHandler) LogoutHandler(authClient authpb.AdminSessionServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token, ok := r.Context().Value(middleware.ContextToken).(string)
		if !ok || token == "" {
			response.Error(w, http.StatusUnauthorized, "Missing auth token")
			return
		}

		_, err := authClient.DeleteSession(r.Context(), &authpb.DeleteSessionRequest{
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

func (h *AuthHandler) ListSessionsHandler(authClient authpb.AdminSessionServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := r.Context().Value(middleware.ContextUserID).(string)
		if !ok || userID == "" {
			response.Error(w, http.StatusUnauthorized, "Unauthorized")
			return
		}

		res, err := authClient.ListSessions(r.Context(), &authpb.ListSessionsRequest{
			UserId: userID,
		})
		log.Printf("GRPC response: %v", res)
		if err != nil {
			log.Printf("Failed to list sessions: %v", err)
			response.Error(w, http.StatusInternalServerError, "Could not fetch sessions")
			return
		}

		response.JSON(w, http.StatusOK, res.Sessions)
	}
}

func (h *AuthHandler) DeleteSessionByIDHandler(authClient authpb.AdminSessionServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID := chi.URLParam(r, "id")
		if sessionID == "" {
			response.Error(w, http.StatusBadRequest, "Missing session ID")
			return
		}

		// Optional: check user owns this session first if needed

		_, err := authClient.DeleteSessionByID(r.Context(), &authpb.DeleteSessionByIDRequest{
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

func (h *AuthHandler) LogoutAllHandler(authClient authpb.AdminSessionServiceClient, rdb *redis.Client) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        userID, ok := r.Context().Value(middleware.ContextUserID).(string)
        if !ok || userID == "" {
            response.Error(w, http.StatusUnauthorized, "Unauthorized")
            return
        }

        // Step 1: Call gRPC to delete all sessions
        _, err := authClient.DeleteAllSessions(r.Context(), &authpb.DeleteAllSessionsRequest{
            UserId: userID,
        })
        if err != nil {
            log.Printf("DeleteAllSessions failed: %v", err)
            response.Error(w, http.StatusInternalServerError, "Could not logout from all sessions")
            return
        }

        // Step 2: Publish logout event to Redis
        event := ws.Message{
            Type:   "logout",
            UserID: userID,
            Data: map[string]string{
                "reason": "All sessions have been invalidated",
            },
        }
		
        payload, _ := json.Marshal(event)

        if err := rdb.Publish(r.Context(), "auth_events", payload).Err(); err != nil {
            log.Printf("Failed to publish logout event: %v", err)
        }

        // Step 3: Respond
        response.JSON(w, http.StatusOK, "Logged out from all sessions")
    }
}

