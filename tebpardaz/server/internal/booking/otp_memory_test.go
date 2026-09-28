package booking

import (
	"errors"
	"testing"
)

// TestMemoryOTPStoreIssuesAndVerifiesACode covers the in-memory path used when no database is configured.
func TestMemoryOTPStoreIssuesAndVerifiesACode(t *testing.T) {
	store := NewOTPStore(nil)
	in := OTPCreateInput{Mobile: "09121111111", FirstName: "سارا", LastName: "رضایی", NationalID: "0012345678"}
	sent, _, err := store.CreateAndStore(in)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if len(sent.Code) != otpLength {
		t.Fatalf("code %q", sent.Code)
	}
	_, _, err = store.CreateAndStore(in)
	if !errors.Is(err, ErrOTPRateLimited) {
		t.Fatalf("resend: %v", err)
	}
	verified, err := store.Verify(in.Mobile, sent.Code)
	if err != nil || verified.Ticket == "" {
		t.Fatalf("verify: %v %+v", err, verified)
	}
}

// TestPendingCodeKeepsTheOriginalCodeUntilResend ensures a return visit can enter the code already sent.
func TestPendingCodeKeepsTheOriginalCodeUntilResend(t *testing.T) {
	store := NewOTPStore(nil)
	in := OTPCreateInput{Mobile: "09121111111", FirstName: "سارا", LastName: "رضایی", NationalID: "0012345678"}
	sent, _, err := store.CreateAndStore(in)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	wait, expiresAt, ok := store.PendingCode(in.Mobile, in.NationalID)
	if !ok || expiresAt.IsZero() {
		t.Fatal("expected pending code")
	}
	if wait <= 0 || wait > otpResendCooldown {
		t.Fatalf("resend wait %v", wait)
	}
	_, _, ok = store.PendingCode(in.Mobile, "0087654321")
	if ok {
		t.Fatal("different national id must not reuse the code")
	}
	verified, err := store.Verify(in.Mobile, sent.Code)
	if err != nil || verified.Ticket == "" {
		t.Fatalf("verify original: %v %+v", err, verified)
	}
	if _, _, ok = store.PendingCode(in.Mobile, in.NationalID); ok {
		t.Fatal("verified code must not stay pending")
	}
}
