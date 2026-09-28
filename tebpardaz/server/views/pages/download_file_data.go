package pages

// DownloadPageData is the view model for the patient file download page.
// Ads are not embedded here; the browser loads them after render so a cached page still gets the current campaign.
type DownloadPageData struct {
	ClinicName    string
	ClinicPhone   string
	PageTitle     string
	CategoryLabel string
	Filename      string
	DownloadURL   string
	FileReady     bool
}
