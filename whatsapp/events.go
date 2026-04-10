package whatsapp

import (
	"context"
	"fmt"
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

		msgContent := v.Message.GetConversation()
		if msgContent == "" && v.Message.ExtendedTextMessage != nil {
			msgContent = v.Message.ExtendedTextMessage.GetText()
		}

		if msgContent != "" {
			timestamp := time.Now().Format("02 Jan 15:04")
			phone := senderJID.User
			if v.Info.IsFromMe {
				phone = chatJID.User
			}

			// LLD Logic: Distinguish Web vs Mobile WhatsApp
			isWeb := false
			if v.Info.Sender.Device > 0 || v.Info.DeviceSentMeta != nil {
				isWeb = true
			}

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