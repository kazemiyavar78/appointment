package admin

import (
	"net/http"
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
)

// ApprovalHandler manages doctor approval requests.
type ApprovalHandler struct {
	Doctors     *repository.DoctorRepo
	Specialties *repository.SpecialtyRepo
	Clinics     *repository.ClinicRepo
	Scope       *auth.ClinicScope
	Hub         *websocket.Hub
	Cache       *cache.Store
	DefaultExpiration time.Duration
	CleanupInterval time.Duration
}

// NewApprovalHandler constructs an ApprovalHandler.
func NewApprovalHandler(
	doctors *repository.DoctorRepo,
	specialties *repository.SpecialtyRepo,
	clinics *repository.ClinicRepo,
	scope *auth.ClinicScope,
	hub *websocket.Hub,
	cache *cache.Store,
	defaultExpiration time.Duration,
	cleanupInterval time.Duration,
) *ApprovalHandler {
	return &ApprovalHandler{
		Doctors:     doctors,
		Specialties: specialties,
		Clinics:     clinics,
		Scope:       scope,
		Hub:         hub,
		Cache:       cache,
		DefaultExpiration: defaultExpiration,
		CleanupInterval: cleanupInterval,
	}
}

// ListPending fetches live HIS doctors, appends server-only rows, then paginates.
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

	allowed, err := h.Scope.AllowedClinics(user)
	if err != nil {
		c.String(http.StatusInternalServerError, "failed to load clinics")
		return
	}
	if len(allowed) == 0 {
		h.render(c, user, adminviews.DoctorApprovalView{
			Nav:     adminviews.BuildAdminNav(adminviews.NavApprovals, user.Role),
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
	}

	var allItems []adminviews.ApprovalItem

	if fetchLive {
		if _, online := onlineSet[clinicID]; !online {
			view.Message = "مرکز انتخاب‌شده آنلاین نیست. فقط پزشکان ثبت‌شده روی سرور نمایش داده می‌شوند."
		} else {
			liveItems, liveErr := h.fetchLiveItems(clinicID)
			if liveErr != nil {
				view.Message = liveListErrorMessage(liveErr)
			} else {
				allItems = append(allItems, liveItems...)
			}
		}
	}

	serverItems, err := h.fetchServerOnlyItems(clinicID, allItems)
	if err != nil {
		c.String(http.StatusInternalServerError, "failed to load server doctors")
		return
	}
	allItems = append(allItems, serverItems...)

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
		view.Message = "پزشکی برای نمایش یافت نشد. «دریافت لیست زنده» را بزنید."
	}
	h.render(c, user, view)
}

