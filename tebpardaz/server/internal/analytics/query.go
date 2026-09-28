package analytics

import (
	"context"
	"errors"
	"strings"
	"time"

	"tebpardaz/server/internal/models"

	"gorm.io/gorm"
)

// VisitorIPFilter filters the admin list of unique visitor IPs.
type VisitorIPFilter struct {
	IPQuery           string
	ClinicID          uint
	PlatformOnly      bool
	RestrictClinicIDs []uint
	From              *time.Time
	To                *time.Time
	Page              int
	PerPage           int
}

// VisitDetailFilter filters the admin list of page visits.
type VisitDetailFilter struct {
	IPQuery           string
	VisitorIPID       uint
	Host              string
	PathQuery         string
	Browser           string
	OS                string
	FromGoogle        *bool
	ClinicID          uint
	PlatformOnly      bool
	RestrictClinicIDs []uint
	From              *time.Time
	To                *time.Time
	Page              int
	PerPage           int
}

// VisitorIPListResult is a paginated list of visitor IPs.
type VisitorIPListResult struct {
	Rows  []models.VisitorIP
	Total int64
}

// VisitDetailListResult is a paginated list of page visits with the parent IP preloaded.
type VisitDetailListResult struct {
	Rows  []models.VisitDetail
	Total int64
}

// KnownBrowsers returns the closed browser list used in analytics and admin filters.
// Inputs: none.
// Output: allowed browser names including Other.
func KnownBrowsers() []string {
	return []string{
		BrowserChrome, BrowserFirefox, BrowserSafari, BrowserEdge,
		BrowserOpera, BrowserSamsung, BrowserIE, BrowserOther,
	}
}

// KnownOperatingSystems returns the closed OS list used in analytics and admin filters.
// Inputs: none.
// Output: allowed OS names including Unknown.
func KnownOperatingSystems() []string {
	return []string{OSWindows, OSAndroid, OSIOS, OSMacOS, OSLinux, OSUnknown}
}

// ListVisitorIPs returns unique IPs matching the filter, newest last-visit first.
// Inputs: ctx for cancellation, filter with search/clinic/date/page.
// Output: rows and total count, or a database error.
func (r *GormVisitRepository) ListVisitorIPs(ctx context.Context, filter VisitorIPFilter) (VisitorIPListResult, error) {
	out := VisitorIPListResult{Rows: []models.VisitorIP{}}
	if r == nil || r.DB == nil {
		return out, nil
	}
	if filter.RestrictClinicIDs != nil && len(filter.RestrictClinicIDs) == 0 && !filter.PlatformOnly {
		return out, nil
	}
	page, perPage := normalizePage(filter.Page, filter.PerPage)
	q := applyVisitorIPFilter(r.DB.WithContext(ctx).Model(&models.VisitorIP{}), filter)
	if err := q.Count(&out.Total).Error; err != nil {
		return out, err
	}
	err := q.Order("last_visit_at desc").
		Offset((page - 1) * perPage).
		Limit(perPage).
		Find(&out.Rows).Error
	return out, err
}

// ListVisitDetails returns page visits matching the filter, newest first.
// Inputs: ctx for cancellation, filter with IP/path/browser/clinic/date/page.
// Output: rows (VisitorIP preloaded) and total count, or a database error.
func (r *GormVisitRepository) ListVisitDetails(ctx context.Context, filter VisitDetailFilter) (VisitDetailListResult, error) {
	out := VisitDetailListResult{Rows: []models.VisitDetail{}}
	if r == nil || r.DB == nil {
		return out, nil
	}
	if filter.RestrictClinicIDs != nil && len(filter.RestrictClinicIDs) == 0 && !filter.PlatformOnly {
		return out, nil
	}
	page, perPage := normalizePage(filter.Page, filter.PerPage)
	q := applyVisitDetailFilter(r.DB.WithContext(ctx).Model(&models.VisitDetail{}), filter)
	if err := q.Count(&out.Total).Error; err != nil {
		return out, err
	}
	err := q.Preload("VisitorIP").
		Order("visitor_visit_details.id desc").
		Offset((page - 1) * perPage).
		Limit(perPage).
		Find(&out.Rows).Error
	return out, err
}

