package whatsapp

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/trezutil/trezutil/internal/domain"
)

// Config holds WhatsApp adapter configuration.
type Config struct {
	VerifyToken   string
	WebhookSecret string
}

// Adapter handles WhatsApp provider-specific parsing, verification, and outbound mapping.
type Adapter struct {
	config Config
}

// NewAdapter creates a new WhatsApp adapter.
func NewAdapter(config Config) *Adapter {
	return &Adapter{config: config}
}

// VerifyWebhook verifies the webhook signature using HMAC-SHA256.
func (a *Adapter) VerifyWebhook(r *http.Request) error {
	if a.config.WebhookSecret == "" {
		return fmt.Errorf("webhook secret not configured")
	}
	signature := r.Header.Get("X-Hub-Signature-256")
	if signature == "" {
		return fmt.Errorf("missing webhook signature")
	}
	body, err := readBody(r)
	if err != nil {
		return fmt.Errorf("failed to read body: %w", err)
	}
	return a.VerifyWebhookSignature(body, signature)
}

// VerifyWebhookSignature verifies a webhook signature from raw body and signature string.
func (a *Adapter) VerifyWebhookSignature(body []byte, signature string) error {
	if a.config.WebhookSecret == "" {
		return fmt.Errorf("webhook secret not configured")
	}
	if signature == "" {
		return fmt.Errorf("missing webhook signature")
	}
	if !a.verifySignature(body, signature) {
		return fmt.Errorf("invalid webhook signature")
	}
	return nil
}

// ParsePayload parses the raw body into a ProviderPayload.
func (a *Adapter) ParsePayload(body []byte) (*ProviderPayload, error) {
	var payload ProviderPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("failed to parse payload: %w", err)
	}
	return &payload, nil
}

// ParseInbound converts a provider payload into channel, sender, conversation ID, WABA ID, and domain messages.
func (a *Adapter) ParseInbound(payload *ProviderPayload) (domain.Channel, string, string, string, []domain.Message, error) {
	if len(payload.Entry) == 0 {
		return "", "", "", "", nil, fmt.Errorf("no entries in payload")
	}
	entry := payload.Entry[0]
	if len(entry.Changes) == 0 {
		return "", "", "", "", nil, fmt.Errorf("no changes in entry")
	}
	value := entry.Changes[0].Value
	channel := domain.ChannelWhatsApp
	senderPhone := ""
	conversationExternalID := ""
	wabaID := value.WaID
	if len(value.Contacts) > 0 {
		senderPhone = value.Contacts[0].WaID
	}
	if len(value.Conversations) > 0 {
		conversationExternalID = value.Conversations[0].ID
	}
	var messages []domain.Message
	for _, msg := range value.Messages {
		dm, err := a.parseMessage(msg, conversationExternalID)
		if err != nil {
			return "", "", "", "", nil, err
		}
		messages = append(messages, dm)
	}
	return channel, senderPhone, conversationExternalID, wabaID, messages, nil
}

func (a *Adapter) parseMessage(msg ProviderMessage, conversationExternalID string) (domain.Message, error) {
	timestamp, err := parseTimestamp(msg.Timestamp)
	if err != nil {
		return domain.Message{}, fmt.Errorf("invalid timestamp: %w", err)
	}
	msgID := msg.ID
	if msgID == "" {
		return domain.Message{}, fmt.Errorf("message ID is required")
	}
	senderID := msg.From
	if senderID == "" {
		return domain.Message{}, fmt.Errorf("sender ID is required")
	}
	msgType, err := parseMessageType(msg.Type)
	if err != nil {
		return domain.Message{}, err
	}
	content := &domain.MessageContent{
		Data: make(map[string]interface{}),
	}
	switch msgType {
	case domain.MessageTypeText:
		if msg.Text != nil {
			content.Text = msg.Text.Body
		}
	case domain.MessageTypeImage, domain.MessageTypeVideo, domain.MessageTypeAudio, domain.MessageTypeDocument:
		var mediaURL string
		var caption string
		if msg.Image != nil {
			mediaURL = msg.Image.URL
			caption = msg.Image.Caption
		} else if msg.Video != nil {
			mediaURL = msg.Video.URL
			caption = msg.Video.Caption
		} else if msg.Audio != nil {
			mediaURL = msg.Audio.URL
			caption = msg.Audio.Caption
		} else if msg.Document != nil {
			mediaURL = msg.Document.URL
			caption = msg.Document.Caption
		}
		if mediaURL != "" {
			content.MediaURLs = []string{mediaURL}
		}
		if caption != "" {
			content.Text = caption
		}
		content.Data["type"] = string(msgType)
	case domain.MessageTypeLocation:
		if msg.Location != nil {
			content.Data["latitude"] = msg.Location.Latitude
			content.Data["longitude"] = msg.Location.Longitude
			content.Data["name"] = msg.Location.Name
			content.Data["address"] = msg.Location.Address
		}
	default:
		content.Text = msg.Type
		content.Data["type"] = string(msgType)
	}
	return domain.Message{
		ID:             msgID,
		ConversationID: conversationExternalID,
		Direction:      domain.MessageDirectionInbound,
		Type:           msgType,
		SenderID:       senderID,
		Content:        content,
		ExternalRef:    msgID,
		IdempotencyKey: msgID,
		Language:       "en",
		CreatedAt:      timestamp,
	}, nil
}

