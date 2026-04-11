package whatsapp

import (
	"context"
	"fmt"
	"os"
	"time"

	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

type EventDispatcher struct {
	Adapter  *WhatsAppAdapter
	Listener MessageListener
}

func NewEventDispatcher(adapter *WhatsAppAdapter, listener MessageListener) *EventDispatcher {
	dispatcher := &EventDispatcher{
		Adapter:  adapter,
		Listener: listener,
	}
	adapter.Dispatcher = dispatcher
	adapter.Client.AddEventHandler(dispatcher.HandleEvent)
	return dispatcher
}

func (d *EventDispatcher) HandleEvent(evt interface{}) {
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
			if chatJID.Server == "lid" && d.Adapter.Client.Store.LIDs != nil {
				if pn, err := d.Adapter.Client.Store.LIDs.GetPNForLID(context.Background(), chatJID); err == nil && !pn.IsEmpty() {
					chatJID = pn
				}
			}
		}

		senderJID := v.Info.Sender
		if senderJID.Server == "lid" {
			if !v.Info.SenderAlt.IsEmpty() && v.Info.SenderAlt.Server == "s.whatsapp.net" {
				senderJID = v.Info.SenderAlt
			} else if d.Adapter.Client.Store.LIDs != nil {
				if pn, err := d.Adapter.Client.Store.LIDs.GetPNForLID(context.Background(), senderJID); err == nil && !pn.IsEmpty() {
					senderJID = pn
				}
			}
		}

		// LLD Logic: Distinguish Web vs Mobile WhatsApp
		isWeb := false
		msgContent := v.Message.GetConversation()

		if msgContent == "" && v.Message.ExtendedTextMessage != nil {
			msgContent = v.Message.ExtendedTextMessage.GetText()
			isWeb = true
		}

		// Handle Media Messages
		if img := v.Message.GetImageMessage(); img != nil {
			data, err := d.Adapter.Client.Download(context.Background(), img)
			if err == nil {
				filename := fmt.Sprintf("media/images/received_%d.jpg", time.Now().Unix())
				os.WriteFile(filename, data, 0644)
				msgContent = fmt.Sprintf("[Image received and saved to %s]", filename)
			} else {
				msgContent = fmt.Sprintf("[Image received but failed to download: %v]", err)
			}
		}

		if doc := v.Message.GetDocumentMessage(); doc != nil {
			data, err := d.Adapter.Client.Download(context.Background(), doc)
			if err == nil {
				ext := ".file"
				if doc.GetFileName() != "" {
					ext = "_" + doc.GetFileName()
				}
				filename := fmt.Sprintf("media/documents/received_%d%s", time.Now().Unix(), ext)
				os.WriteFile(filename, data, 0644)
				msgContent = fmt.Sprintf("[Document received and saved to %s]", filename)
			} else {
				msgContent = fmt.Sprintf("[Document received but failed to download: %v]", err)
			}
		}

		if vid := v.Message.GetVideoMessage(); vid != nil {
			data, err := d.Adapter.Client.Download(context.Background(), vid)
			if err == nil {
				filename := fmt.Sprintf("media/videos/received_%d.mp4", time.Now().Unix())
				os.WriteFile(filename, data, 0644)
				msgContent = fmt.Sprintf("[Video received and saved to %s]", filename)
			} else {
				msgContent = fmt.Sprintf("[Video received but failed to download: %v]", err)
			}
		}

		if msgContent != "" {
			timestamp := time.Now().Format("02 Jan 15:04")
			phone := senderJID.User
			if v.Info.IsFromMe {
				phone = chatJID.User
			}

			
			// Debug: print parsed text payload from the event message.
			//fmt.Printf("Message payload text: %q\n", msgContent)
			// if v.Info.Sender.Device > 0 || v.Info.DeviceSentMeta != nil {
			// 	isWeb = true
			// }

			if d.Listener != nil {
				d.Listener.OnMessageReceived(phone, msgContent, v.Info.IsFromMe, isWeb, timestamp)
			}
		}

	case *events.LoggedOut:
		fmt.Println("\n[!] WARNING: The device was unlinked.")
		if d.Listener != nil {
			d.Listener.OnLoggedOut()
		}
	}
}