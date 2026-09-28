package repository

import "testing"

// TestEscapeSQLServerLike نویسه‌های خاص الگوی LIKE را در SQL Server بی‌اثر می‌کند.
func TestEscapeSQLServerLike(t *testing.T) {
	got := escapeSQLServerLike(`100%_a[b]`)
	want := `100[%][_]a[[]b]`
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
