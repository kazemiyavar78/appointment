package db

import (
	"strings"
	"testing"
)

func TestSpecialtySlugSQL(t *testing.T) {
	if !strings.Contains(specialtySlugColumnSQL, "NVARCHAR(120) NULL") {
		t.Fatal("column must start nullable")
	}
	if !strings.Contains(specialtySlugNotNullSQL, "NOT NULL") {
		t.Fatal("not-null step missing")
	}
	if !strings.Contains(specialtySlugUniqueSQL, "CREATE UNIQUE INDEX UX_specialties_slug") {
		t.Fatal("unique index missing")
	}
	if strings.Contains(specialtySlugColumnSQL, "specialty") && strings.Contains(specialtySlugUniqueSQL, "DEFAULT 'specialty'") {
		t.Fatal("fake slug default")
	}
	if !strings.Contains(specialtySlugColumnSQL, "NOT EXISTS") {
		t.Fatal("column add must be idempotent")
	}
	if !strings.Contains(specialtySlugNotNullSQL, "is_nullable = 1") {
		t.Fatal("NOT NULL must skip when already enforced")
	}
	if !strings.Contains(specialtySlugUniqueSQL, "NOT EXISTS") {
		t.Fatal("unique index create must be idempotent")
	}
}

func TestSpecialtySlugIndexKeyFitsSQLServer(t *testing.T) {
	const chars = 120
	const bmpBytes = chars * 2
	const worstBytes = chars * 4
	const sqlServerKeyLimit = 900
	if bmpBytes != 240 || bmpBytes > sqlServerKeyLimit || worstBytes > sqlServerKeyLimit {
		t.Fatalf("key size bmp=%d worst=%d", bmpBytes, worstBytes)
	}
	if !strings.Contains(specialtySlugColumnSQL, "NVARCHAR(120)") || !strings.Contains(specialtySlugNotNullSQL, "NVARCHAR(120)") {
		t.Fatal("column length drifted")
	}
}
