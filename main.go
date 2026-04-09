package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
	"github.com/skip2/go-qrcode"
	_ "modernc.org/sqlite"
	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"
)

var wac *whatsmeow.Client
var qrCodeStr string
var connecting bool
var loginTimeout bool

type MessageLog struct {
	Phone     string `json:"phone"`
	Message   string `json:"message"`
	Type      string `json:"type"` // "sent" or "received"
	Timestamp string `json:"timestamp"`
}

var messageHistory []MessageLog

func eventHandler(evt interface{}) {
	switch v := evt.(type) {
	case *events.Message:
		chatJID := v.Info.Chat
		if chatJID.Server == "lid" {
			if !v.Info.RecipientAlt.IsEmpty() && v.Info.RecipientAlt.Server == "s.whatsapp.net" {
				chatJID = v.Info.RecipientAlt
			} else if v.Info.DeviceSentMeta != nil && v.Info.DeviceSentMeta.DestinationJID != "" {
				if parsed, err := types.ParseJID(v.Info.DeviceSentMeta.DestinationJID); err == nil && parsed.Server == "s.whatsapp.net" {
					chatJID = parsed
				}
			}
			if chatJID.Server == "lid" && wac.Store.LIDs != nil {
				if pn, err := wac.Store.LIDs.GetPNForLID(context.Background(), chatJID); err == nil && !pn.IsEmpty() {
					chatJID = pn
				}
			}
		}

		senderJID := v.Info.Sender
		if senderJID.Server == "lid" {
			if !v.Info.SenderAlt.IsEmpty() && v.Info.SenderAlt.Server == "s.whatsapp.net" {
				senderJID = v.Info.SenderAlt
			} else if wac.Store.LIDs != nil {
				if pn, err := wac.Store.LIDs.GetPNForLID(context.Background(), senderJID); err == nil && !pn.IsEmpty() {
					senderJID = pn
				}
			}
		}

		msgContent := v.Message.GetConversation()
		if msgContent == "" && v.Message.ExtendedTextMessage != nil {
			msgContent = v.Message.ExtendedTextMessage.GetText()
		}

		if msgContent != "" {
			entry := MessageLog{
				Message:   msgContent,
				Timestamp: time.Now().Format("02 Jan 15:04"),
			}

			if v.Info.IsFromMe {
				entry.Phone = chatJID.User
				entry.Type = "sent"
				fmt.Printf("[Outgoing] To %s: %s\n", entry.Phone, entry.Message)
			} else {
				entry.Phone = senderJID.User
				entry.Type = "received"
				fmt.Printf("[Incoming] From %s: %s\n", entry.Phone, entry.Message)
			}
			messageHistory = append(messageHistory, entry)
		}

	case *events.LoggedOut:
		fmt.Println("\n[!] WARNING: The device was unlinked.")
	}
}

// Huma Models

type LoginInput struct{}

type LoginOutput struct {
	Body struct {
		Status      string `json:"status" doc:"Status of the connection"`
		QRCode      string `json:"qr_code,omitempty" doc:"QR code string to be scanned"`
		QRCodeImage string `json:"qr_code_image,omitempty" doc:"Base64 encoded PNG of the QR code"`
		Message     string `json:"message" doc:"Additional information"`
	}
}

