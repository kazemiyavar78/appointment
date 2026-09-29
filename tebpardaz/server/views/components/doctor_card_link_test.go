package components

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestDoctorCardSpecialtyLink(t *testing.T) {
	var buf bytes.Buffer
	err := DoctorCard(DoctorCardView{
		Name:          "رضا",
		SpecialtyName: "داخلی",
		SpecialtyURL:  "/specialties/%D8%AF%D8%A7%D8%AE%D9%84%DB%8C",
		BookingURL:    "/booking/center/reza",
	}).Render(context.Background(), &buf)
	if err != nil {
		t.Fatal(err)
	}
	html := buf.String()
	if !strings.Contains(html, `href="/specialties/%D8%AF%D8%A7%D8%AE%D9%84%DB%8C"`) || !strings.Contains(html, "داخلی") {
		t.Fatal(html)
	}
	plain := bytes.Buffer{}
	err = DoctorCard(DoctorCardView{
		Name:          "رضا",
		SpecialtyName: "داخلی",
		BookingURL:    "/booking/reza",
	}).Render(context.Background(), &plain)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(plain.String(), "/specialties/") {
		t.Fatal(plain.String())
	}
	var clinic bytes.Buffer
	err = DoctorCard(DoctorCardView{
		Name:            "رضا",
		ClinicName:      "چمران",
		ShowClinicBadge: true,
		ClinicURL:       "/clinics/%D8%B3",
		BookingURL:      "/booking/chamran/reza",
	}).Render(context.Background(), &clinic)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(clinic.String(), `href="/clinics/%D8%B3"`) {
		t.Fatal(clinic.String())
	}
}
