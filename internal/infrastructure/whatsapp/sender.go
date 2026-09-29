package whatsapp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// SenderConfig configures the WhatsApp Cloud API sender.
type SenderConfig struct {
	// BaseURL is the Graph API base, e.g. https://graph.facebook.com/v20.0
	BaseURL       string
	PhoneNumberID string
	AccessToken   string
	// OTPTemplate is the name of an approved "authentication" template used to
	// deliver sign-in codes outside the 24h customer-service window. When empty,
	// codes are sent as plain text (only deliverable inside that window).
	OTPTemplate     string
	OTPTemplateLang string
	Timeout         time.Duration
	HTTPClient      *http.Client
}

// Configured reports whether the sender has everything needed to call the API.
func (c SenderConfig) Configured() bool {
	return c.BaseURL != "" && c.PhoneNumberID != "" && c.AccessToken != ""
}

// Sender delivers outbound WhatsApp messages via the Cloud API.
type Sender struct {
	cfg    SenderConfig
	client *http.Client
}

// ErrSenderNotConfigured is returned when outbound credentials are missing.
var ErrSenderNotConfigured = errors.New("whatsapp sender not configured")

// NewSender creates a Cloud API sender.
func NewSender(cfg SenderConfig) *Sender {
	client := cfg.HTTPClient
	if client == nil {
		timeout := cfg.Timeout
		if timeout <= 0 {
			timeout = 10 * time.Second
		}
		client = &http.Client{Timeout: timeout}
	}
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")
	if cfg.OTPTemplateLang == "" {
		cfg.OTPTemplateLang = "en"
	}
	return &Sender{cfg: cfg, client: client}
}

// SendText sends a plain text message to a WhatsApp user (E.164 or wa_id).
func (s *Sender) SendText(ctx context.Context, to, text string) error {
	return s.post(ctx, map[string]interface{}{
		"messaging_product": "whatsapp",
		"recipient_type":    "individual",
		"to":                strings.TrimPrefix(to, "+"),
		"type":              "text",
		"text":              map[string]interface{}{"body": text, "preview_url": false},
	})
}

// SendOutbound delivers the messages produced by the Presenter/Adapter. Rich
// messages are flattened to text so replies are always deliverable.
func (s *Sender) SendOutbound(ctx context.Context, to string, messages []OutboundMessage) error {
	for _, m := range messages {
		text := m.Text
		if text == "" {
			parts := []string{}
			if m.Body != "" {
				parts = append(parts, m.Body)
			}
			for _, b := range m.Buttons {
				if b.Title != "" {
					parts = append(parts, "• "+b.Title)
				}
			}
			if m.Footer != "" {
				parts = append(parts, m.Footer)
			}
			text = strings.Join(parts, "\n")
		}
		if strings.TrimSpace(text) == "" {
			continue
		}
		if err := s.SendText(ctx, to, text); err != nil {
			return err
		}
	}
	return nil
}

// Send implements domain.OTPProvider by delivering a sign-in code.
func (s *Sender) Send(ctx context.Context, destination string, code string) error {
	if s.cfg.OTPTemplate == "" {
		return s.SendText(ctx, destination, fmt.Sprintf("Your Trezutil verification code is %s. It expires in 5 minutes. Never share it.", code))
	}
	return s.post(ctx, map[string]interface{}{
		"messaging_product": "whatsapp",
		"recipient_type":    "individual",
		"to":                strings.TrimPrefix(destination, "+"),
		"type":              "template",
		"template": map[string]interface{}{
			"name":     s.cfg.OTPTemplate,
			"language": map[string]string{"code": s.cfg.OTPTemplateLang},
			"components": []interface{}{
				map[string]interface{}{
					"type":       "body",
					"parameters": []interface{}{map[string]string{"type": "text", "text": code}},
				},
				map[string]interface{}{
					"type":       "button",
					"sub_type":   "url",
					"index":      "0",
					"parameters": []interface{}{map[string]string{"type": "text", "text": code}},
				},
			},
		},
	})
}

func (s *Sender) post(ctx context.Context, payload map[string]interface{}) error {
	if !s.cfg.Configured() {
		return ErrSenderNotConfigured
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	url := fmt.Sprintf("%s/%s/messages", s.cfg.BaseURL, s.cfg.PhoneNumberID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+s.cfg.AccessToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("whatsapp send failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		// Never echo the access token; the provider body carries only an error code/message.
		detail, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("whatsapp send failed: status %d: %s", resp.StatusCode, strings.TrimSpace(string(detail)))
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	return nil
}