// Decide approves or rejects a pending doctor request.
func (h *ApprovalHandler) Decide(c *gin.Context) {
	user, ok := auth.UserFromGin(c)
	if !ok {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.String(http.StatusBadRequest, "invalid id")
		return
	}
	decision := c.PostForm("decision")
	note := c.PostForm("note")
	approve := decision == "approve"
	clinicIDStr := c.PostForm("clinic_id")

	var specialtyID uint
	if approve {
		sid, err := strconv.ParseUint(c.PostForm("specialty_id"), 10, 64)
		if err != nil || sid == 0 {
			c.String(http.StatusBadRequest, "انتخاب تخصص الزامی است")
			return
		}
		specialtyID = uint(sid)
	}

	req, err := h.Doctors.DecideApproval(uint(id), user.ID, approve, specialtyID, note)
	if err != nil {
		c.String(http.StatusInternalServerError, "failed to update approval")
		return
	}
	if req != nil && !h.Scope.CanAccess(user, req.ClinicID) {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	if approve {
		h.notifyClinic(req)
	}

	redirect := buildRedirect(clinicIDStr, req, c)
	c.Redirect(http.StatusFound, redirect)
}

func buildRedirect(clinicIDStr string, req *models.DoctorApprovalRequest, c *gin.Context) string {
	redirect := "/admin/approvals?fetch=1"
	if clinicIDStr != "" {
		redirect += "&clinic_id=" + clinicIDStr
	} else if req != nil {
		redirect += "&clinic_id=" + strconv.FormatUint(uint64(req.ClinicID), 10)
	}
	if q := c.PostForm("q"); q != "" {
		redirect += "&q=" + q
	}
	if p := c.PostForm("page"); p != "" {
		redirect += "&page=" + p
	}
	return redirect
}

func (h *ApprovalHandler) resolveClinicID(c *gin.Context, allowed []models.Clinic) uint {
	if raw := c.Query("clinic_id"); raw != "" {
		if id, err := strconv.ParseUint(raw, 10, 64); err == nil {
			return uint(id)
		}
	}
	return 0
}

func (h *ApprovalHandler) clinicOptions(allowed []models.Clinic, online map[uint]struct{}, selected uint) []adminviews.ClinicOption {
	opts := make([]adminviews.ClinicOption, 0, len(allowed))
	for _, c := range allowed {
		_, isOnline := online[c.ID]
		opts = append(opts, adminviews.ClinicOption{
			ID:       c.ID,
			Name:     c.Name,
			IsOnline: isOnline,
			Selected: c.ID == selected,
		})
	}
	return opts
}

func toSpecialtyOptions(specs []models.Specialty) []adminviews.SpecialtyOption {
	opts := make([]adminviews.SpecialtyOption, 0, len(specs))
	for _, s := range specs {
		opts = append(opts, adminviews.SpecialtyOption{ID: s.ID, Name: s.Name})
	}
	return opts
}

func (h *ApprovalHandler) fetchLiveItems(clinicID uint) ([]adminviews.ApprovalItem, error) {
	push, err := h.Hub.RequestDoctorList(clinicID, liveDoctorListTimeout)
	if err != nil {
		return nil, err
	}
	
	items := make([]adminviews.ApprovalItem, 0, len(push.Doctors))
	for _, dto := range push.Doctors {
		item, err := h.itemFromLiveDTO(clinicID, dto)
		if err != nil {
			return items, err
		}
		items = append(items, item)
	}
	return items, nil
}

func (h *ApprovalHandler) itemFromLiveDTO(clinicID uint, dto protocol.DoctorDTO) (adminviews.ApprovalItem, error) {
	item := itemFromDTO(clinicID, dto, "clinic")
	doctor, found, err := h.Doctors.FindExistingPublic(clinicID, dto)
	if err != nil {
		return item, err
	}
	if !found {
		return item, nil
	}
	return h.enrichItem(item, doctor)
}

func (h *ApprovalHandler) fetchServerOnlyItems(clinicID uint, live []adminviews.ApprovalItem) ([]adminviews.ApprovalItem, error) {
	rows, err := h.Doctors.ListByClinic(clinicID)
	if err != nil {
		return nil, err
	}
	seen := newDoctorKeySet(live)
	items := make([]adminviews.ApprovalItem, 0)
	for _, doc := range rows {
		if seen.hasDoctor(doc.ID, doc.LocalCode, doc.ExternalID, doc.NationalID) {
			continue
		}
		item := itemFromDoctor(doc, "server")
		item, err = h.enrichItem(item, doc)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func (h *ApprovalHandler) enrichItem(item adminviews.ApprovalItem, doctor models.Doctor) (adminviews.ApprovalItem, error) {
	item.DoctorID = doctor.ID
	item.ExternalID = doctor.ExternalID
	item.LocalCode = doctor.LocalCode
	item.FirstName = doctor.FirstName
	item.LastName = doctor.LastName
	item.Mobile = doctor.Mobile
	item.NationalID = doctor.NationalID
	item.DoctorSystemID = doctor.DoctorSystemID
	item.SpecialtyCode = doctor.SpecialtyCode
	item.PhotoURL = doctor.PhotoURL
	item.IsActive = doctor.IsActive
	if doctor.Name != "" {
		item.Name = doctor.Name
	}
	if doctor.Specialty.ID != 0 {
		item.SpecialtyName = doctor.Specialty.Name
	}
	if doctor.IsApproved {
		item.Status = string(constants.ApprovalApproved)
		item.CanDecide = false
		return item, nil
	}
	req, err := h.Doctors.FindPendingApprovalByDoctorID(doctor.ID)
	if err != nil {
		return item, err
	}
	if req != nil {
		item.ID = req.ID
		item.Status = req.Status
		item.CanDecide = true
	} else {
		item.Status = "registered"
	}
	return item, nil
}

func itemFromDTO(clinicID uint, dto protocol.DoctorDTO, source string) adminviews.ApprovalItem {
	name := dto.Name
	if name == "" {
		name = strings.TrimSpace(dto.FirstName + " " + dto.LastName)
	}
	return adminviews.ApprovalItem{
		ClinicID:       clinicID,
		LocalCode:      dto.LocalCode,
		FirstName:      dto.FirstName,
		LastName:       dto.LastName,
		Name:           name,
		NationalID:     dto.NationalID,
		Mobile:         dto.Mobile,
		DoctorSystemID: dto.DoctorSystemID,
		SpecialtyCode:  dto.SpecialtyCode,
		PhotoURL:       dto.PhotoURL,
		ExternalID:     dto.ExternalID,
		IsActive:       dto.IsActive,
		Source:         source,
		Status:         string(constants.ApprovalPending),
	}
}

func itemFromDoctor(doc models.Doctor, source string) adminviews.ApprovalItem {
	spec := ""
	if doc.Specialty.ID != 0 {
		spec = doc.Specialty.Name
	}
	return adminviews.ApprovalItem{
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
		Source:         source,
	}
}

type doctorKeySet struct {
	doctor map[uint]struct{}
	local  map[int]struct{}
	ext    map[string]struct{}
	nid    map[string]struct{}
}

func newDoctorKeySet(live []adminviews.ApprovalItem) doctorKeySet {
	k := doctorKeySet{
		doctor: make(map[uint]struct{}),
		local:  make(map[int]struct{}),
		ext:    make(map[string]struct{}),
		nid:    make(map[string]struct{}),
	}
	for _, item := range live {
		if item.DoctorID > 0 {
			k.doctor[item.DoctorID] = struct{}{}
		}
		if item.LocalCode > 0 {
			k.local[item.LocalCode] = struct{}{}
		}
		if item.ExternalID != "" {
			k.ext[item.ExternalID] = struct{}{}
		}
		if item.NationalID != "" {
			k.nid[item.NationalID] = struct{}{}
		}
	}
	return k
}

func (k doctorKeySet) hasDoctor(id uint, local int, ext, nid string) bool {
	if _, ok := k.doctor[id]; ok && id > 0 {
		return true
	}
	if _, ok := k.local[local]; ok && local > 0 {
		return true
	}
	if _, ok := k.ext[ext]; ok && ext != "" {
		return true
	}
	if _, ok := k.nid[nid]; ok && nid != "" {
		return true
	}
	return false
}

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

func paginateTotalPages(total, perPage int) int {
	if perPage <= 0 {
		perPage = defaultPageSize
	}
	if total == 0 {
		return 0
	}
	return (total + perPage - 1) / perPage
}

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

func (h *ApprovalHandler) render(c *gin.Context, user *models.AppointmentUser, view adminviews.DoctorApprovalView) {
	if view.Nav.NavItems == nil {
		view.Nav = adminviews.BuildAdminNav(adminviews.NavApprovals, user.Role)
	}
	c.Header("Content-Type", "text/html; charset=utf-8")
	_ = adminviews.DoctorApproval(view).Render(c.Request.Context(), c.Writer)
}

func (h *ApprovalHandler) notifyClinic(req *models.DoctorApprovalRequest) {
	if h.Hub == nil || req == nil || req.ExternalID == "" {
		return
	}
	localCode := 0
	doctorID := uint(0)
	if req.DoctorID != nil {
		doctorID = *req.DoctorID
		if doc, err := h.Doctors.GetByID(doctorID); err == nil {
			localCode = doc.LocalCode
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
