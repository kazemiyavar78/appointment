package admin

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"tebpardaz/server/internal/auth"
	"tebpardaz/server/internal/branding"
	"tebpardaz/server/internal/cache"
	"tebpardaz/server/internal/models"
	"tebpardaz/server/internal/repository"
	"tebpardaz/server/internal/websocket"
	adminviews "tebpardaz/server/views/admin"
	"tebpardaz/shared/constants"
	"tebpardaz/shared/protocol"

	"github.com/gin-gonic/gin"
)

const (
	liveDoctorListTimeout = 15 * time.Second
	defaultPageSize       = 20
	doctorUploadDir       = "static/uploads/doctors"
	tabPending            = "pending"
	tabApproved           = "approved"
)

// ApprovalHandler manages doctor approval (pending from cache, approved from DB).
type ApprovalHandler struct {
	Doctors        *repository.DoctorRepo
	Specialties    *repository.SpecialtyRepo
	Clinics        *repository.ClinicRepo
	Scope          *auth.ClinicScope
	Hub            *websocket.Hub
	PendingDoctors *cache.DoctorCache
}

// NewApprovalHandler constructs an ApprovalHandler.
// Inputs: doctors, specialties, clinics, scope, hub, pending doctor cache.
// Output: ready ApprovalHandler.
func NewApprovalHandler(
	doctors *repository.DoctorRepo,
	specialties *repository.SpecialtyRepo,
	clinics *repository.ClinicRepo,
	scope *auth.ClinicScope,
	hub *websocket.Hub,
	pending *cache.DoctorCache,
) *ApprovalHandler {
	return &ApprovalHandler{
		Doctors:        doctors,
		Specialties:    specialties,
		Clinics:        clinics,
		Scope:          scope,
		Hub:            hub,
		PendingDoctors: pending,
	}
}

