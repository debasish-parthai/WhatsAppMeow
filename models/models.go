package models

// MessageLog represents a logged message
type MessageLog struct {
	Phone     string `json:"phone"`
	Message   string `json:"message"`
	Type      string `json:"type"` // "sent" or "received"
	Timestamp string `json:"timestamp"`
}

type LoginInput struct{}

type LoginOutput struct {
	Body struct {
		Status      string `json:"status" doc:"Status of the connection"`
		QRCode      string `json:"qr_code,omitempty" doc:"QR code string to be scanned"`
		QRCodeImage string `json:"qr_code_image,omitempty" doc:"Base64 encoded PNG of the QR code"`
		Message     string `json:"message" doc:"Additional information"`
	}
}

type SendMessageInput struct {
	Body struct {
		Phone   string `json:"phone" doc:"Phone number with country code, e.g. 1234567890"`
		Message string `json:"message" doc:"The message to send"`
	}
}

type SendMessageOutput struct {
	Body struct {
		Success   bool   `json:"success" doc:"True if message sent successfully"`
		MessageID string `json:"message_id,omitempty" doc:"ID of the sent message"`
	}
}

type StatusInput struct{}

type StatusOutput struct {
	Body struct {
		Connected bool `json:"connected" doc:"Is the client connected to WhatsApp"`
		LoggedIn  bool `json:"logged_in" doc:"Is the client logged in"`
	}
}

type LogoutInput struct{}

type LogoutOutput struct {
	Body struct {
		Success bool `json:"success" doc:"Logout success"`
	}
}

type HistoryInput struct{}

type HistoryOutput struct {
	Body []MessageLog
}