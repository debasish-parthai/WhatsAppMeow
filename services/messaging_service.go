package services

import (
	"context"
	"fmt"

	"whatsmeow/models"
	"whatsmeow/whatsapp"
)

type MessagingService struct {
	Sender         whatsapp.MessageSender
	LogoutAction   func()
	MessageHistory []models.MessageLog
}

func NewMessagingService(sender whatsapp.MessageSender) *MessagingService {
	return &MessagingService{
		Sender:         sender,
		MessageHistory: make([]models.MessageLog, 0),
	}
}

func (s *MessagingService) SendMessage(ctx context.Context, input *models.SendMessageInput) (*models.SendMessageOutput, error) {
	msgID, err := s.Sender.SendTextMessage(ctx, input.Body.Phone, input.Body.Message)
	fmt.Printf("[Outgoing (API)] To %s: %s\n", input.Body.Phone, input.Body.Message)

	if err != nil {
		return nil, err
	}

	output := &models.SendMessageOutput{}
	output.Body.Success = true
	output.Body.MessageID = msgID
	return output, nil
}

func (s *MessagingService) SendMediaMessage(ctx context.Context, input *models.SendMediaMessageInput) (*models.SendMediaMessageOutput, error) {
	msgID, err := s.Sender.SendMediaMessage(ctx, input.Body.Phone, input.Body.FilePath, input.Body.MediaType, input.Body.Caption)
	fmt.Printf("[Outgoing (API) Media] To %s: %s - %s\n", input.Body.Phone, input.Body.MediaType, input.Body.FilePath)

	if err != nil {
		return nil, err
	}

	output := &models.SendMediaMessageOutput{}
	output.Body.Success = true
	output.Body.MessageID = msgID
	return output, nil
}

func (s *MessagingService) GetHistory() []models.MessageLog {
	return s.MessageHistory
}

func (s *MessagingService) OnMessageReceived(phone string, message string, isFromMe bool, isWeb bool, timestamp string) {
	entry := models.MessageLog{
		Phone:     phone,
		Message:   message,
		Timestamp: timestamp,
	}

	origin := "Mobile"
	if isWeb {
		origin = "Web"
	}

	if isFromMe {
		entry.Type = "sent"
		fmt.Printf("[Outgoing (%s)] To %s: %s\n", origin, entry.Phone, entry.Message)
	} else {
		entry.Type = "received"
		fmt.Printf("[Incoming (%s)] From %s: %s\n", origin, entry.Phone, entry.Message)
	}

	s.MessageHistory = append(s.MessageHistory, entry)
}

func (s *MessagingService) OnLoggedOut() {
	if s.LogoutAction != nil {
		s.LogoutAction()
	}
}