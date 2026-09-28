package news

import (
	"strings"
	"testing"
)

// TestSanitizeHTMLKeepsImageOrder ensures mixed text and images stay in document order.
func TestSanitizeHTMLKeepsImageOrder(t *testing.T) {
	raw := `<div>متن اول</div><img src="/static/uploads/news/a.jpg" alt=""><div>متن دوم</div><img src="/static/uploads/news/b.jpg" alt="">`
	got := SanitizeHTML(raw)
	first := strings.Index(got, "/static/uploads/news/a.jpg")
	secondText := strings.Index(got, "متن دوم")
	third := strings.Index(got, "/static/uploads/news/b.jpg")
	if first < 0 || secondText < 0 || third < 0 {
		t.Fatalf("missing expected fragments in %q", got)
	}
	if !(first < secondText && secondText < third) {
		t.Fatalf("images were reordered: %q", got)
	}
}

// TestSanitizeHTMLInlineImageStaysBetweenText keeps an image between two text runs.
func TestSanitizeHTMLInlineImageStaysBetweenText(t *testing.T) {
	raw := `سلام<img src="/static/uploads/news/mid.jpg" alt="">دنیا`
	got := SanitizeHTML(raw)
	hello := strings.Index(got, "سلام")
	img := strings.Index(got, "/static/uploads/news/mid.jpg")
	world := strings.Index(got, "دنیا")
	if hello < 0 || img < 0 || world < 0 || !(hello < img && img < world) {
		t.Fatalf("inline image order lost: %q", got)
	}
}

// TestSanitizeImgTagForcesBlockStyle writes display:block so RTL pages keep image position.
func TestSanitizeImgTagForcesBlockStyle(t *testing.T) {
	got := SanitizeHTML(`<img src="/static/uploads/news/x.jpg" alt="cover">`)
	if !strings.Contains(got, `style="display:block;max-width:100%;height:auto;margin:0.75rem 0;"`) {
		t.Fatalf("expected block image style, got %q", got)
	}
}
