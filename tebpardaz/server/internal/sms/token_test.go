package sms

import (
	"encoding/base64"
	"testing"
)

func TestEncryptPayload_roundTripFormat(t *testing.T) {
	aesKey := make([]byte, 32)
	hmacKey := make([]byte, 32)
	for i := range aesKey {
		aesKey[i] = byte(i)
	}
	for i := range hmacKey {
		hmacKey[i] = byte(i + 1)
	}

	token, err := encryptPayload(AuthPayload{
		CenterCode: "1001",
		IP:         "127.0.0.1",
		Timestamp:  "1700000000",
		UserCode:   "1",
	}, aesKey, hmacKey)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}
	if _, err := base64.StdEncoding.DecodeString(token); err != nil {
		t.Fatalf("token is not valid base64: %v", err)
	}
}

func TestTokenGenerator_Generate(t *testing.T) {
	aesKeyB64 := base64.StdEncoding.EncodeToString(make([]byte, 32))
	hmacKeyB64 := base64.StdEncoding.EncodeToString(make([]byte, 32))

	gen, err := NewTokenGenerator(aesKeyB64, hmacKeyB64, "1")
	if err != nil {
		t.Fatalf("NewTokenGenerator: %v", err)
	}
	token, err := gen.Generate(1001, "192.168.1.1")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if token == "" {
		t.Fatal("expected token")
	}
}

func TestNewTokenGenerator_invalidAESKey(t *testing.T) {
	_, err := NewTokenGenerator("dGVzdA==", base64.StdEncoding.EncodeToString(make([]byte, 32)), "1")
	if err == nil {
		t.Fatal("expected error for short AES key")
	}
}

func TestClient_authHeader_staticFallback(t *testing.T) {
	c, err := NewClient(Config{
		BaseURL:   "http://localhost",
		AuthToken: "static-token",
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	auth, err := c.authHeader(1001, "127.0.0.1")
	if err != nil {
		t.Fatalf("authHeader: %v", err)
	}
	if auth != "static-token" {
		t.Fatalf("got %q want static-token", auth)
	}
}

func TestClient_authHeader_dynamic(t *testing.T) {
	aesKeyB64 := base64.StdEncoding.EncodeToString(make([]byte, 32))
	hmacKeyB64 := base64.StdEncoding.EncodeToString(make([]byte, 32))
	c, err := NewClient(Config{
		BaseURL:    "http://localhost",
		AESKeyB64:  aesKeyB64,
		HMACKeyB64: hmacKeyB64,
		UserCode:   "1",
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	auth, err := c.authHeader(1001, "127.0.0.1")
	if err != nil {
		t.Fatalf("authHeader: %v", err)
	}
	if auth == "" {
		t.Fatal("expected dynamic token")
	}
}