// ListPending renders the doctor approval page with pending (cache) and approved (DB) tabs.
// Inputs: gin context (query: clinic_id, tab, q, page, fetch).
// Output: HTML page.
func (h *ApprovalHandler) ListPending(c *gin.Context) {
	user, ok := auth.UserFromGin(c)
	if !ok {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", strconv.Itoa(defaultPageSize)))
	search := strings.TrimSpace(c.Query("q"))
	fetchLive := c.Query("fetch") == "1"
	tab := c.DefaultQuery("tab", tabPending)
	if tab != tabApproved {
		tab = tabPending
	}

	allowed, err := h.Scope.AllowedClinics(user)
	if err != nil {
		c.String(http.StatusInternalServerError, "failed to load clinics")
		return
	}
	if len(allowed) == 0 {
		h.render(c, user, adminviews.DoctorApprovalView{
			Nav:     adminviews.BuildAdminNav(adminviews.NavApprovals, user.Role),
			Tab:     tab,
			Message: "هیچ مرکزی برای حساب شما تعریف نشده است.",
		})
		return
	}

	clinicID := h.resolveClinicID(c, allowed)
	if clinicID == 0 {
		clinicID = allowed[0].ID
	}
	if !h.Scope.CanAccess(user, clinicID) {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	onlineSet := make(map[uint]struct{})
	for _, id := range h.Hub.OnlineClinicIDs() {
		onlineSet[id] = struct{}{}
	}

	specs, _ := h.Specialties.ListAll()
	view := adminviews.DoctorApprovalView{
		Nav:           adminviews.BuildAdminNav(adminviews.NavApprovals, user.Role),
		ClinicID:      clinicID,
		Clinics:       h.clinicOptions(allowed, onlineSet, clinicID),
		Specialties:   toSpecialtyOptions(specs),
		SearchQuery:   search,
		Page:          page,
		PerPage:       perPage,
		FetchLive:     fetchLive,
		Tab:           tab,
		ClinicLogoURL: h.clinicLogoURL(clinicID),
	}

	if fetchLive {
		if _, online := onlineSet[clinicID]; !online {
			view.Message = "مرکز انتخاب‌شده آنلاین نیست. لیست کش‌شده نمایش داده می‌شود."
		} else if _, liveErr := h.Hub.RequestDoctorList(clinicID, liveDoctorListTimeout); liveErr != nil {
			view.Message = liveListErrorMessage(liveErr)
		} else {
			view.Message = "لیست زنده از مرکز دریافت و کش به‌روز شد."
		}
	}

	var allItems []adminviews.ApprovalItem
	switch tab {
	case tabApproved:
		items, loadErr := h.approvedItems(clinicID)
		if loadErr != nil {
			c.String(http.StatusInternalServerError, "failed to load approved doctors")
			return
		}
		allItems = items
	default:
		allItems = h.pendingItems(clinicID)
	}

	filtered := filterItems(allItems, search)
	view.TotalItems = len(filtered)
	view.TotalPages = paginateTotalPages(view.TotalItems, perPage)
	if view.Page < 1 {
		view.Page = 1
	}
	if view.TotalPages > 0 && view.Page > view.TotalPages {
		view.Page = view.TotalPages
	}
	view.Items = paginateSlice(filtered, view.Page, perPage)

	if view.Message == "" && len(allItems) == 0 {
		if tab == tabApproved {
			view.Message = "پزشک تأییدشده‌ای برای این مرکز وجود ندارد."
		} else {
			view.Message = "پزشک تأییدنشده‌ای در کش نیست. «دریافت لیست زنده» را بزنید یا منتظر سینک خودکار کلینیک بمانید."
		}
	}
	h.render(c, user, view)
}

// Decide approves (creates DB row with photo+specialty) or rejects (removes from cache) a pending doctor.
// Inputs: gin context form (national_id, clinic_id, decision, specialty_id, photo file/url).
// Output: redirect to approvals page.
func (h *ApprovalHandler) Decide(c *gin.Context) {
	user, ok := auth.UserFromGin(c)
	if !ok {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	nationalID := strings.TrimSpace(c.PostForm("national_id"))
	clinicIDStr := c.PostForm("clinic_id")
	decision := c.PostForm("decision")
	note := c.PostForm("note")
	approve := decision == "approve"
	tab := c.DefaultPostForm("tab", tabPending)

	clinicID, err := strconv.ParseUint(clinicIDStr, 10, 64)
	if err != nil || clinicID == 0 || nationalID == "" {
		c.String(http.StatusBadRequest, "clinic_id و national_id الزامی است")
		return
	}
	if !h.Scope.CanAccess(user, uint(clinicID)) {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	pending, found := h.PendingDoctors.GetPending(uint(clinicID), nationalID)
	if !found {
		c.String(http.StatusNotFound, "پزشک در لیست تأییدنشده یافت نشد")
		return
	}

	if !approve {
		h.PendingDoctors.RemovePending(uint(clinicID), nationalID)
		c.Redirect(http.StatusFound, buildRedirect(clinicIDStr, tab, c))
		return
	}

	specialtyID, err := strconv.ParseUint(c.PostForm("specialty_id"), 10, 64)
	if err != nil || specialtyID == 0 {
		c.String(http.StatusBadRequest, "انتخاب تخصص الزامی است")
		return
	}
	useLogo := formUseClinicLogo(c)
	var photoURL, photo300, photo600, photo900, photo1200 string
	if useLogo {
		logo := h.clinicLogoURL(uint(clinicID))
		if logo == "" {
			c.String(http.StatusBadRequest, "لوگوی مرکز بارگذاری نشده است. ابتدا لوگو را در برندینگ مرکز آپلود کنید")
			return
		}
		photoURL, photo300, photo600, photo900, photo1200 = logo, logo, logo, logo, logo
	} else {
		photoURL, err = h.resolvePhotoURL(c, "")
		if err != nil {
			c.String(http.StatusBadRequest, err.Error())
			return
		}
		photo300, err = h.resolvePhotoFile(c, "photo_300", "photo_300_url", "")
		if err != nil {
			c.String(http.StatusBadRequest, err.Error())
			return
		}
		photo600, err = h.resolvePhotoFile(c, "photo_600", "photo_600_url", "")
		if err != nil {
			c.String(http.StatusBadRequest, err.Error())
			return
		}
		photo900, err = h.resolvePhotoFile(c, "photo_900", "photo_900_url", "")
		if err != nil {
			c.String(http.StatusBadRequest, err.Error())
			return
		}
		photo1200, err = h.resolvePhotoFile(c, "photo_1200", "photo_1200_url", "")
		if err != nil {
			c.String(http.StatusBadRequest, err.Error())
			return
		}
		if photo1200 == "" && photoURL != "" {
			photo1200 = photoURL
		}
		if photoURL == "" && photo1200 != "" {
			photoURL = photo1200
		}
		if strings.TrimSpace(photoURL) == "" && strings.TrimSpace(photo1200) == "" {
			c.String(http.StatusBadRequest, "آپلود عکس پزشک قبل از تأیید الزامی است")
			return
		}
	}

	req, err := h.Doctors.CreateApproved(repository.ApproveInput{
		ClinicID:       uint(clinicID),
		LocalCode:      pending.LocalCode,
		NationalID:     pending.NationalID,
		FirstName:      pending.FirstName,
		LastName:       pending.LastName,
		Name:           pending.Name,
		Mobile:         pending.Mobile,
		DoctorSystemID: pending.DoctorSystemID,
		SpecialtyCode:  pending.SpecialtyCode,
		SpecialtyID:    uint(specialtyID),
		PhotoURL:       photoURL,
		Photo300:       photo300,
		Photo600:       photo600,
		Photo900:       photo900,
		Photo1200:      photo1200,
		UseClinicLogo:  useLogo,
		IsActive:       pending.IsActive,
		ReviewerID:     user.ID,
		Note:           note,
	})
	if err != nil {
		c.String(http.StatusInternalServerError, "ثبت پزشک تأییدشده ناموفق بود: "+err.Error())
		return
	}

	h.PendingDoctors.RemovePending(uint(clinicID), nationalID)
	h.notifyClinic(req, pending.LocalCode)
	c.Redirect(http.StatusFound, buildRedirect(clinicIDStr, tabApproved, c))
}

// UpdateApproved updates photo and/or specialty for an already-approved doctor.
// Inputs: gin context form (doctor_id, clinic_id, specialty_id, photo_300, photo_600, photo_900, photo_1200, photo).
// Output: redirect to approved tab.
func (h *ApprovalHandler) UpdateApproved(c *gin.Context) {
	user, ok := auth.UserFromGin(c)
	if !ok {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	doc, clinicIDStr, ok := h.loadApprovedDoctorForAction(c, user)
	if !ok {
		return
	}

	var specialtyID uint
	if sid, err := strconv.ParseUint(c.PostForm("specialty_id"), 10, 64); err == nil {
		specialtyID = uint(sid)
	}
	if specialtyID > 0 {
		if _, err := h.Specialties.GetByID(specialtyID); err != nil {
			c.String(http.StatusBadRequest, "تخصص انتخاب‌شده معتبر نیست")
			return
		}
	}
	useLogo := formUseClinicLogo(c)
	photos := repository.DoctorPhotosUpdate{UseClinicLogo: &useLogo}
	if useLogo {
		logo := h.clinicLogoURL(doc.ClinicID)
		if logo == "" {
			c.String(http.StatusBadRequest, "لوگوی مرکز بارگذاری نشده است. ابتدا لوگو را در برندینگ مرکز آپلود کنید")
			return
		}
		photos.LogoURL = logo
	} else {
		photoURL, err := h.resolvePhotoURL(c, "")
		if err != nil {
			c.String(http.StatusBadRequest, err.Error())
			return
		}
		photo300, err := h.resolvePhotoFile(c, "photo_300", "photo_300_url", "")
		if err != nil {
			c.String(http.StatusBadRequest, err.Error())
			return
		}
		photo600, err := h.resolvePhotoFile(c, "photo_600", "photo_600_url", "")
		if err != nil {
			c.String(http.StatusBadRequest, err.Error())
			return
		}
		photo900, err := h.resolvePhotoFile(c, "photo_900", "photo_900_url", "")
		if err != nil {
			c.String(http.StatusBadRequest, err.Error())
			return
		}
		photo1200, err := h.resolvePhotoFile(c, "photo_1200", "photo_1200_url", "")
		if err != nil {
			c.String(http.StatusBadRequest, err.Error())
			return
		}
		if strings.TrimSpace(photoURL) == "" && strings.TrimSpace(photo1200) == "" && (doc.UseClinicLogo || !doctorHasStoredPhoto(doc)) {
			c.String(http.StatusBadRequest, "آپلود عکس پزشک الزامی است")
			return
		}
		photos.PhotoURL = photoURL
		photos.Photo300 = photo300
		photos.Photo600 = photo600
		photos.Photo900 = photo900
		photos.Photo1200 = photo1200
		photos.ReplacePhotos = doc.UseClinicLogo
	}
	shortDesc := c.PostForm("short_desc")
	longDesc := c.PostForm("long_desc")
	if _, err := h.Doctors.UpdateApprovedProfile(doc.ID, specialtyID, photos, shortDesc, longDesc, true); err != nil {
		c.String(http.StatusInternalServerError, "بروزرسانی ناموفق بود")
		return
	}
	c.Redirect(http.StatusFound, buildRedirect(clinicIDStr, tabApproved, c))
}

// SetActive activates or deactivates an approved doctor on the website.
// Inputs: gin context form (doctor_id, clinic_id, active=1|0).
// Output: redirect to approved tab.
func (h *ApprovalHandler) SetActive(c *gin.Context) {
	user, ok := auth.UserFromGin(c)
	if !ok {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	doc, clinicIDStr, ok := h.loadApprovedDoctorForAction(c, user)
	if !ok {
		return
	}
	active := c.PostForm("active") == "1"
	if err := h.Doctors.SetActive(doc.ID, active); err != nil {
		c.String(http.StatusInternalServerError, "تغییر وضعیت ناموفق بود")
		return
	}
	c.Redirect(http.StatusFound, buildRedirect(clinicIDStr, tabApproved, c))
}

// DeleteApproved soft-deletes an approved doctor and notifies the clinic.
// Inputs: gin context form (doctor_id, clinic_id).
// Output: redirect to approved tab.
func (h *ApprovalHandler) DeleteApproved(c *gin.Context) {
	user, ok := auth.UserFromGin(c)
	if !ok {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	doc, clinicIDStr, ok := h.loadApprovedDoctorForAction(c, user)
	if !ok {
		return
	}
	if err := h.Doctors.DeleteApproved(doc.ID); err != nil {
		c.String(http.StatusInternalServerError, "حذف پزشک ناموفق بود")
		return
	}
	h.notifyClinic(&models.DoctorApprovalRequest{
		ClinicID:   doc.ClinicID,
		DoctorID:   &doc.ID,
		ExternalID: doc.ExternalID,
		Status:     string(constants.ApprovalRejected),
	}, doc.LocalCode)
	c.Redirect(http.StatusFound, buildRedirect(clinicIDStr, tabApproved, c))
}

// loadApprovedDoctorForAction loads an approved doctor from form doctor_id and checks clinic scope.
// Inputs: gin context, acting user.
// Output: doctor, clinicID string for redirect, and ok=false when response already written.
func (h *ApprovalHandler) loadApprovedDoctorForAction(
	c *gin.Context,
	user *models.AppointmentUser,
) (*models.Doctor, string, bool) {
	doctorID, err := strconv.ParseUint(c.PostForm("doctor_id"), 10, 64)
	if err != nil || doctorID == 0 {
		c.String(http.StatusBadRequest, "invalid doctor_id")
		return nil, "", false
	}
	doc, err := h.Doctors.GetByID(uint(doctorID))
	if err != nil || doc == nil || !doc.IsApproved {
		c.String(http.StatusNotFound, "پزشک یافت نشد")
		return nil, "", false
	}
	if !h.Scope.CanAccess(user, doc.ClinicID) {
		c.AbortWithStatus(http.StatusForbidden)
		return nil, "", false
	}
	clinicIDStr := c.PostForm("clinic_id")
	if clinicIDStr == "" {
		clinicIDStr = strconv.FormatUint(uint64(doc.ClinicID), 10)
	}
	return doc, clinicIDStr, true
}

// formUseClinicLogo reports whether the admin chose the clinic logo instead of a personal photo.
// Inputs: gin context. Output: true when use_clinic_logo is posted as "1".
func formUseClinicLogo(c *gin.Context) bool {
	return c.PostForm("use_clinic_logo") == "1"
}

// clinicLogoURL returns the clinic logo URL, preferring the uploaded LogoURL.
// Inputs: clinicID. Output: public logo URL, or empty when the clinic has none.
func (h *ApprovalHandler) clinicLogoURL(clinicID uint) string {
	if h == nil || h.Clinics == nil || clinicID == 0 {
		return ""
	}
	clinic, err := h.Clinics.GetByIDForAdmin(clinicID)
	if err != nil || clinic == nil {
		return ""
	}
	if logo := strings.TrimSpace(clinic.LogoURL); logo != "" {
		return logo
	}
	return branding.ClinicLogoURL(clinic)
}

// doctorHasStoredPhoto reports whether any personal photo URL is already stored.
// Inputs: doctor pointer. Output: true when at least one photo column is non-empty.
func doctorHasStoredPhoto(doc *models.Doctor) bool {
	if doc == nil {
		return false
	}
	return strings.TrimSpace(doc.PhotoURL) != "" ||
		strings.TrimSpace(doc.Photo300) != "" ||
		strings.TrimSpace(doc.Photo600) != "" ||
		strings.TrimSpace(doc.Photo900) != "" ||
		strings.TrimSpace(doc.Photo1200) != ""
}

// resolvePhotoFile returns uploaded file URL, form URL, or fallback for a specific form field name.
// Inputs: gin context c, fileField form file key, urlField text url key, fallback string.
// Output: public photo URL or error.
func (h *ApprovalHandler) resolvePhotoFile(c *gin.Context, fileField, urlField, fallback string) (string, error) {
	file, hdr, err := c.Request.FormFile(fileField)
	if err == nil && file != nil {
		defer file.Close()
		ext := strings.ToLower(filepath.Ext(hdr.Filename))
		switch ext {
		case ".jpg", ".jpeg", ".png", ".gif", ".webp":
		default:
			return "", fmt.Errorf("فرمت تصویر مجاز نیست")
		}
		if hdr.Size > 5<<20 {
			return "", fmt.Errorf("حجم تصویر نباید بیش از ۵ مگابایت باشد")
		}
		if err := os.MkdirAll(doctorUploadDir, 0o755); err != nil {
			return "", fmt.Errorf("خطا در ذخیره فایل")
		}
		name := fmt.Sprintf("%d_%s%s", time.Now().UnixNano(), fileField, ext)
		destPath := filepath.Join(doctorUploadDir, name)
		out, createErr := os.Create(destPath)
		if createErr != nil {
			return "", fmt.Errorf("خطا در ذخیره فایل")
		}
		defer out.Close()
		if _, copyErr := io.Copy(out, file); copyErr != nil {
			return "", fmt.Errorf("خطا در ذخیره فایل")
		}
		return "/static/uploads/doctors/" + name, nil
	}
	if urlField != "" {
		if url := strings.TrimSpace(c.PostForm(urlField)); url != "" {
			return url, nil
		}
	}
	return strings.TrimSpace(fallback), nil
}

// resolvePhotoURL returns uploaded file URL, form photo_url, or fallback.
// Inputs: gin context, fallback URL.
// Output: public photo URL or error.
func (h *ApprovalHandler) resolvePhotoURL(c *gin.Context, fallback string) (string, error) {
	return h.resolvePhotoFile(c, "photo", "photo_url", fallback)
}

// buildRedirect builds the post-action redirect URL preserving filters.
// Inputs: clinicIDStr, tab, gin context (q, page).
// Output: path with query string.
func buildRedirect(clinicIDStr, tab string, c *gin.Context) string {
	redirect := "/admin/approvals?tab=" + tab
	if clinicIDStr != "" {
		redirect += "&clinic_id=" + clinicIDStr
	}
	if q := c.PostForm("q"); q != "" {
		redirect += "&q=" + q
	}
	if p := c.PostForm("page"); p != "" {
		redirect += "&page=" + p
	}
	return redirect
}

// resolveClinicID reads clinic_id from the query string when valid.
// Inputs: gin context, allowed clinics.
// Output: clinic id or 0.
func (h *ApprovalHandler) resolveClinicID(c *gin.Context, allowed []models.Clinic) uint {
	if raw := c.Query("clinic_id"); raw != "" {
		if id, err := strconv.ParseUint(raw, 10, 64); err == nil {
			return uint(id)
		}
	}
	return 0
}

// clinicOptions maps clinics to selector options with online state.
// Inputs: allowed clinics, online set, selected id.
// Output: ClinicOption slice.
func (h *ApprovalHandler) clinicOptions(allowed []models.Clinic, online map[uint]struct{}, selected uint) []adminviews.ClinicOption {
	opts := make([]adminviews.ClinicOption, 0, len(allowed))
	for _, cl := range allowed {
		_, isOnline := online[cl.ID]
		opts = append(opts, adminviews.ClinicOption{
			ID:       cl.ID,
			Name:     cl.Name,
			IsOnline: isOnline,
			Selected: cl.ID == selected,
		})
	}
	return opts
}

// toSpecialtyOptions maps specialty models to view options.
// Inputs: specialty rows.
// Output: SpecialtyOption slice.
func toSpecialtyOptions(specs []models.Specialty) []adminviews.SpecialtyOption {
	opts := make([]adminviews.SpecialtyOption, 0, len(specs))
	for _, s := range specs {
		opts = append(opts, adminviews.SpecialtyOption{ID: s.ID, Name: s.Name})
	}
	return opts
}

// pendingItems builds approval rows from the pending doctor cache.
// Inputs: clinicID.
// Output: ApprovalItem slice (CanDecide=true).
func (h *ApprovalHandler) pendingItems(clinicID uint) []adminviews.ApprovalItem {
	if h.PendingDoctors == nil {
		return nil
	}
	rows := h.PendingDoctors.ListPending(clinicID)
	items := make([]adminviews.ApprovalItem, 0, len(rows))
	for _, d := range rows {
		items = append(items, adminviews.ApprovalItem{
			ClinicID:       d.ClinicID,
			LocalCode:      d.LocalCode,
			FirstName:      d.FirstName,
			LastName:       d.LastName,
			Name:           d.Name,
			NationalID:     d.NationalID,
			Mobile:         d.Mobile,
			DoctorSystemID: d.DoctorSystemID,
			SpecialtyCode:  d.SpecialtyCode,
			PhotoURL:       d.PhotoURL,
			ExternalID:     d.ExternalID,
			IsActive:       d.IsActive,
			Source:         "clinic",
			Status:         string(constants.ApprovalPending),
			CanDecide:      true,
		})
	}
	return items
}

// approvedItems builds approval rows from approved doctors in the database.
// Inputs: clinicID.
// Output: ApprovalItem slice or error.
func (h *ApprovalHandler) approvedItems(clinicID uint) ([]adminviews.ApprovalItem, error) {
	rows, err := h.Doctors.ListApprovedByClinic(clinicID)
	if err != nil {
		return nil, err
	}
	items := make([]adminviews.ApprovalItem, 0, len(rows))
	for _, doc := range rows {
		spec := ""
		if doc.Specialty.ID != 0 {
			spec = doc.Specialty.Name
		}
		items = append(items, adminviews.ApprovalItem{
			DoctorID:       doc.ID,
			ClinicID:       doc.ClinicID,
			LocalCode:      doc.LocalCode,
			FirstName:      doc.FirstName,
			LastName:       doc.LastName,
			Name:           doc.Name,
			NationalID:     doc.NationalID,
			Mobile:         doc.Mobile,
			DoctorSystemID: doc.DoctorSystemID,
			SpecialtyCode:  doc.SpecialtyCode,
			SpecialtyID:    doc.SpecialtyID,
			SpecialtyName:  spec,
			PhotoURL:       doc.PhotoURL,
			Photo300:       doc.Photo300,
			Photo600:       doc.Photo600,
			Photo900:       doc.Photo900,
			Photo1200:      doc.Photo1200,
			UseClinicLogo:  doc.UseClinicLogo,
			ExternalID:     doc.ExternalID,
			ShortDesc:      doc.ShortDesc,
			LongDesc:       doc.LongDesc,
			IsActive:       doc.IsActive,
			Source:         "server",
			Status:         string(constants.ApprovalApproved),
			CanDecide:      false,
		})
	}
	return items, nil
}

// filterItems filters approval items by a free-text query.
// Inputs: items, query string.
// Output: matching items.
func filterItems(items []adminviews.ApprovalItem, q string) []adminviews.ApprovalItem {
	q = strings.ToLower(strings.TrimSpace(q))
	if q == "" {
		return items
	}
	out := make([]adminviews.ApprovalItem, 0, len(items))
	for _, item := range items {
		if itemMatchesQuery(item, q) {
			out = append(out, item)
		}
	}
	return out
}

// itemMatchesQuery reports whether an item matches the lowercased query.
// Inputs: item, lowercased query.
// Output: true when any field contains q.
func itemMatchesQuery(item adminviews.ApprovalItem, q string) bool {
	fields := []string{
		item.Name, item.FirstName, item.LastName, item.NationalID, item.Mobile,
		item.ExternalID, item.SpecialtyName, item.SpecialtyCode, item.PhotoURL,
		strconv.Itoa(item.LocalCode), strconv.Itoa(item.DoctorSystemID),
	}
	for _, f := range fields {
		if strings.Contains(strings.ToLower(f), q) {
			return true
		}
	}
	return false
}

// paginateTotalPages computes total pages for a list length.
// Inputs: total items, page size.
// Output: page count.
func paginateTotalPages(total, perPage int) int {
	if perPage <= 0 {
		perPage = defaultPageSize
	}
	if total == 0 {
		return 0
	}
	return (total + perPage - 1) / perPage
}

// paginateSlice returns one page of items.
// Inputs: items, page number, page size.
// Output: sliced items for that page.
func paginateSlice(items []adminviews.ApprovalItem, page, perPage int) []adminviews.ApprovalItem {
	if perPage <= 0 {
		perPage = defaultPageSize
	}
	if page < 1 {
		page = 1
	}
	start := (page - 1) * perPage
	if start >= len(items) {
		return []adminviews.ApprovalItem{}
	}
	end := start + perPage
	if end > len(items) {
		end = len(items)
	}
	return items[start:end]
}

// render writes the DoctorApproval HTML response.
// Inputs: gin context, user, view model.
// Output: none (writes response).
func (h *ApprovalHandler) render(c *gin.Context, user *models.AppointmentUser, view adminviews.DoctorApprovalView) {
	if view.Nav.NavItems == nil {
		view.Nav = adminviews.BuildAdminNav(adminviews.NavApprovals, user.Role)
	}
	c.Header("Content-Type", "text/html; charset=utf-8")
	_ = adminviews.DoctorApproval(view).Render(c.Request.Context(), c.Writer)
}

// notifyClinic sends doctor.approval.notify to the clinic WebSocket client.
// Inputs: approval request, local HIS code.
// Output: none.
func (h *ApprovalHandler) notifyClinic(req *models.DoctorApprovalRequest, localCode int) {
	if h.Hub == nil || req == nil || req.ExternalID == "" {
		return
	}
	doctorID := uint(0)
	if req.DoctorID != nil {
		doctorID = *req.DoctorID
		if localCode == 0 {
			if doc, err := h.Doctors.GetByID(doctorID); err == nil && doc != nil {
				localCode = doc.LocalCode
			}
		}
	}
	_ = h.Hub.SendToClinic(req.ClinicID, protocol.TypeDoctorApprovalNotify, protocol.DoctorApprovalNotify{
		DoctorID:   doctorID,
		LocalCode:  localCode,
		ExternalID: req.ExternalID,
		IsApproved: req.Status == string(constants.ApprovalApproved),
		Status:     req.Status,
	})
}

// liveListErrorMessage maps hub errors to Persian admin messages.
// Inputs: error from RequestDoctorList.
// Output: user-facing message.
func liveListErrorMessage(err error) string {
	switch err {
	case websocket.ErrClinicOffline:
		return "مرکز انتخاب‌شده آنلاین نیست."
	case websocket.ErrRequestTimeout:
		return "پاسخ لیست پزشکان از مرکز به‌موقع نرسید."
	default:
		return "خطا در دریافت لیست پزشکان از مرکز: " + err.Error()
	}
}
