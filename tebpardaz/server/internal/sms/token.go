package sms

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"time"
)

// AuthPayload is the JSON payload encrypted into the messaging Authorization token.
type AuthPayload struct {
	CenterCode string `json:"center_code"`
	IP         string `json:"ip"`
	Timestamp  string `json:"timestamp"`
	UserCode   string `json:"user_code"`
}

// TokenGenerator builds per-request Authorization tokens for the messaging API.
type TokenGenerator struct {
	aesKey   []byte
	hmacKey  []byte
	userCode string
}

// NewTokenGenerator constructs a TokenGenerator from base64-encoded keys.
// Inputs: aesKeyB64 and hmacKeyB64 (32 bytes each when decoded), userCode for payload.
// Output: TokenGenerator or error when keys are invalid.
func NewTokenGenerator(aesKeyB64, hmacKeyB64, userCode string) (*TokenGenerator, error) {
	aesKey, err := base64.StdEncoding.DecodeString(aesKeyB64)
	if err != nil {
		return nil, fmt.Errorf("decode messaging AES key: %w", err)
	}
	hmacKey, err := base64.StdEncoding.DecodeString(hmacKeyB64)
	if err != nil {
		return nil, fmt.Errorf("decode messaging HMAC key: %w", err)
	}
	if len(aesKey) != 32 {
		return nil, fmt.Errorf("messaging AES key must decode to 32 bytes")
	}
	if len(hmacKey) == 0 {
		return nil, fmt.Errorf("messaging HMAC key must not be empty")
	}
	if userCode == "" {
		userCode = "1"
	}
	return &TokenGenerator{
		aesKey:   aesKey,
		hmacKey:  hmacKey,
		userCode: userCode,
	}, nil
}

// Generate builds an Authorization token for the given clinic code and client IP.
// Inputs: centerCode (clinic HIS code), clientIP.
// Output: base64 token string or error.
func (g *TokenGenerator) Generate(centerCode int, clientIP string) (string, error) {
	if g == nil {
		return "", fmt.Errorf("token generator is nil")
	}
	if clientIP == "" {
		clientIP = "127.0.0.1"
	}
	return encryptPayload(AuthPayload{
		CenterCode: strconv.Itoa(centerCode),
		IP:         clientIP,
		Timestamp:  strconv.FormatInt(time.Now().Unix(), 10),
		UserCode:   g.userCode,
	}, g.aesKey, g.hmacKey)
}

// encryptPayload JSON-encrypts payload with AES-256-CBC and appends HMAC-SHA256.
// Inputs: payload, 32-byte AES key, HMAC key.
// Output: base64 token or error.
func encryptPayload(p AuthPayload, aesKey, hmacKey []byte) (string, error) {
	if len(aesKey) != 32 {
		return "", fmt.Errorf("AES key must be 32 bytes")
	}

	plainJSON, err := json.Marshal(p)
	if err != nil {
		return "", err
	}

	blockSize := aes.BlockSize
	padLen := blockSize - (len(plainJSON) % blockSize)
	for i := 0; i < padLen; i++ {
		plainJSON = append(plainJSON, byte(padLen))
	}

	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return "", err
	}

	iv := make([]byte, aes.BlockSize)
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return "", err
	}

	cipherText := make([]byte, len(plainJSON))
	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(cipherText, plainJSON)

	dataToSign := append(iv, cipherText...)
	mac := hmac.New(sha256.New, hmacKey)
	mac.Write(dataToSign)
	msgHMAC := mac.Sum(nil)

	full := append(dataToSign, msgHMAC...)
	return base64.StdEncoding.EncodeToString(full), nil
}
