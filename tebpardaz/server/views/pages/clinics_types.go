package pages

import "tebpardaz/server/views/components"

// ClinicIndexItem یک مرکز در فهرست پلتفرم است.
type ClinicIndexItem struct {
	Name        string
	URL         string
	City        string
	Province    string
	Address     string
	Phone       string
	Description string
	LogoURL     string
}

// ClinicsView دادهٔ صفحه فهرست مراکز را نگه می‌دارد.
type ClinicsView struct {
	Items []ClinicIndexItem
}

// ClinicDetailView دادهٔ صفحه یک مرکز را نگه می‌دارد.
type ClinicDetailView struct {
	Name          string
	Slug          string
	Description   string
	Address       string
	Phone         string
	City          string
	Province      string
	LogoURL       string
	Doctors       []components.DoctorCardView
	Page          int
	TotalPages    int
	TotalCount    int
	Empty         bool
	ShowDirectory bool
}
