package repository

import (
	"errors"
	"strings"
	"testing"
	"time"

	"tebpardaz/server/internal/models"
	"tebpardaz/server/internal/slug"

	"gorm.io/gorm"
)

func specRow(id uint, name, current string) models.Specialty {
	return models.Specialty{Model: gorm.Model{ID: id}, Name: name, Slug: current}
}

func TestPlanSpecialtySlugCreateShapes(t *testing.T) {
	rows := []models.Specialty{
		specRow(1, "داخلي", ""),
		specRow(2, "زنان و زایمان", ""),
		specRow(3, "دندان\u200cپزشکی", ""),
		specRow(4, "رئیس بخش", ""),
	}
	plan, err := PlanSpecialtySlugs(rows)
	if err != nil {
		t.Fatal(err)
	}
	want := map[uint]string{
		1: "داخلی",
		2: "زنان-و-زایمان",
		3: "دندان-پزشکی",
		4: "رئیس-بخش",
	}
	for id, slugValue := range want {
		if plan[id] != slugValue {
			t.Fatalf("id %d slug = %q, want %q", id, plan[id], slugValue)
		}
		if strings.Contains(slugValue, "specialty") {
			t.Fatal("invented fallback")
		}
	}
}

func TestPlanSpecialtySlugCollisionUsesID(t *testing.T) {
	rows := []models.Specialty{
		specRow(42, "داخلی", ""),
		specRow(7, "داخلي", ""),
	}
	plan, err := PlanSpecialtySlugs(rows)
	if err != nil {
		t.Fatal(err)
	}
	if plan[7] != "داخلی" || plan[42] != "داخلی-42" {
		t.Fatalf("plan = %#v", plan)
	}
}

func TestPlanSpecialtySlugKeepsExistingAndIsIdempotent(t *testing.T) {
	rows := []models.Specialty{
		specRow(5, "قلب و عروق", "slug-dasti"),
		specRow(9, "داخلی", "داخلی"),
	}
	first, err := PlanSpecialtySlugs(rows)
	if err != nil {
		t.Fatal(err)
	}
	if first[5] != "slug-dasti" || first[9] != "داخلی" {
		t.Fatalf("first = %#v", first)
	}
	rows[0].Slug = first[5]
	rows[1].Slug = first[9]
	rows[0].Name = "نام کاملاً متفاوت"
	second, err := PlanSpecialtySlugs(rows)
	if err != nil {
		t.Fatal(err)
	}
	if second[5] != "slug-dasti" || second[9] != "داخلی" {
		t.Fatalf("second = %#v", second)
	}
}

func TestPlanSpecialtySlugRejectsUnusableName(t *testing.T) {
	_, err := PlanSpecialtySlugs([]models.Specialty{specRow(8, "   ", "")})
	if err == nil || !strings.Contains(err.Error(), "8") {
		t.Fatalf("err = %v", err)
	}
	_, err = PlanSpecialtySlugs([]models.Specialty{specRow(11, "...", "")})
	if err == nil || !strings.Contains(err.Error(), "11") {
		t.Fatalf("punctuation err = %v", err)
	}
	if _, emptyErr := slug.MakePersian("..."); emptyErr == nil {
		t.Fatal("make should fail")
	}
}

func TestGetBySlugRejectsBlank(t *testing.T) {
	repo := &SpecialtyRepo{}
	for _, value := range []string{"", "  "} {
		if _, err := repo.GetBySlug(value); err != gorm.ErrRecordNotFound {
			t.Fatalf("GetBySlug(%q) = %v", value, err)
		}
	}
}

func TestSpecialtySlugOnUpdateKeepsValue(t *testing.T) {
	got, keep := specialtySlugOnUpdate("داخلی")
	if !keep || got != "داخلی" {
		t.Fatalf("update kept %q %v", got, keep)
	}
	if _, keep := specialtySlugOnUpdate("  "); keep {
		t.Fatal("blank slug must be allocated")
	}
}

func TestDecideCreateSlugHasNoSharedPlaceholder(t *testing.T) {
	chosen, explicitID, err := decideSpecialtyCreateSlug("داخلی", nil, 1)
	if err != nil || chosen != "داخلی" || explicitID != 0 {
		t.Fatalf("base = %q id=%d err=%v", chosen, explicitID, err)
	}
	if strings.Contains(chosen, "*") || strings.Contains(specialtyIdentityInsertSQL, "'*'") {
		t.Fatal("shared placeholder")
	}
	if !strings.Contains(specialtySlugLockSQL, "sp_getapplock") {
		t.Fatal("missing applock")
	}
	if err := validateSpecialtySlug("*"); err == nil {
		t.Fatal("placeholder must be invalid")
	}
}

func TestDecideCreateSlugCollisionUsesNextID(t *testing.T) {
	rows := []models.Specialty{specRow(7, "تخصص داخلی", "داخلی")}
	nextID := nextSpecialtyID(rows)
	if nextID != 8 {
		t.Fatalf("next id = %d", nextID)
	}
	chosen, explicitID, err := decideSpecialtyCreateSlug("داخلي", rows, nextID)
	if err != nil || chosen != "داخلی-8" || explicitID != 8 {
		t.Fatalf("chosen=%q id=%d err=%v", chosen, explicitID, err)
	}
}

func TestDecideCreateSlugFinalCollisionRollsBack(t *testing.T) {
	rows := []models.Specialty{
		specRow(7, "تخصص داخلی", "داخلی"),
		specRow(8, "سایر", "داخلی-9"),
	}
	chosen, explicitID, err := decideSpecialtyCreateSlug("داخلي", rows, 9)
	if !errors.Is(err, ErrSpecialtySlugUsed) || chosen != "" || explicitID != 0 {
		t.Fatalf("chosen=%q id=%d err=%v", chosen, explicitID, err)
	}
}

func TestDecideCreateSlugRejectsDuplicateName(t *testing.T) {
	rows := []models.Specialty{specRow(7, "داخلي", "داخلی")}
	_, _, err := decideSpecialtyCreateSlug("داخلی", rows, 8)
	if !errors.Is(err, ErrSpecialtyNameTaken) {
		t.Fatalf("err = %v", err)
	}
}

func TestPlanExistingDuplicateSlugDoesNotRewrite(t *testing.T) {
	rows := []models.Specialty{
		specRow(1, "قلب", "داخلی"),
		specRow(2, "عروق", "داخلی"),
		specRow(3, "زنان و زایمان", ""),
	}
	_, err := PlanSpecialtySlugs(rows)
	if err == nil || !strings.Contains(err.Error(), "id 1") || !strings.Contains(err.Error(), "id 2") {
		t.Fatalf("err = %v", err)
	}
	if rows[0].Slug != "داخلی" || rows[1].Slug != "داخلی" || rows[2].Slug != "" {
		t.Fatalf("rows rewritten: %#v", rows)
	}
}

func TestDeletedSlugStillCollides(t *testing.T) {
	row := specRow(7, "تخصص داخلی", "داخلی")
	row.DeletedAt = gorm.DeletedAt{Time: time.Now(), Valid: true}
	chosen, explicitID, err := decideSpecialtyCreateSlug("داخلي", []models.Specialty{row}, 11)
	if err != nil || chosen != "داخلی-11" || explicitID != 11 {
		t.Fatalf("chosen=%q id=%d err=%v", chosen, explicitID, err)
	}
}

func TestAllocateKeepsHamza(t *testing.T) {
	got, err := allocateSpecialtySlug("رئیس بخش", 3, map[string]uint{})
	if err != nil || got != "رئیس-بخش" {
		t.Fatalf("got %q %v", got, err)
	}
}
