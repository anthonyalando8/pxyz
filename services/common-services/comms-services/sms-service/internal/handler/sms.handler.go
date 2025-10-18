package handler

import (
    "context"
    "sms-service/internal/domain"
    "sms-service/internal/usecase"
    "x/shared/genproto/smswhatsapppb"
)

type MessageHandler struct {
    smswhatsapppb.UnimplementedSMSWhatsAppServiceServer
    uc *usecase.MessageUsecase
}

func NewMessageHandler(uc *usecase.MessageUsecase) *MessageHandler {
    return &MessageHandler{uc: uc}
}

func (h *MessageHandler) SendMessage(ctx context.Context, req *smswhatsapppb.SendMessageRequest) (*smswhatsapppb.SendMessageResponse, error) {
	msg := &domain.Message{
		UserID:    req.UserId,
		Recipient: req.Recipient,
		Body:      req.Body,
		Channel:   req.Channel.String(), // enum -> string
		Type:      req.Type,
	}

	err := h.uc.SendMessage(ctx, msg)
	if err != nil {
		return &smswhatsapppb.SendMessageResponse{
			Success:      false,
			ErrorMessage: err.Error(),
		}, nil
	}

	return &smswhatsapppb.SendMessageResponse{Success: true}, nil
}

