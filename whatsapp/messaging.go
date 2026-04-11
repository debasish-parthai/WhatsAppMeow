package whatsapp

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"go.mau.fi/whatsmeow"
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

func (s *DefaultMessageSender) SendMediaMessage(ctx context.Context, to string, filePath string, mediaType string, caption string) (string, error) {
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

	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %v", err)
	}

	var waMediaType whatsmeow.MediaType
	if mediaType == "image" {
		waMediaType = whatsmeow.MediaImage
	} else if mediaType == "document" {
		waMediaType = whatsmeow.MediaDocument
	} else if mediaType == "video" {
		waMediaType = whatsmeow.MediaVideo
	} else {
		return "", fmt.Errorf("unsupported media type")
	}

	uploaded, err := s.Adapter.Client.Upload(ctx, data, waMediaType)
	if err != nil {
		return "", fmt.Errorf("failed to upload media: %v", err)
	}

	msgPayload := &waProto.Message{}
	fileName := filepath.Base(filePath)

	// Simple mime type detection
	mimeType := http.DetectContentType(data)

	if mediaType == "image" {
		msgPayload.ImageMessage = &waProto.ImageMessage{
			Caption:       proto.String(caption),
			Mimetype:      proto.String(mimeType),
			URL:           &uploaded.URL,
			DirectPath:    &uploaded.DirectPath,
			MediaKey:      uploaded.MediaKey,
			FileEncSHA256: uploaded.FileEncSHA256,
			FileSHA256:    uploaded.FileSHA256,
			FileLength:    &uploaded.FileLength,
		}
	} else if mediaType == "document" {
		msgPayload.DocumentMessage = &waProto.DocumentMessage{
			Title:         proto.String(fileName),
			FileName:      proto.String(fileName),
			Mimetype:      proto.String(mimeType),
			URL:           &uploaded.URL,
			DirectPath:    &uploaded.DirectPath,
			MediaKey:      uploaded.MediaKey,
			FileEncSHA256: uploaded.FileEncSHA256,
			FileSHA256:    uploaded.FileSHA256,
			FileLength:    &uploaded.FileLength,
		}
	} else if mediaType == "video" {
		msgPayload.VideoMessage = &waProto.VideoMessage{
			Caption:       proto.String(caption),
			Mimetype:      proto.String(mimeType),
			URL:           &uploaded.URL,
			DirectPath:    &uploaded.DirectPath,
			MediaKey:      uploaded.MediaKey,
			FileEncSHA256: uploaded.FileEncSHA256,
			FileSHA256:    uploaded.FileSHA256,
			FileLength:    &uploaded.FileLength,
		}
	}

	resp, err := s.Adapter.Client.SendMessage(ctx, targetJID, msgPayload)
	if err != nil {
		return "", err
	}

	return resp.ID, nil
}