package pages

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestDownloadFileLoadsAdsAfterRender(t *testing.T) {
	var buf bytes.Buffer
	err := DownloadFile(DownloadPageData{
		ClinicName:    "درمانگاه نمونه",
		PageTitle:     "دانلود فایل",
		CategoryLabel: "جواب آزمایش",
		Filename:      "result.pdf",
		DownloadURL:   "/storage/lab/2026-09/12/result.pdf/download",
		FileReady:     true,
	}).Render(context.Background(), &buf)
	if err != nil {
		t.Fatal(err)
	}
	html := buf.String()
	if strings.Contains(html, `id="overlay"`) {
		t.Fatal("ad markup was embedded in the cached page")
	}
	if !strings.Contains(html, `id="adRoot"`) || !strings.Contains(html, "/ads?_=") || !strings.Contains(html, "method:'POST'") {
		t.Fatal("page does not request fresh ads")
	}
	if !strings.Contains(html, "دانلود فایل") || !strings.Contains(html, "درمانگاه نمونه") {
		t.Fatal("download page content missing")
	}
}

func TestDownloadFileSkipsAdRequestWhenFileIsNotReady(t *testing.T) {
	var buf bytes.Buffer
	err := DownloadFile(DownloadPageData{
		ClinicName:  "درمانگاه نمونه",
		PageTitle:   "دانلود فایل",
		Filename:    "result.pdf",
		DownloadURL: "/storage/lab/2026-09/12/result.pdf/download",
		FileReady:   false,
	}).Render(context.Background(), &buf)
	if err != nil {
		t.Fatal(err)
	}
	html := buf.String()
	if strings.Contains(html, "/ads?_=") || !strings.Contains(html, "فایل هنوز آماده نیست") {
		t.Fatal("not-ready page should not load ads")
	}
}