// GetVisitorIPByAddress returns the unique visitor IP row for an exact address.
// Inputs: ctx for cancellation, ip (trimmed client address).
// Output: VisitorIP pointer, nil when missing, or a database error.
func (r *GormVisitRepository) GetVisitorIPByAddress(ctx context.Context, ip string) (*models.VisitorIP, error) {
	ip = strings.TrimSpace(ip)
	if r == nil || r.DB == nil || ip == "" {
		return nil, nil
	}
	var row models.VisitorIP
	err := r.DB.WithContext(ctx).Where("ip_address = ?", ip).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// applyVisitorIPFilter applies list filters onto a visitor_ips query.
func applyVisitorIPFilter(q *gorm.DB, filter VisitorIPFilter) *gorm.DB {
	if ip := strings.TrimSpace(filter.IPQuery); ip != "" {
		q = q.Where("ip_address LIKE ?", "%"+ip+"%")
	}
	if filter.From != nil {
		q = q.Where("last_visit_at >= ?", *filter.From)
	}
	if filter.To != nil {
		q = q.Where("last_visit_at <= ?", *filter.To)
	}
	if filter.PlatformOnly || filter.ClinicID > 0 || filter.RestrictClinicIDs != nil {
		sub := q.Session(&gorm.Session{NewDB: true}).Model(&models.VisitDetail{}).Select("visitor_ip_id")
		sub = applyVisitClinicScope(sub, "clinic_id", filter.ClinicID, filter.PlatformOnly, filter.RestrictClinicIDs)
		q = q.Where("id IN (?)", sub)
	}
	return q
}

// applyVisitDetailFilter applies list filters onto a visitor_visit_details query.
func applyVisitDetailFilter(q *gorm.DB, filter VisitDetailFilter) *gorm.DB {
	if filter.VisitorIPID > 0 {
		q = q.Where("visitor_visit_details.visitor_ip_id = ?", filter.VisitorIPID)
	}
	if ip := strings.TrimSpace(filter.IPQuery); ip != "" {
		q = q.Joins("JOIN visitor_ips ON visitor_ips.id = visitor_visit_details.visitor_ip_id").
			Where("visitor_ips.ip_address LIKE ?", "%"+ip+"%")
	}
	if host := strings.TrimSpace(filter.Host); host != "" {
		q = q.Where("visitor_visit_details.host LIKE ?", "%"+host+"%")
	}
	if path := strings.TrimSpace(filter.PathQuery); path != "" {
		like := "%" + path + "%"
		q = q.Where("(visitor_visit_details.path LIKE ? OR visitor_visit_details.full_url LIKE ? OR visitor_visit_details.referrer LIKE ?)", like, like, like)
	}
	if browser := strings.TrimSpace(filter.Browser); browser != "" {
		q = q.Where("visitor_visit_details.browser = ?", browser)
	}
	if osName := strings.TrimSpace(filter.OS); osName != "" {
		q = q.Where("visitor_visit_details.os = ?", osName)
	}
	if filter.FromGoogle != nil {
		q = q.Where("visitor_visit_details.is_from_google = ?", *filter.FromGoogle)
	}
	q = applyVisitClinicScope(q, "visitor_visit_details.clinic_id", filter.ClinicID, filter.PlatformOnly, filter.RestrictClinicIDs)
	if filter.From != nil {
		q = q.Where("visitor_visit_details.created_at >= ?", *filter.From)
	}
	if filter.To != nil {
		q = q.Where("visitor_visit_details.created_at <= ?", *filter.To)
	}
	return q
}

// applyVisitClinicScope restricts rows by clinic_id, platform-only (NULL), or an allow-list.
func applyVisitClinicScope(q *gorm.DB, col string, clinicID uint, platformOnly bool, restrict []uint) *gorm.DB {
	if col == "" {
		col = "clinic_id"
	}
	if platformOnly {
		return q.Where(col + " IS NULL")
	}
	if clinicID > 0 {
		return q.Where(col+" = ?", clinicID)
	}
	if restrict != nil {
		return q.Where(col+" IN ?", restrict)
	}
	return q
}

// normalizePage clamps page and per-page to safe defaults used by admin lists.
// Inputs: requested page and perPage.
// Output: page (>=1) and perPage in 1..200 (default 50).
func normalizePage(page, perPage int) (int, int) {
	if page < 1 {
		page = 1
	}
	if perPage <= 0 {
		perPage = 50
	}
	if perPage > 200 {
		perPage = 200
	}
	return page, perPage
}

// TotalPages returns how many pages a total count occupies.
// Inputs: total rows, perPage.
// Output: at least 1.
func TotalPages(total int64, perPage int) int {
	if perPage <= 0 {
		perPage = 50
	}
	n := int((total + int64(perPage) - 1) / int64(perPage))
	if n < 1 {
		return 1
	}
	return n
}

// IsKnownBrowser reports whether name is in the closed browser list.
// Inputs: name from the admin filter.
// Output: true when name is an allowed browser token.
func IsKnownBrowser(name string) bool {
	for _, b := range KnownBrowsers() {
		if b == name {
			return true
		}
	}
	return false
}

// IsKnownOS reports whether name is in the closed OS list.
// Inputs: name from the admin filter.
// Output: true when name is an allowed OS token.
func IsKnownOS(name string) bool {
	for _, osName := range KnownOperatingSystems() {
		if osName == name {
			return true
		}
	}
	return false
}
