package booking

import "testing"

func TestIsValidIranianNationalID(t *testing.T) {
	// Well-known valid sample: 0013542419
	if !IsValidIranianNationalID("0013542419") {
		t.Fatal("expected valid national id")
	}
	if IsValidIranianNationalID("0000000000") {
		t.Fatal("all zeros should be invalid")
	}
	if IsValidIranianNationalID("123") {
		t.Fatal("short id should be invalid")
	}
}

func TestValidatePatientForm(t *testing.T) {
	form, err := ValidatePatientForm("علی", "رضایی", "0013542419", "09121234567", "1990-05-10", "مرد")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if form.BirthJalali == "" {
		t.Fatal("expected jalali birth date")
	}
	if form.SexCode != 1 {
		t.Fatalf("sex code want 1 got %d", form.SexCode)
	}

	_, err = ValidatePatientForm("Ali", "رضایی", "0013542419", "09121234567", "1990-05-10", "مرد")
	if err == nil {
		t.Fatal("expected error for latin first name")
	}
	_, err = ValidatePatientForm("علی", "رضایی", "0013542419", "09121234567", "1990-05-10", "نامشخص")
	if err == nil {
		t.Fatal("expected error for invalid sex")
	}
	_, err = ValidatePatientForm("علی", "رضایی", "0013542419", "08121234567", "1990-05-10", "مرد")
	if err == nil {
		t.Fatal("expected error for invalid mobile")
	}
}
