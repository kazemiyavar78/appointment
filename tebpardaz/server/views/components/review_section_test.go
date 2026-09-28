package components

import (
	"context"
	"strings"
	"testing"
)

func TestReviewSection_emojiFormWithoutName(t *testing.T) {
	var buf strings.Builder
	err := ReviewSection(ReviewSectionView{
		FormAction: "/reviews",
		CSRFToken:  "tok",
		TargetType: "doctor",
		TargetID:   "4",
		ClinicID:   "2",
	}).Render(context.Background(), &buf)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	html := buf.String()
	if strings.Contains(html, `name="author_name"`) {
		t.Fatal("review form must not ask for the patient name")
	}
	if strings.Contains(html, "<select") {
		t.Fatal("review form must not use a rating select")
	}
	for _, want := range []string{`name="rating"`, `value="1"`, `value="5"`, "😍", "😞", "متن نظر (اختیاری)"} {
		if !strings.Contains(html, want) {
			t.Fatalf("review form missing %q", want)
		}
	}
}

func TestReviewSection_thanksAfterSubmit(t *testing.T) {
	var buf strings.Builder
	err := ReviewSection(ReviewSectionView{
		FormAction: "/reviews",
		Submitted:  true,
	}).Render(context.Background(), &buf)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	html := buf.String()
	for _, want := range []string{"از نظر ارزشمند شما سپاسگزاریم", "نظر شما حتماً بررسی می‌شود."} {
		if !strings.Contains(html, want) {
			t.Fatalf("thanks message missing %q", want)
		}
	}
	if strings.Contains(html, `name="rating"`) {
		t.Fatal("form should be hidden after a successful submit")
	}
}

func TestRatingEmoji(t *testing.T) {
	if got := RatingEmoji(5); got != "😍" {
		t.Fatalf("RatingEmoji(5) = %q", got)
	}
	if got := RatingLabel(1); got != "خیلی بد" {
		t.Fatalf("RatingLabel(1) = %q", got)
	}
	if got := AverageEmoji(4.6); got != "😍" {
		t.Fatalf("AverageEmoji(4.6) = %q", got)
	}
}
