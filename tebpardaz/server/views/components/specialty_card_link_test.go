package components

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestSpecialtyCardHrefPlatformAndFilter(t *testing.T) {
	var landing bytes.Buffer
	err := SpecialtyCard(SpecialtyCardView{
		ID:   2,
		Name: "داخلی",
		Href: "/specialties/%D8%AF%D8%A7%D8%AE%D9%84%DB%8C",
	}).Render(context.Background(), &landing)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(landing.String(), `href="/specialties/%D8%AF%D8%A7%D8%AE%D9%84%DB%8C"`) || strings.Contains(landing.String(), "%25") {
		t.Fatal(landing.String())
	}

	var filter bytes.Buffer
	err = SpecialtyCard(SpecialtyCardView{ID: 8, Name: "چشم"}).Render(context.Background(), &filter)
	if err != nil {
		t.Fatal(err)
	}
	html := filter.String()
	if !strings.Contains(html, `href="/doctors?specialty_id=8"`) || strings.Contains(html, "/specialties/") {
		t.Fatal(html)
	}
}
