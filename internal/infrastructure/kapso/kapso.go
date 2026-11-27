package kapso

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

type Config struct {
	APIKey         string
	PhoneNumberID  string
	WebhookSecret  string
	BaseURL        string
	Timeout        time.Duration
	HTTPClient     *http.Client
}

func ConfigFromEnv() Config {
	return Config{
		APIKey:        os.Getenv("KAPSO_API_KEY"),
		PhoneNumberID: os.Getenv("KAPSO_PHONE_NUMBER_ID"),
		WebhookSecret: os.Getenv("KAPSO_WEBHOOK_SECRET"),
	}
}

func (c Config) Validate() error {
	if c.APIKey == "" {
		return errors.New("KAPSO_API_KEY is required")
	}
	if c.PhoneNumberID == "" {
		return errors.New("KAPSO_PHONE_NUMBER_ID is required")
	}
	if c.WebhookSecret == "" {
		return errors.New("KAPSO_WEBHOOK_SECRET is required")
	}
	return nil
}

type Client struct {
	cfg    Config
	client *http.Client
}

func NewClient(cfg Config) *Client {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.kapso.ai"
	}
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")
	client := cfg.HTTPClient
	if client == nil {
		timeout := cfg.Timeout
		if timeout <= 0 {
			timeout = 10 * time.Second
		}
		client = &http.Client{Timeout: timeout}
	}
	return &Client{cfg: cfg, client: client}
}

func (c *Client) SendText(ctx context.Context, to, fromUserID, text string) error {
	body := map[string]interface{}{
		"messaging_product": "whatsapp",
		"type":              "text",
		"text":              map[string]string{"body": text},
	}
	if to != "" {
		body["to"] = to
	} else {
		body["recipient"] = map[string]string{"from_user_id": fromUserID}
	}
	_, err := c.post(ctx, body)
	return err
}

func (c *Client) post(ctx context.Context, payload map[string]interface{}) (string, error) {
	if err := c.cfg.Validate(); err != nil {
		return "", err
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	url := fmt.Sprintf("%s/meta/whatsapp/v24.0/%s/messages", c.cfg.BaseURL, c.cfg.PhoneNumberID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		return "", err
	}
	req.Header.Set("X-API-Key", c.cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("kapso send failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		detail, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return "", fmt.Errorf("kapso send failed: status %d: %s", resp.StatusCode, strings.TrimSpace(string(detail)))
	}
	var out struct {
		Messages []struct {
			ID string `json:"id"`
		} `json:"messages"`
	}
	_ = json.NewDecoder(io.LimitReader(resp.Body, 64*1024)).Decode(&out)
	_, _ = io.Copy(io.Discard, resp.Body)
	if len(out.Messages) > 0 {
		return out.Messages[0].ID, nil
	}
	return "", nil
}

func VerifySignature(secret string, body []byte, signature string) bool {
	if secret == "" {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	sig := strings.TrimSpace(signature)
	if len(sig) > 7 && strings.EqualFold(sig[:7], "sha256=") {
		sig = sig[7:]
	}
	return hmac.Equal([]byte(expected), []byte(sig))
}
