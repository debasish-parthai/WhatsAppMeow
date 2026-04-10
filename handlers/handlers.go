package handlers

import (
	"context"

	"github.com/danielgtaylor/huma/v2"

	"whatsmeow/models"
	"whatsmeow/services"
)

type APIHandlers struct {
	AuthService      *services.AuthService
	MessagingService *services.MessagingService
}

func NewAPIHandlers(auth *services.AuthService, msg *services.MessagingService) *APIHandlers {
	return &APIHandlers{
		AuthService:      auth,
		MessagingService: msg,
	}
}

func (h *APIHandlers) LoginHandler(ctx context.Context, input *models.LoginInput) (*models.LoginOutput, error) {
	resp, err := h.AuthService.GetLogin()
	if err != nil {
		return nil, huma.Error500InternalServerError(err.Error())
	}
	if resp.Body.Status == "error" {
		return nil, huma.Error500InternalServerError(resp.Body.Message)
	}
	return resp, nil
}

func (h *APIHandlers) SendMessageHandler(ctx context.Context, input *models.SendMessageInput) (*models.SendMessageOutput, error) {
	resp, err := h.MessagingService.SendMessage(ctx, input)
	if err != nil {
		if err.Error() == "WhatsApp is not logged in or connected" {
			return nil, huma.Error401Unauthorized(err.Error())
		}
		if err.Error() == "invalid phone number format" {
			return nil, huma.Error400BadRequest(err.Error())
		}
		return nil, huma.Error500InternalServerError("Failed to send message: " + err.Error())
	}
	return resp, nil
}

func (h *APIHandlers) StatusHandler(ctx context.Context, input *models.StatusInput) (*models.StatusOutput, error) {
	resp, err := h.AuthService.Status()
	if err != nil {
		return nil, huma.Error500InternalServerError(err.Error())
	}
	return resp, nil
}

func (h *APIHandlers) LogoutHandler(ctx context.Context, input *models.LogoutInput) (*models.LogoutOutput, error) {
	resp, err := h.AuthService.Logout(ctx)
	if err != nil {
		return nil, huma.Error500InternalServerError("Failed to logout: " + err.Error())
	}
	return resp, nil
}

func (h *APIHandlers) HistoryHandler(ctx context.Context, input *models.HistoryInput) (*models.HistoryOutput, error) {
	return &models.HistoryOutput{Body: h.MessagingService.GetHistory()}, nil
}