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
		Nav:         adminviews.BuildAdminNav(adminviews.NavApprovals, user.Role),
		ClinicID:    clinicID,
		Clinics:     h.clinicOptions(allowed, onlineSet, clinicID),
		Specialties: toSpecialtyOptions(specs),
		SearchQuery: search,
		Page:        page,
		PerPage:     perPage,
		FetchLive:   fetchLive,
		Tab:         tab,
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
	photoURL, err := h.resolvePhotoURL(c, pending.PhotoURL)
	if err != nil {
		c.String(http.StatusBadRequest, err.Error())
		return
	}
	if strings.TrimSpace(photoURL) == "" {
		c.String(http.StatusBadRequest, "آپلود یا وارد کردن عکس پزشک قبل از تأیید الزامی است")
		return
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
// Inputs: gin context form (doctor_id, clinic_id, specialty_id, photo).
// Output: redirect to approved tab.
func (h *ApprovalHandler) UpdateApproved(c *gin.Context) {
	user, ok := auth.UserFromGin(c)
	if !ok {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	doctorID, err := strconv.ParseUint(c.PostForm("doctor_id"), 10, 64)
	if err != nil || doctorID == 0 {
		c.String(http.StatusBadRequest, "invalid doctor_id")
		return
	}
	clinicIDStr := c.PostForm("clinic_id")
	clinicID, _ := strconv.ParseUint(clinicIDStr, 10, 64)

	doc, err := h.Doctors.GetByID(uint(doctorID))
	if err != nil || doc == nil {
		c.String(http.StatusNotFound, "پزشک یافت نشد")
		return
	}
	if !h.Scope.CanAccess(user, doc.ClinicID) {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	var specialtyID uint
	if sid, err := strconv.ParseUint(c.PostForm("specialty_id"), 10, 64); err == nil {
		specialtyID = uint(sid)
	}
	photoURL, err := h.resolvePhotoURL(c, "")
	if err != nil {
		c.String(http.StatusBadRequest, err.Error())
		return
	}
	if _, err := h.Doctors.UpdateApprovedProfile(uint(doctorID), specialtyID, photoURL); err != nil {
		c.String(http.StatusInternalServerError, "بروزرسانی ناموفق بود")
		return
	}
	if clinicID == 0 {
		clinicID = uint64(doc.ClinicID)
		clinicIDStr = strconv.FormatUint(clinicID, 10)
	}
	c.Redirect(http.StatusFound, buildRedirect(clinicIDStr, tabApproved, c))
}

// resolvePhotoURL returns uploaded file URL, form photo_url, or fallback.
// Inputs: gin context, fallback URL.
// Output: public photo URL or error.
func (h *ApprovalHandler) resolvePhotoURL(c *gin.Context, fallback string) (string, error) {
	file, hdr, err := c.Request.FormFile("photo")
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
		name := fmt.Sprintf("%d%s", time.Now().UnixNano(), ext)
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
	if url := strings.TrimSpace(c.PostForm("photo_url")); url != "" {
		return url, nil
	}
	return strings.TrimSpace(fallback), nil
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
			SpecialtyName:  spec,
			PhotoURL:       doc.PhotoURL,
			ExternalID:     doc.ExternalID,
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
