package sms

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// PatientRelation is one recipient for an outbound message.
type PatientRelation struct {
	Phone      string `json:"phone"`
	FirstName  string `json:"first_name"`
	LastName   string `json:"last_name"`
	NationalID string `json:"national_id"`
}

// SendRequest is the body for POST /api/v1/outbound/send.
type SendRequest struct {
	PatientRelations []PatientRelation `json:"patient_relations"`
	ClinicCode       int               `json:"clinic_code"`
	PatientCode      int               `json:"patient_code"`
	MessageText      string            `json:"message_text"`
	Messenger        string            `json:"messenger"` // SMS | Bale | BaleANDSMS | BaleORSMS
	Operator         string            `json:"operator"`
	IP               string            `json:"ip"`
}

// Config holds outbound messaging API settings.
type Config struct {
	BaseURL   string
	Messenger string // default SMS
	Operator  string // default MCI
	HTTPClient *http.Client

	// Token generation (preferred over static AuthToken).
	AESKeyB64  string
	HMACKeyB64 string
	UserCode   string

	// AuthToken is a static fallback when token generator keys are not configured.
	AuthToken string
}

// Client sends outbound SMS/messenger messages via the clinic messaging API.
type Client struct {
	baseURL   string
	messenger string
	operator  string
	http      *http.Client
	tokens    *TokenGenerator
	authToken string // static fallback
}

// NewClient constructs a messaging Client from Config.
// Inputs: cfg with BaseURL and either token keys or static AuthToken.
// Output: pointer to Client; nil cfg fields get safe defaults.
func NewClient(cfg Config) (*Client, error) {
	messenger := strings.TrimSpace(cfg.Messenger)
	if messenger == "" {
		messenger = "SMS"
	}
	operator := strings.TrimSpace(cfg.Operator)
	if operator == "" {
		operator = "MCI"
	}
	hc := cfg.HTTPClient
	if hc == nil {
		hc = &http.Client{Timeout: 15 * time.Second}
	}

	c := &Client{
		baseURL:   strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/"),
		messenger: messenger,
		operator:  operator,
		http:      hc,
		authToken: strings.TrimSpace(cfg.AuthToken),
	}

	if cfg.AESKeyB64 != "" && cfg.HMACKeyB64 != "" {
		gen, err := NewTokenGenerator(cfg.AESKeyB64, cfg.HMACKeyB64, cfg.UserCode)
		if err != nil {
			return nil, err
		}
		c.tokens = gen
	}

	return c, nil
}

// Enabled reports whether the client has enough config to send messages.
// Inputs: none (uses client fields).
// Output: true when BaseURL and auth (token generator or static token) are configured.
func (c *Client) Enabled() bool {
	if c == nil || c.baseURL == "" {
		return false
	}
	return c.tokens != nil || c.authToken != ""
}

// authHeader returns the Authorization header value for one send request.
// Inputs: clinicCode and clientIP for dynamic token generation.
// Output: token string or error when auth is not configured.
func (c *Client) authHeader(clinicCode int, clientIP string) (string, error) {
	if c.tokens != nil {
		return c.tokens.Generate(clinicCode, clientIP)
	}
	if c.authToken != "" {
		return c.authToken, nil
	}
	return "", fmt.Errorf("messaging auth is not configured")
}

// SendParams is the input for a single outbound send.
type SendParams struct {
	Phone       string
	FirstName   string
	LastName    string
	NationalID  string
	ClinicCode  int
	PatientCode int
	MessageText string
	Messenger   string // optional override; empty uses client default
	IP          string
}

// Send posts one outbound message to the messaging API.
// Inputs: ctx for cancellation, params for recipient and text.
// Output: error when disabled, HTTP failure, or non-2xx status.
func (c *Client) Send(ctx context.Context, params SendParams) error {
	if !c.Enabled() {
		return fmt.Errorf("messaging client is not configured")
	}
	messenger := strings.TrimSpace(params.Messenger)
	if messenger == "" {
		messenger = c.messenger
	}
	ip := strings.TrimSpace(params.IP)
	if ip == "" {
		ip = "127.0.0.1"
	}

	auth, err := c.authHeader(params.ClinicCode, ip)
	if err != nil {
		return err
	}

	body := SendRequest{
		PatientRelations: []PatientRelation{{
			Phone:      strings.TrimSpace(params.Phone),
			FirstName:  strings.TrimSpace(params.FirstName),
			LastName:   strings.TrimSpace(params.LastName),
			NationalID: strings.TrimSpace(params.NationalID),
		}},
		ClinicCode:  params.ClinicCode,
		PatientCode: params.PatientCode,
		MessageText: params.MessageText,
		Messenger:   messenger,
		Operator:    c.operator,
		IP:          ip,
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v1/outbound/send", bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", auth)

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("messaging API status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	return nil
}
