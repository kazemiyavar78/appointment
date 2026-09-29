package pages

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"tebpardaz/server/views/components"
)

func TestBookingSpecialtyLinkPlatformOnly(t *testing.T) {
	var linked bytes.Buffer
	err := Booking(BookingView{
		DoctorName:    "رضا",
		SpecialtyName: "داخلی",
		SpecialtyURL:  "/specialties/dakheli",
		Slots:         components.SlotPickerView{},
	}).Render(context.Background(), &linked)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(linked.String(), `href="/specialties/dakheli"`) {
		t.Fatal(linked.String())
	}
	var plain bytes.Buffer
	err = Booking(BookingView{
		DoctorName:    "رضا",
		SpecialtyName: "داخلی",
		Slots:         components.SlotPickerView{},
	}).Render(context.Background(), &plain)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(plain.String(), "/specialties/") {
		t.Fatal(plain.String())
	}
	var clinic bytes.Buffer
	err = Booking(BookingView{
		DoctorName: "رضا",
		ClinicName: "چمران",
		ClinicURL:  "/clinics/chamran",
		ShowClinic: true,
		Slots:      components.SlotPickerView{},
	}).Render(context.Background(), &clinic)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(clinic.String(), `href="/clinics/chamran"`) {
		t.Fatal(clinic.String())
	}
}
