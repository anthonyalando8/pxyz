package httphandler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	emailclient "x/shared/email"
	smsclient "x/shared/sms"

	"notification-service/internal/domain"
	"notification-service/internal/usecase"
	xerrors "x/shared/utils/errors"
	"x/shared/auth/middleware"
	"x/shared/response"
)

type NotificationHandler struct {
	uc          *usecase.NotificationUsecase
	emailClient *emailclient.EmailClient
	smsClient   *smsclient.SMSClient
}

func NewNotificationHandler(
	uc *usecase.NotificationUsecase,
	emailClient *emailclient.EmailClient,
	smsClient *smsclient.SMSClient,
) *NotificationHandler {
	return &NotificationHandler{
		uc:          uc,
		emailClient: emailClient,
		smsClient:   smsClient,
	}
}

// ----------------------
// Notification Handlers
// ----------------------

func (h *NotificationHandler) ListNotifications(w http.ResponseWriter, r *http.Request) {
	ownerID := r.Context().Value(middleware.ContextUserID).(string)
	ownerType := r.Context().Value(middleware.ContextUserType).(string)

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	items, err := h.uc.ListNotificationsByOwner(r.Context(), ownerType, ownerID, limit, offset)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, items)
}

func (h *NotificationHandler) ListUnread(w http.ResponseWriter, r *http.Request) {
	ownerID := r.Context().Value(middleware.ContextUserID).(string)
	ownerType := r.Context().Value(middleware.ContextUserType).(string)

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	items, err := h.uc.ListUnread(r.Context(), ownerType, ownerID, limit, offset)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, items)
}

func (h *NotificationHandler) CountUnread(w http.ResponseWriter, r *http.Request) {
	ownerID := r.Context().Value(middleware.ContextUserID).(string)
	ownerType := r.Context().Value(middleware.ContextUserType).(string)

	count, err := h.uc.CountUnread(r.Context(), ownerType, ownerID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, map[string]int{"count": count})
}

func (h *NotificationHandler) MarkAsRead(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, _ := strconv.ParseInt(idStr, 10, 64)

	ownerID := r.Context().Value(middleware.ContextUserID).(string)
	ownerType := r.Context().Value(middleware.ContextUserType).(string)

	if err := h.uc.MarkAsRead(r.Context(), id, ownerType, ownerID); err != nil {
		if err == xerrors.ErrNotFound {
			response.Error(w, http.StatusNotFound, "notification not found")
			return
		}
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *NotificationHandler) HideNotification(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, _ := strconv.ParseInt(idStr, 10, 64)

	ownerID := r.Context().Value(middleware.ContextUserID).(string)
	ownerType := r.Context().Value(middleware.ContextUserType).(string)

	if err := h.uc.HideFromApp(r.Context(), id, ownerType, ownerID); err != nil {
		if err == xerrors.ErrNotFound {
			response.Error(w, http.StatusNotFound, "notification not found")
			return
		}
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ----------------------
// Preference Handlers
// ----------------------

func (h *NotificationHandler) GetPreference(w http.ResponseWriter, r *http.Request) {
	ownerID := r.Context().Value(middleware.ContextUserID).(string)
	ownerType := r.Context().Value(middleware.ContextUserType).(string)

	pref, err := h.uc.GetPreferenceByOwner(r.Context(), ownerType, ownerID)
	if err != nil {
		if err == xerrors.ErrNotFound {
			response.Error(w, http.StatusNotFound, "preferences not found")
			return
		}
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, pref)
}

func (h *NotificationHandler) UpsertPreference(w http.ResponseWriter, r *http.Request) {
	var pref domain.NotificationPreference
	if err := json.NewDecoder(r.Body).Decode(&pref); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid input")
		return
	}

	ownerID := r.Context().Value(middleware.ContextUserID).(string)
	ownerType := r.Context().Value(middleware.ContextUserType).(string)
	pref.OwnerID = ownerID
	pref.OwnerType = ownerType

	created, err := h.uc.UpsertPreference(r.Context(), &pref)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, created)
}

func (h *NotificationHandler) DeletePreference(w http.ResponseWriter, r *http.Request) {
	ownerID := r.Context().Value(middleware.ContextUserID).(string)
	ownerType := r.Context().Value(middleware.ContextUserType).(string)

	if err := h.uc.DeletePreferenceByOwner(r.Context(), ownerType, ownerID); err != nil {
		if err == xerrors.ErrNotFound {
			response.Error(w, http.StatusNotFound, "preferences not found")
			return
		}
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
