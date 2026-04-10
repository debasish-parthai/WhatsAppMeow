package whatsapp

import (
	"context"
	"fmt"
	"strings"

	"go.mau.fi/whatsmeow/types"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"google.golang.org/protobuf/proto"
)

type DefaultMessageSender struct {
	Adapter *WhatsAppAdapter
}

func NewDefaultMessageSender(adapter *WhatsAppAdapter) *DefaultMessageSender {
	return &DefaultMessageSender{
		Adapter: adapter,
	}
}

// Factory method for creating message payload (Strategy/Builder)
func buildTextMessage(text string) *waProto.Message {
	return &waProto.Message{
		Conversation: proto.String(text),
	}
}

func (s *DefaultMessageSender) SendTextMessage(ctx context.Context, to string, message string) (string, error) {
	if !s.Adapter.Client.IsConnected() || !s.Adapter.Client.IsLoggedIn() {
		return "", fmt.Errorf("WhatsApp is not logged in or connected")
	}

	jidStr := to
	if !strings.Contains(jidStr, "@") {
		jidStr = jidStr + "@s.whatsapp.net"
	}

	targetJID, err := types.ParseJID(jidStr)
	if err != nil {
		return "", fmt.Errorf("invalid phone number format")
	}

	msgPayload := buildTextMessage(message)

	resp, err := s.Adapter.Client.SendMessage(ctx, targetJID, msgPayload)
	if err != nil {
		return "", err
	}

	return resp.ID, nil
}