// BuildOutbound transforms a ConversationResponse into WhatsApp outbound messages.
func (a *Adapter) BuildOutbound(response *domain.ConversationResponse) ([]OutboundMessage, error) {
	var messages []OutboundMessage
	if response.Text != "" {
		messages = append(messages, OutboundMessage{
			Type: "text",
			Text: response.Text,
		})
	}
	for _, card := range response.Cards {
		var buttons []OutboundButton
		for k, v := range card.Data {
			buttons = append(buttons, OutboundButton{ID: k, Title: fmt.Sprintf("%v", v)})
		}
		if len(buttons) > 0 {
			messages = append(messages, OutboundMessage{
				Type:            "interactive",
				InteractiveType: "button",
				Body:            card.Title,
				Footer:          card.Description,
				Buttons:         buttons,
			})
		}
	}
	for _, action := range response.Actions {
		messages = append(messages, OutboundMessage{
			Type:            "interactive",
			InteractiveType: "button",
			Body:            action.Label,
			Buttons:         []OutboundButton{{ID: string(action.ActionType), Title: action.Label}},
		})
	}
	if response.Confirmation != nil {
		messages = append(messages, OutboundMessage{Type: "text", Text: response.Confirmation.Summary})
	}
	if len(messages) == 0 {
		messages = append(messages, OutboundMessage{Type: "text", Text: "OK"})
	}
	return messages, nil
}

func (a *Adapter) verifySignature(body []byte, signature string) bool {
	mac := hmac.New(sha256.New, []byte(a.config.WebhookSecret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	if len(signature) > 7 && signature[:7] == "sha256=" {
		signature = signature[7:]
	}
	return hmac.Equal([]byte(expected), []byte(signature))
}

func readBody(r *http.Request) ([]byte, error) {
	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(r.Body); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func parseTimestamp(ts string) (int64, error) {
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		sec, err2 := strconv.ParseInt(ts, 10, 64)
		if err2 != nil {
			return 0, err
		}
		return sec, nil
	}
	return t.Unix(), nil
}

func parseMessageType(t string) (domain.MessageType, error) {
	switch t {
	case "text":
		return domain.MessageTypeText, nil
	case "image":
		return domain.MessageTypeImage, nil
	case "video":
		return domain.MessageTypeVideo, nil
	case "audio":
		return domain.MessageTypeAudio, nil
	case "document":
		return domain.MessageTypeDocument, nil
	case "location":
		return domain.MessageTypeLocation, nil
	default:
		return "", fmt.Errorf("unsupported message type: %s", t)
	}
}

// Provider types (infrastructure only - NOT in domain)
type ProviderPayload struct {
	Object string          `json:"object"`
	Entry  []ProviderEntry `json:"entry"`
}
type ProviderEntry struct {
	Changes []ProviderChange `json:"changes"`
}
type ProviderChange struct {
	Value ProviderValue `json:"value"`
}
type ProviderValue struct {
	WaID          string                 `json:"wa_id"`
	Messages      []ProviderMessage      `json:"messages"`
	Contacts      []ProviderContact      `json:"contacts"`
	Conversations []ProviderConversation `json:"conversations"`
}
type ProviderMessage struct {
	ID        string            `json:"id"`
	From      string            `json:"from"`
	To        string            `json:"to"`
	Timestamp string            `json:"timestamp"`
	Type      string            `json:"type"`
	Text      *ProviderText     `json:"text"`
	Image     *ProviderMedia    `json:"image"`
	Video     *ProviderMedia    `json:"video"`
	Audio     *ProviderMedia    `json:"audio"`
	Document  *ProviderMedia    `json:"document"`
	Location  *ProviderLocation `json:"location"`
}
type ProviderText struct {
	Body       string `json:"body"`
	PreviewURL bool   `json:"preview_url"`
}
type ProviderMedia struct {
	ID       string `json:"id"`
	MIMEType string `json:"mime_type"`
	Caption  string `json:"caption"`
	URL      string `json:"url"`
}
type ProviderLocation struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Name      string  `json:"name"`
	Address   string  `json:"address"`
}
type ProviderContact struct {
	Profile ProviderContactProfile `json:"profile"`
	WaID    string                 `json:"wa_id"`
}
type ProviderContactProfile struct {
	DisplayName string `json:"display_name"`
}
type ProviderConversation struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	ImageURL string `json:"image_url"`
	Status   string `json:"status"`
}

// Outbound types (infrastructure only - NOT in domain)
type OutboundMessage struct {
	Type            string           `json:"type"`
	Text            string           `json:"text,omitempty"`
	PreviewURL      bool             `json:"preview_url,omitempty"`
	InteractiveType string           `json:"interactive_type,omitempty"`
	Body            string           `json:"body,omitempty"`
	Footer          string           `json:"footer,omitempty"`
	Buttons         []OutboundButton `json:"buttons,omitempty"`
}
type OutboundButton struct {
	ID    string `json:"id,omitempty"`
	Title string `json:"title,omitempty"`
	URL   string `json:"url,omitempty"`
}
