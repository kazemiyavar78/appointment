package public

import (
	"errors"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"tebpardaz/server/internal/filestore"
	"tebpardaz/server/internal/models"
	"tebpardaz/server/views/pages"

	"github.com/a-h/templ"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

var (
	fileMonthPattern = regexp.MustCompile(`^\d{4}-\d{2}$`)
	fileCodePattern  = regexp.MustCompile(`^\d+$`)
)

// FileDownloadHandler serves the patient file page from the shared storage root.
type FileDownloadHandler struct {
	db    *gorm.DB
	store *filestore.Store
}

// patientFile is a message attachment row stored in tp_managment.
type patientFile struct {
	ID                 uint
	ClinicID           uint
	Category           string
	OriginalFilename   string
	StoredRelativePath *string
	IsFileCreated      bool
	DeletedAt          gorm.DeletedAt
	Clinic             models.Clinic `gorm:"foreignKey:ClinicID;references:ID"`
}

// TableName returns the SmsService attachment table.
func (patientFile) TableName() string { return "message_attachments" }

// downloadAd is an active file-page advertisement row.
type downloadAd struct {
	ID               uint
	Title            string
	Description      string
	ImageFile        string
	LinkURL          string
	ButtonText       string
	OpenInNewTab     bool
	BadgeText        string
	ShowSeconds      int
	SkipAfterSeconds int
	StartsAt         *time.Time
	EndsAt           *time.Time
	SortOrder        int
	DeletedAt        gorm.DeletedAt
}

// publicAdSlide is one ad sent to the download page after the HTML has loaded.
type publicAdSlide struct {
	ID               uint   `json:"id"`
	ImageURL         string `json:"imageURL"`
	Title            string `json:"title"`
	Description      string `json:"description"`
	LinkURL          string `json:"linkURL"`
	ButtonText       string `json:"buttonText"`
	OpenInNewTab     bool   `json:"openInNewTab"`
	BadgeText        string `json:"badgeText"`
	ShowSeconds      int    `json:"showSeconds"`
	SkipAfterSeconds int    `json:"skipAfterSeconds"`
}

// TableName returns the clinic-project advertisement table.
func (downloadAd) TableName() string { return "file_download_ads" }

// NewFileDownloadHandler builds the public file page handler.
// Inputs: management DB and shared file store (store may be nil when FILE_STORAGE_ROOT is unset).
// Output: handler.
func NewFileDownloadHandler(db *gorm.DB, store *filestore.Store) *FileDownloadHandler {
	return &FileDownloadHandler{db: db, store: store}
}

// Show renders the download page for one stored patient file.
// Inputs: path category, month, clinic code, filename. Output: HTML or an error status.
func (h *FileDownloadHandler) Show(c *gin.Context) {
	file, rel, ok := h.lookup(c)
	if !ok {
		return
	}
	ready := file.IsFileCreated && file.StoredRelativePath != nil && strings.TrimSpace(*file.StoredRelativePath) != ""
	data := pages.DownloadPageData{
		ClinicName:    file.Clinic.Name,
		ClinicPhone:   file.Clinic.Phone,
		PageTitle:     "دانلود فایل | " + file.Clinic.Name,
		CategoryLabel: fileCategoryLabel(file.Category),
		Filename:      file.OriginalFilename,
		DownloadURL:   "/storage/" + rel + "/download",
		FileReady:     ready,
	}
	setNoStore(c)
	templ.Handler(pages.DownloadFile(data)).ServeHTTP(c.Writer, c.Request)
}

// Ads returns the clinic's current file-page ads as JSON.
// The HTML page asks for this after paint so a CDN-cached document still shows the latest campaign.
// Inputs: the same path params as Show. Output: {"slides":[...]} with no-store headers.
func (h *FileDownloadHandler) Ads(c *gin.Context) {
	file, _, ok := h.lookup(c)
	if !ok {
		return
	}
	setNoStore(c)
	slides, err := h.adSlides(file.ClinicID, time.Now())
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"slides": []publicAdSlide{}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"slides": slides})
}