func loginHandler(ctx context.Context, input *LoginInput) (*LoginOutput, error) {
	resp := &LoginOutput{}

	if wac.IsLoggedIn() {
		resp.Body.Status = "logged_in"
		resp.Body.Message = "Device is already logged in"
		return resp, nil
	}

	if wac.Store.ID == nil {
		if loginTimeout {
			resp.Body.Status = "timeout"
			resp.Body.Message = "QR code generation timed out. Please request again."
			loginTimeout = false
			return resp, nil
		}

		if qrCodeStr == "" && !connecting {
			connecting = true
			qrChan, _ := wac.GetQRChannel(context.Background())
			err := wac.Connect()
			if err != nil {
				connecting = false
				return nil, huma.Error500InternalServerError("Failed to connect: " + err.Error())
			}
			go func() {
				for evt := range qrChan {
					if evt.Event == "code" {
						qrCodeStr = evt.Code
						fmt.Println(qrCodeStr)
						fmt.Println("New QR Code generated.")
					} else if evt.Event == "success" {
						qrCodeStr = ""
						connecting = false
						loginTimeout = false
						fmt.Println("Login successful!")
					} else if evt.Event == "timeout" {
						qrCodeStr = ""
						connecting = false
						loginTimeout = true
						fmt.Println("Login timeout. Please request a new QR code.")
						wac.Disconnect()
					}
				}
			}()
			time.Sleep(2 * time.Second) // Wait a bit for the first QR code to arrive
		}
		
		if qrCodeStr != "" {
			resp.Body.Status = "qr_ready"
			resp.Body.QRCode = qrCodeStr
			
			// Generate the QR code image
			png, err := qrcode.Encode(qrCodeStr, qrcode.Medium, 256)
			if err == nil {
				resp.Body.QRCodeImage = "data:image/png;base64," + base64.StdEncoding.EncodeToString(png)
			}
			
			resp.Body.Message = "Please scan the QR code to log in."
		} else {
			resp.Body.Status = "generating"
			resp.Body.Message = "QR code is being generated. Please request again."
		}
	} else {
		if !wac.IsConnected() {
			err := wac.Connect()
			if err != nil {
				return nil, huma.Error500InternalServerError("Failed to connect: " + err.Error())
			}
		}
		resp.Body.Status = "connecting"
		resp.Body.Message = "Connecting using existing session"
	}

	return resp, nil
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

func sendMessageHandler(ctx context.Context, input *SendMessageInput) (*SendMessageOutput, error) {
	if !wac.IsConnected() || !wac.IsLoggedIn() {
		return nil, huma.Error401Unauthorized("WhatsApp is not logged in or connected")
	}

	jidStr := input.Body.Phone
	if !strings.Contains(jidStr, "@") {
		jidStr = jidStr + "@s.whatsapp.net"
	}

	targetJID, err := types.ParseJID(jidStr)
	if err != nil {
		return nil, huma.Error400BadRequest("Invalid phone number format")
	}

	resp, err := wac.SendMessage(context.Background(), targetJID, &waProto.Message{
		Conversation: proto.String(input.Body.Message),
	})

	if err != nil {
		return nil, huma.Error500InternalServerError("Failed to send message: " + err.Error())
	}

	fmt.Printf("Sent a message to %s: %s\n", targetJID.User, input.Body.Message)

	output := &SendMessageOutput{}
	output.Body.Success = true
	output.Body.MessageID = resp.ID

	return output, nil
}

type StatusInput struct{}

type StatusOutput struct {
	Body struct {
		Connected bool `json:"connected" doc:"Is the client connected to WhatsApp"`
		LoggedIn  bool `json:"logged_in" doc:"Is the client logged in"`
	}
}

func statusHandler(ctx context.Context, input *StatusInput) (*StatusOutput, error) {
	resp := &StatusOutput{}
	resp.Body.Connected = wac.IsConnected()
	resp.Body.LoggedIn = wac.IsLoggedIn()
	return resp, nil
}

type LogoutInput struct{}

type LogoutOutput struct {
	Body struct {
		Success bool `json:"success" doc:"Logout success"`
	}
}

func logoutHandler(ctx context.Context, input *LogoutInput) (*LogoutOutput, error) {
	err := wac.Logout(ctx)
	if err != nil {
		return nil, huma.Error500InternalServerError("Failed to logout: " + err.Error())
	}
	resp := &LogoutOutput{}
	resp.Body.Success = true
	return resp, nil
}

type HistoryInput struct{}
type HistoryOutput struct {
	Body []MessageLog
}

func historyHandler(ctx context.Context, input *HistoryInput) (*HistoryOutput, error) {
	return &HistoryOutput{Body: messageHistory}, nil
}

func main() {
	// Create data directory if it doesn't exist
	_ = os.Mkdir("data", 0755)

	dbLog := waLog.Stdout("Database", "INFO", true)
	dsn := "file:data/whatsmeow.db?_pragma=foreign_keys(ON)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)"
	container, err := sqlstore.New(context.Background(), "sqlite", dsn, dbLog)
	if err != nil {
		panic(err)
	}
    // ... rest of main

	// If you want multiple sessions, remember their JIDs and use .GetDevice(jid) or .GetAllDevices() for all.
	deviceStore, err := container.GetFirstDevice(context.Background())
	if err != nil {
		panic(err)
	}

	clientLog := waLog.Stdout("Client", "INFO", true)
	wac = whatsmeow.NewClient(deviceStore, clientLog)
	wac.AddEventHandler(eventHandler)

	if wac.Store.ID != nil {
		// No ID stored, new login
		err = wac.Connect()
		if err != nil {
			fmt.Println("Failed to connect:", err)
		}
	}

	// Setup API
	mux := http.NewServeMux()
	api := humago.New(mux, huma.DefaultConfig("WhatsApp API", "1.0.0"))

	huma.Register(api, huma.Operation{
		OperationID: "login",
		Method:      http.MethodPost,
		Path:        "/login",
		Summary:     "Get QR Code / Login",
		Description: "Returns a QR code to be scanned with the WhatsApp app if not logged in.",
	}, loginHandler)

	huma.Register(api, huma.Operation{
		OperationID: "send-message",
		Method:      http.MethodPost,
		Path:        "/send",
		Summary:     "Send a message",
		Description: "Sends a text message to a specified phone number.",
	}, sendMessageHandler)

	huma.Register(api, huma.Operation{
		OperationID: "status",
		Method:      http.MethodGet,
		Path:        "/status",
		Summary:     "Get connection status",
		Description: "Returns whether the client is currently connected and logged in.",
	}, statusHandler)
	
	huma.Register(api, huma.Operation{
		OperationID: "logout",
		Method:      http.MethodPost,
		Path:        "/logout",
		Summary:     "Logout",
		Description: "Logs out the current WhatsApp session.",
	}, logoutHandler)

	huma.Register(api, huma.Operation{
		OperationID: "history",
		Method:      http.MethodGet,
		Path:        "/history",
		Summary:     "Get Message History",
		Description: "Returns all captured incoming and outgoing messages.",
	}, historyHandler)

	go func() {
		fmt.Println("Server running on http://localhost:8080")
		fmt.Println("Docs available at http://localhost:8080/docs")
		
		// Simple CORS middleware
		corsMux := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
			w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")

			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			mux.ServeHTTP(w, r)
		})

		if err := http.ListenAndServe(":8080", corsMux); err != nil {
			panic(err)
		}
	}()

	// Listen to Ctrl+C (you can also do something else that prevents the program from exiting)
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	wac.Disconnect()
}