// AdClick counts one click on an ad that belongs to the file's clinic.
// Inputs: path params plus ad_id. Output: 204, or the lookup error status.
func (h *FileDownloadHandler) AdClick(c *gin.Context) {
	file, _, ok := h.lookup(c)
	if !ok {
		return
	}
	id, err := strconv.ParseUint(strings.TrimSpace(c.Param("ad_id")), 10, 32)
	if err != nil || id == 0 || h.db == nil {
		c.Status(http.StatusNotFound)
		return
	}
	setNoStore(c)
	h.db.Model(&downloadAd{}).
		Where("id = ?", id).
		Where("EXISTS (SELECT 1 FROM file_download_ad_clinics AS link WHERE link.file_download_ad_id = file_download_ads.id AND link.clinic_id = ?)", file.ClinicID).
		UpdateColumn("click_count", gorm.Expr("click_count + ?", 1))
	c.Status(http.StatusNoContent)
}

// Download sends the stored file as an attachment.
// Inputs: the same path params as Show. Output: file bytes or an error status.
func (h *FileDownloadHandler) Download(c *gin.Context) {
	file, _, ok := h.lookup(c)
	if !ok {
		return
	}
	if h.store == nil || !file.IsFileCreated || file.StoredRelativePath == nil || strings.TrimSpace(*file.StoredRelativePath) == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "فایل هنوز آماده نیست"})
		return
	}
	abs, err := h.store.Absolute(*file.StoredRelativePath)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "فایل یافت نشد"})
		return
	}
	name := strings.TrimSpace(file.OriginalFilename)
	if name == "" {
		name = "file"
	}
	c.FileAttachment(abs, name)
}

// AdMedia serves a promotional image stored under the shared ads directory.
// Inputs: name path param. Output: image bytes or 404.
func (h *FileDownloadHandler) AdMedia(c *gin.Context) {
	if h.store == nil {
		c.Status(http.StatusNotFound)
		return
	}
	abs, err := h.store.AdImage(c.Param("name"))
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	c.File(abs)
}

// lookup loads the attachment addressed by the public storage path.
// Inputs: gin context path params. Output: row, relative key, and whether the response is still open.
func (h *FileDownloadHandler) lookup(c *gin.Context) (*patientFile, string, bool) {
	rel, ok := storageRelativePath(c)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "فایل یافت نشد"})
		return nil, "", false
	}
	if h.db == nil || h.store == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "مسیر ذخیره فایل تنظیم نشده است"})
		return nil, "", false
	}
	var file patientFile
	err := h.db.Preload("Clinic").Where("stored_relative_path = ?", rel).First(&file).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "فایل یافت نشد"})
			return nil, "", false
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "خواندن فایل انجام نشد"})
		return nil, "", false
	}
	return &file, rel, true
}

// adSlides loads active ads assigned to the clinic that are inside their schedule.
// Inputs: clinicID and now. Output: public slides, or an error from the database.
func (h *FileDownloadHandler) adSlides(clinicID uint, now time.Time) ([]publicAdSlide, error) {
	if h.db == nil || clinicID == 0 {
		return []publicAdSlide{}, nil
	}
	var ads []downloadAd
	err := h.db.Model(&downloadAd{}).
		Joins("INNER JOIN file_download_ad_clinics AS link ON link.file_download_ad_id = file_download_ads.id").
		Where("link.clinic_id = ? AND file_download_ads.is_active = ?", clinicID, true).
		Where("(file_download_ads.starts_at IS NULL OR file_download_ads.starts_at <= ?)", now).
		Where("(file_download_ads.ends_at IS NULL OR file_download_ads.ends_at >= ?)", now).
		Order("file_download_ads.sort_order ASC, file_download_ads.id ASC").
		Find(&ads).Error
	if err != nil {
		return nil, err
	}
	slides := make([]publicAdSlide, 0, len(ads))
	for _, ad := range ads {
		if !adIsLive(ad, now) {
			continue
		}
		name := strings.TrimSpace(ad.ImageFile)
		slides = append(slides, publicAdSlide{
			ID:               ad.ID,
			ImageURL:         "/file-ads/media/" + name,
			Title:            ad.Title,
			Description:      ad.Description,
			LinkURL:          sanitizePublicLink(ad.LinkURL),
			ButtonText:       strings.TrimSpace(ad.ButtonText),
			OpenInNewTab:     ad.OpenInNewTab,
			BadgeText:        strings.TrimSpace(ad.BadgeText),
			ShowSeconds:      clampAdSeconds(ad.ShowSeconds, 2, 30, 5),
			SkipAfterSeconds: clampAdSeconds(ad.SkipAfterSeconds, 0, 30, 0),
		})
	}
	return slides, nil
}

// adIsLive reports whether the ad has an image and its schedule includes now.
// Inputs: ad and now. Output: true when the slide may be shown.
func adIsLive(ad downloadAd, now time.Time) bool {
	if strings.TrimSpace(ad.ImageFile) == "" || strings.Contains(ad.ImageFile, "..") || strings.ContainsAny(ad.ImageFile, `/\`) {
		return false
	}
	if ad.StartsAt != nil && now.Before(*ad.StartsAt) {
		return false
	}
	if ad.EndsAt != nil && now.After(*ad.EndsAt) {
		return false
	}
	return true
}

// sanitizePublicLink keeps http(s) and same-site paths, and drops every other scheme.
// Inputs: raw stored link. Output: a safe URL or an empty string.
func sanitizePublicLink(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || len(raw) > 500 || strings.ContainsAny(raw, " \t\r\n\\") {
		return ""
	}
	if strings.HasPrefix(raw, "/") && !strings.HasPrefix(raw, "//") {
		return raw
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" || parsed.User != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return ""
	}
	return raw
}

// clampAdSeconds limits a duration and replaces out-of-range values with fallback.
// Inputs: n, inclusive bounds, and fallback. Output: the allowed number.
func clampAdSeconds(n, min, max, fallback int) int {
	if n < min || n > max {
		return fallback
	}
	return n
}

// setNoStore marks the response so browsers and CDNs should not reuse it.
// Inputs: gin context. Output: cache headers on the response.
func setNoStore(c *gin.Context) {
	c.Header("Cache-Control", "private, no-store, no-cache, max-age=0, must-revalidate")
	c.Header("Pragma", "no-cache")
	c.Header("Expires", "0")
	c.Header("CDN-Cache-Control", "no-store")
	c.Header("Surrogate-Control", "no-store")
}

// storageRelativePath builds the database key from route params.
// Inputs: gin context. Output: relative path and true when every segment is safe.
func storageRelativePath(c *gin.Context) (string, bool) {
	category := c.Param("category")
	month := c.Param("month")
	code := c.Param("clinic_code")
	name := c.Param("filename")
	if !safePathSegment(category) || !fileMonthPattern.MatchString(month) || !fileCodePattern.MatchString(code) || !safePathSegment(name) {
		return "", false
	}
	switch category {
	case "lab", "receipt", "promotional", "takmili", "support", "other":
	default:
		return "", false
	}
	return category + "/" + month + "/" + code + "/" + name, true
}

// safePathSegment reports whether a URL segment cannot escape the storage tree.
// Inputs: segment. Output: true when the segment is a single file or folder name.
func safePathSegment(segment string) bool {
	segment = strings.TrimSpace(segment)
	if segment == "" || segment == "." || segment == ".." {
		return false
	}
	return !strings.ContainsAny(segment, `/\`)
}

// fileCategoryLabel returns the Persian label shown on the download page.
// Inputs: stored category. Output: display label.
func fileCategoryLabel(category string) string {
	switch strings.TrimSpace(category) {
	case "lab":
		return "جواب آزمایش"
	case "receipt":
		return "رسید درمانگاه"
	case "promotional":
		return "محتوای تبلیغاتی"
	case "takmili":
		return "اطلاعات تکمیلی"
	case "support":
		return "پشتیبانی"
	default:
		return "فایل"
	}
}
