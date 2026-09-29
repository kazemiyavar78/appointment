package admin

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"tebpardaz/server/internal/auth"
	"tebpardaz/server/internal/models"
	"tebpardaz/server/internal/repository"
	"tebpardaz/server/internal/seo"
	adminviews "tebpardaz/server/views/admin"
	"tebpardaz/shared/constants"

	"github.com/gin-gonic/gin"
)

const (
	sectionUploadDir = "static/uploads/sections"
	maxMessageLen    = 800
	maxServicesCount = 5
)

// SectionAdminHandler handles administration of clinic sections and their child components.
type SectionAdminHandler struct {
	Sections *repository.SectionRepo
	Clinics  *repository.ClinicRepo
	Scope    *auth.ClinicScope
}

// NewSectionAdminHandler initializes a new SectionAdminHandler.
// Input: section repo, clinic repo, clinic scope.
// Output: pointer to SectionAdminHandler.
func NewSectionAdminHandler(sections *repository.SectionRepo, clinics *repository.ClinicRepo, scope *auth.ClinicScope) *SectionAdminHandler {
	return &SectionAdminHandler{
		Sections: sections,
		Clinics:  clinics,
		Scope:    scope,
	}
}

// List handles listing sections for a clinic and rendering the create/edit view.
// Input: gin.Context containing user auth session and query parameters.
// Output: HTML page rendering section management interface.
func (h *SectionAdminHandler) List(c *gin.Context) {
	user, allowed, ok := h.requireUserClinics(c)
	if !ok {
		return
	}

	clinicID := h.resolveClinicID(c, user, allowed)
	if clinicID == 0 {
		c.String(http.StatusBadRequest, "مرکز معتبری یافت نشد.")
		return
	}

	sections, err := h.Sections.ListSectionsByClinic(clinicID)
	if err != nil {
		c.String(http.StatusInternalServerError, "خطا در دریافت بخش‌ها")
		return
	}

	sectionSummaries := toSectionSummaries(sections)
	nav := adminviews.BuildAdminNavWithSections(adminviews.NavSections, user.Role, sectionSummaries)

	msg := sectionFlashMessage(c.Query("msg"))

	quota, _ := h.Sections.GetClinicSectionQuota(clinicID)
	currentCount, _ := h.Sections.CountSectionsByClinic(clinicID)

	view := adminviews.SectionsPageView{
		Nav:              nav,
		ClinicID:         clinicID,
		Clinics:          toSectionClinicOptions(allowed),
		ShowClinicPicker: len(allowed) > 1,
		Sections:         h.toSectionRows(sections, clinicID),
		Message:          msg,
		IsSuperAdmin:     constants.UserRole(user.Role) == constants.UserRoleSuperAdmin,
		CurrentCount:     int(currentCount),
		MaxSections:      quota,
	}

	editID, _ := strconv.ParseUint(c.Query("edit"), 10, 64)
	if editID > 0 {
		if sec, err := h.Sections.GetSectionByID(uint(editID)); err == nil && sec.ClinicID == clinicID {
			view.EditID = sec.ID
			view.EditTitle = sec.Title
			view.EditSlug = sec.Slug
			view.EditSortOrder = sec.SortOrder
			view.EditIsActive = sec.IsActive
		}
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
	_ = adminviews.Sections(view).Render(c.Request.Context(), c.Writer)
}

// Create handles creating a new section for a clinic.
// Input: gin.Context with form data (clinic_id, title, slug, sort_order, is_active).
// Output: redirects to section list or re-renders with error.
func (h *SectionAdminHandler) Create(c *gin.Context) {
	user, allowed, ok := h.requireUserClinics(c)
	if !ok {
		return
	}

	clinicID := h.resolveClinicID(c, user, allowed)
	if clinicID == 0 || !h.Scope.CanAccess(user, clinicID) {
		c.String(http.StatusForbidden, "دسترسی به این مرکز مجاز نیست.")
		return
	}

	title := strings.TrimSpace(c.PostForm("title"))
	slug := sanitizeSlug(c.PostForm("slug"))
	sortOrder, _ := strconv.Atoi(c.PostForm("sort_order"))
	isActive := c.PostForm("is_active") == "1"

	if title == "" || slug == "" {
		c.Redirect(http.StatusFound, fmt.Sprintf("/admin/sections?clinic_id=%d&msg=empty_fields", clinicID))
		return
	}

	// کنترل سقف ایجاد بخش برای کاربران غیر سوپر ادمین
	if constants.UserRole(user.Role) != constants.UserRoleSuperAdmin {
		quota, _ := h.Sections.GetClinicSectionQuota(clinicID)
		count, _ := h.Sections.CountSectionsByClinic(clinicID)
		if int(count) >= quota {
			c.Redirect(http.StatusFound, fmt.Sprintf("/admin/sections?clinic_id=%d&msg=quota_exceeded", clinicID))
			return
		}
	}

	// Check if slug is already taken for this clinic
	if existing, _ := h.Sections.GetSectionByClinicAndSlug(clinicID, slug); existing != nil {
		c.Redirect(http.StatusFound, fmt.Sprintf("/admin/sections?clinic_id=%d&msg=duplicate_slug", clinicID))
		return
	}

	section := &models.AppointmentClinicSection{
		ClinicID:  clinicID,
		Title:     title,
		Slug:      slug,
		SortOrder: sortOrder,
		IsActive:  isActive,
	}

	if err := h.Sections.CreateSection(section); err != nil {
		c.Redirect(http.StatusFound, fmt.Sprintf("/admin/sections?clinic_id=%d&msg=error", clinicID))
		return
	}

	c.Redirect(http.StatusFound, fmt.Sprintf("/admin/sections?clinic_id=%d&msg=created", clinicID))
}

// Update handles updating basic section information.
// Input: gin.Context with URL param id and form data.
// Output: redirects to section list or error.
func (h *SectionAdminHandler) Update(c *gin.Context) {
	user, _, ok := h.requireUserClinics(c)
	if !ok {
		return
	}

	id, err := parseUintParam(c, "id")
	if err != nil {
		c.String(http.StatusBadRequest, "شناسه نامعتبر است.")
		return
	}

	sec, err := h.Sections.GetSectionByID(id)
	if err != nil || !h.Scope.CanAccess(user, sec.ClinicID) {
		c.String(http.StatusForbidden, "بخش مورد نظر یافت نشد یا دسترسی غیرمجاز است.")
		return
	}

	title := strings.TrimSpace(c.PostForm("title"))
	slug := sanitizeSlug(c.PostForm("slug"))
	sortOrder, _ := strconv.Atoi(c.PostForm("sort_order"))
	isActive := c.PostForm("is_active") == "1"

	if title == "" || slug == "" {
		c.Redirect(http.StatusFound, fmt.Sprintf("/admin/sections?clinic_id=%d&edit=%d&msg=empty_fields", sec.ClinicID, id))
		return
	}

	// Check duplicate slug with another section
	if existing, _ := h.Sections.GetSectionByClinicAndSlug(sec.ClinicID, slug); existing != nil && existing.ID != sec.ID {
		c.Redirect(http.StatusFound, fmt.Sprintf("/admin/sections?clinic_id=%d&edit=%d&msg=duplicate_slug", sec.ClinicID, id))
		return
	}

	sec.Title = title
	sec.Slug = slug
	sec.SortOrder = sortOrder
	sec.IsActive = isActive

	if err := h.Sections.UpdateSection(sec); err != nil {
		c.Redirect(http.StatusFound, fmt.Sprintf("/admin/sections?clinic_id=%d&msg=error", sec.ClinicID))
		return
	}

	c.Redirect(http.StatusFound, fmt.Sprintf("/admin/sections?clinic_id=%d&msg=updated", sec.ClinicID))
}

// Delete handles removing a section and all its subcomponents.
// Input: gin.Context with URL param id.
// Output: redirects to section list.
func (h *SectionAdminHandler) Delete(c *gin.Context) {
	user, _, ok := h.requireUserClinics(c)
	if !ok {
		return
	}

	id, err := parseUintParam(c, "id")
	if err != nil {
		c.String(http.StatusBadRequest, "شناسه نامعتبر است.")
		return
	}

	sec, err := h.Sections.GetSectionByID(id)
	if err != nil || !h.Scope.CanAccess(user, sec.ClinicID) {
		c.String(http.StatusForbidden, "دسترسی غیرمجاز است.")
		return
	}

	clinicID := sec.ClinicID
	_ = h.Sections.DeleteSection(id, clinicID)

	c.Redirect(http.StatusFound, fmt.Sprintf("/admin/sections?clinic_id=%d&msg=deleted", clinicID))
}

// BannerEdit renders the banner editor for a section.
// Input: gin.Context with section ID.
// Output: HTML page for banner editing.
func (h *SectionAdminHandler) BannerEdit(c *gin.Context) {
	user, sec, ok := h.requireSectionAccess(c)
	if !ok {
		return
	}

	banner, err := h.Sections.GetBannerBySectionID(sec.ID)
	if err != nil {
		c.String(http.StatusInternalServerError, "خطا در دریافت بنر")
		return
	}

	sections, _ := h.Sections.ListSectionsByClinic(sec.ClinicID)
	nav := adminviews.BuildAdminNavWithSections(fmt.Sprintf("section_%d_banner", sec.ID), user.Role, toSectionSummaries(sections))

	overlayLeft := models.ClampPercent(banner.OverlayOpacityLeft, 85)
	overlayBottom := models.ClampPercent(banner.OverlayOpacityBottom, 60)
	if overlayLeft == 0 && overlayBottom == 0 {
		overlayLeft = 85
		overlayBottom = 60
	}

	view := adminviews.SectionBannerView{
		Nav:                   nav,
		SectionID:             sec.ID,
		SectionTitle:          sec.Title,
		ClinicID:              sec.ClinicID,
		Slogan:                banner.Slogan,
		Description:           banner.Description,
		Services:              banner.Services,
		BackgroundColor:       models.SanitizeHexColor(banner.BackgroundColor, "#0a2e2e"),
		BackgroundColorEnd:    models.SanitizeHexColor(banner.BackgroundColorEnd, "#134e4a"),
		UseBackgroundGradient: banner.UseBackgroundGradient,
		BackgroundGradientDir: models.SanitizeGradientDir(banner.BackgroundGradientDir),
		ImageURL:              banner.ImageURL,
		OverlayColor:          models.SanitizeHexColor(banner.OverlayColor, banner.BackgroundColor),
		UseOverlayGradient:    banner.OverlayEnabled(),
		OverlayOpacityLeft:    overlayLeft,
		OverlayOpacityBottom:  overlayBottom,
		Message:               sectionFlashMessage(c.Query("msg")),
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
	_ = adminviews.SectionBannerEdit(view).Render(c.Request.Context(), c.Writer)
}

// BannerSave updates the banner fields and optional banner image.
// Input: gin.Context with form data (slogan, description, services, background/overlay gradient fields, image).
// Output: redirects back to banner editor with success or error message.
func (h *SectionAdminHandler) BannerSave(c *gin.Context) {
	_, sec, ok := h.requireSectionAccess(c)
	if !ok {
		return
	}

	slogan := strings.TrimSpace(c.PostForm("slogan"))
	description := strings.TrimSpace(c.PostForm("description"))
	rawServices := strings.TrimSpace(c.PostForm("services"))
	bgColor := models.SanitizeHexColor(c.PostForm("background_color"), "#0a2e2e")
	bgColorEnd := models.SanitizeHexColor(c.PostForm("background_color_end"), "#134e4a")
	useBgGradient := c.PostForm("use_background_gradient") == "1"
	bgGradientDir := models.SanitizeGradientDir(c.PostForm("background_gradient_dir"))
	overlayColor := models.SanitizeHexColor(c.PostForm("overlay_color"), bgColor)
	useOverlay := c.PostForm("use_overlay_gradient") == "1"
	overlayLeft := parsePercentForm(c.PostForm("overlay_opacity_left"), 85)
	overlayBottom := parsePercentForm(c.PostForm("overlay_opacity_bottom"), 60)

	// Validate services count <= 5
	servicesList := parseServicesLines(rawServices, maxServicesCount)
	validatedServices := strings.Join(servicesList, "\n")

	banner, _ := h.Sections.GetBannerBySectionID(sec.ID)
	existingImage := ""
	if banner != nil {
		existingImage = banner.ImageURL
	}

	imageURL, err := h.saveUploadedImage(c, "image", existingImage)
	if err != nil {
		c.Redirect(http.StatusFound, fmt.Sprintf("/admin/sections/%d/banner?msg=invalid_image", sec.ID))
		return
	}

	bannerToSave := &models.SectionBanner{
		SectionID:             sec.ID,
		Slogan:                slogan,
		Description:           description,
		Services:              validatedServices,
		BackgroundColor:       bgColor,
		ImageURL:              imageURL,
		BackgroundColorEnd:    bgColorEnd,
		UseBackgroundGradient: useBgGradient,
		BackgroundGradientDir: bgGradientDir,
		OverlayColor:          overlayColor,
		UseOverlayGradient:    useOverlay,
		OverlayOpacityLeft:    overlayLeft,
		OverlayOpacityBottom:  overlayBottom,
	}

	if err := h.Sections.SaveBanner(bannerToSave); err != nil {
		c.Redirect(http.StatusFound, fmt.Sprintf("/admin/sections/%d/banner?msg=error", sec.ID))
		return
	}

	c.Redirect(http.StatusFound, fmt.Sprintf("/admin/sections/%d/banner?msg=saved", sec.ID))
}

// ScheduleEdit renders the 7-day schedule configuration form.
// Input: gin.Context with section ID.
// Output: HTML page for schedule editing.
func (h *SectionAdminHandler) ScheduleEdit(c *gin.Context) {
	user, sec, ok := h.requireSectionAccess(c)
	if !ok {
		return
	}

	schedules, err := h.Sections.GetScheduleBySectionID(sec.ID)
	if err != nil {
		c.String(http.StatusInternalServerError, "خطا در دریافت ساعات کاری")
		return
	}

	sections, _ := h.Sections.ListSectionsByClinic(sec.ClinicID)
	nav := adminviews.BuildAdminNavWithSections(fmt.Sprintf("section_%d_schedule", sec.ID), user.Role, toSectionSummaries(sections))

	var days []adminviews.DayScheduleForm
	for _, s := range schedules {
		days = append(days, adminviews.DayScheduleForm{
			DayOfWeek:   s.DayOfWeek,
			DayName:     s.DayName,
			IsOpen:      s.IsOpen,
			Shift1Start: s.Shift1Start,
			Shift1End:   s.Shift1End,
			Shift2Start: s.Shift2Start,
			Shift2End:   s.Shift2End,
			HasShift2:   s.HasShift2,
		})
	}

	view := adminviews.SectionScheduleView{
		Nav:          nav,
		SectionID:    sec.ID,
		SectionTitle: sec.Title,
		ClinicID:     sec.ClinicID,
		Days:         days,
		Message:      sectionFlashMessage(c.Query("msg")),
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
	_ = adminviews.SectionScheduleEdit(view).Render(c.Request.Context(), c.Writer)
}

// ScheduleSave updates the 7 days of shifts for the section.
// Input: gin.Context with form data for days 0..6.
// Output: redirects to schedule edit page.
func (h *SectionAdminHandler) ScheduleSave(c *gin.Context) {
	_, sec, ok := h.requireSectionAccess(c)
	if !ok {
		return
	}

	var schedules []models.SectionSchedule
	for dayIdx := 0; dayIdx < 7; dayIdx++ {
		idxStr := strconv.Itoa(dayIdx)
		isOpen := c.PostForm("is_open_"+idxStr) == "1"
		shift1Start := strings.TrimSpace(c.PostForm("shift1_start_" + idxStr))
		shift1End := strings.TrimSpace(c.PostForm("shift1_end_" + idxStr))
		hasShift2 := c.PostForm("has_shift2_"+idxStr) == "1"
		shift2Start := strings.TrimSpace(c.PostForm("shift2_start_" + idxStr))
		shift2End := strings.TrimSpace(c.PostForm("shift2_end_" + idxStr))

		if shift1Start == "" {
			shift1Start = "08:00"
		}
		if shift1End == "" {
			shift1End = "14:00"
		}
		if shift2Start == "" {
			shift2Start = "16:00"
		}
		if shift2End == "" {
			shift2End = "20:00"
		}

		schedules = append(schedules, models.SectionSchedule{
			SectionID:   sec.ID,
			DayOfWeek:   dayIdx,
			IsOpen:      isOpen,
			Shift1Start: shift1Start,
			Shift1End:   shift1End,
			Shift2Start: shift2Start,
			Shift2End:   shift2End,
			HasShift2:   hasShift2,
		})
	}

	if err := h.Sections.SaveSchedule(sec.ID, schedules); err != nil {
		c.Redirect(http.StatusFound, fmt.Sprintf("/admin/sections/%d/schedule?msg=error", sec.ID))
		return
	}

	c.Redirect(http.StatusFound, fmt.Sprintf("/admin/sections/%d/schedule?msg=saved", sec.ID))
}

// MessagesList renders the list of patient messages and the message creation/edit form.
// Input: gin.Context with section ID and optional edit query param.
// Output: HTML page for message management.
func (h *SectionAdminHandler) MessagesList(c *gin.Context) {
	user, sec, ok := h.requireSectionAccess(c)
	if !ok {
		return
	}

	messages, err := h.Sections.ListMessagesBySectionID(sec.ID)
	if err != nil {
		c.String(http.StatusInternalServerError, "خطا در دریافت پیام‌ها")
		return
	}

	sections, _ := h.Sections.ListSectionsByClinic(sec.ClinicID)
	nav := adminviews.BuildAdminNavWithSections(fmt.Sprintf("section_%d_messages", sec.ID), user.Role, toSectionSummaries(sections))

	var rows []adminviews.MessageRow
	for _, m := range messages {
		rows = append(rows, adminviews.MessageRow{
			ID:          m.ID,
			Title:       m.Title,
			Content:     m.Content,
			SenderTitle: m.SenderTitle,
			SortOrder:   m.SortOrder,
			IsActive:    m.IsActive,
		})
	}

	view := adminviews.SectionMessagesView{
		Nav:          nav,
		SectionID:    sec.ID,
		SectionTitle: sec.Title,
		ClinicID:     sec.ClinicID,
		Messages:     rows,
		Message:      sectionFlashMessage(c.Query("msg")),
	}

	editID, _ := strconv.ParseUint(c.Query("edit"), 10, 64)
	if editID > 0 {
		if msg, err := h.Sections.GetMessageByID(uint(editID)); err == nil && msg.SectionID == sec.ID {
			view.EditID = msg.ID
			view.EditTitle = msg.Title
			view.EditContent = msg.Content
			view.EditSenderTitle = msg.SenderTitle
			view.EditSortOrder = msg.SortOrder
			view.EditIsActive = msg.IsActive
		}
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
	_ = adminviews.SectionMessages(view).Render(c.Request.Context(), c.Writer)
}

// MessageCreate creates a new patient message with max 800 chars constraint.
// Input: gin.Context with form data (title, content, sender_title, sort_order, is_active).
// Output: redirects to messages list.
func (h *SectionAdminHandler) MessageCreate(c *gin.Context) {
	_, sec, ok := h.requireSectionAccess(c)
	if !ok {
		return
	}

	content := strings.TrimSpace(c.PostForm("content"))
	if content == "" {
		c.Redirect(http.StatusFound, fmt.Sprintf("/admin/sections/%d/messages?msg=empty_content", sec.ID))
		return
	}
	if len([]rune(content)) > maxMessageLen {
		c.Redirect(http.StatusFound, fmt.Sprintf("/admin/sections/%d/messages?msg=msg_too_long", sec.ID))
		return
	}

	title := strings.TrimSpace(c.PostForm("title"))
	senderTitle := strings.TrimSpace(c.PostForm("sender_title"))
	sortOrder, _ := strconv.Atoi(c.PostForm("sort_order"))
	isActive := c.PostForm("is_active") == "1"

	msg := &models.SectionMessage{
		SectionID:   sec.ID,
		Title:       title,
		Content:     content,
		SenderTitle: senderTitle,
		SortOrder:   sortOrder,
		IsActive:    isActive,
	}

	if err := h.Sections.CreateMessage(msg); err != nil {
		c.Redirect(http.StatusFound, fmt.Sprintf("/admin/sections/%d/messages?msg=error", sec.ID))
		return
	}

	c.Redirect(http.StatusFound, fmt.Sprintf("/admin/sections/%d/messages?msg=created", sec.ID))
}

// MessageUpdate updates an existing patient message.
// Input: gin.Context with URL param msg_id and form data.
// Output: redirects to messages list.
func (h *SectionAdminHandler) MessageUpdate(c *gin.Context) {
	_, sec, ok := h.requireSectionAccess(c)
	if !ok {
		return
	}

	msgID, err := parseUintParam(c, "msg_id")
	if err != nil {
		c.String(http.StatusBadRequest, "شناسه نامعتبر است.")
		return
	}

	existing, err := h.Sections.GetMessageByID(msgID)
	if err != nil || existing.SectionID != sec.ID {
		c.String(http.StatusNotFound, "پیام یافت نشد.")
		return
	}

	content := strings.TrimSpace(c.PostForm("content"))
	if content == "" {
		c.Redirect(http.StatusFound, fmt.Sprintf("/admin/sections/%d/messages?edit=%d&msg=empty_content", sec.ID, msgID))
		return
	}
	if len([]rune(content)) > maxMessageLen {
		c.Redirect(http.StatusFound, fmt.Sprintf("/admin/sections/%d/messages?edit=%d&msg=msg_too_long", sec.ID, msgID))
		return
	}

	existing.Title = strings.TrimSpace(c.PostForm("title"))
	existing.Content = content
	existing.SenderTitle = strings.TrimSpace(c.PostForm("sender_title"))
	existing.SortOrder, _ = strconv.Atoi(c.PostForm("sort_order"))
	existing.IsActive = c.PostForm("is_active") == "1"

	if err := h.Sections.UpdateMessage(existing); err != nil {
		c.Redirect(http.StatusFound, fmt.Sprintf("/admin/sections/%d/messages?msg=error", sec.ID))
		return
	}

	c.Redirect(http.StatusFound, fmt.Sprintf("/admin/sections/%d/messages?msg=updated", sec.ID))
}

// MessageDelete deletes a patient message.
// Input: gin.Context with URL param msg_id.
// Output: redirects to messages list.
func (h *SectionAdminHandler) MessageDelete(c *gin.Context) {
	_, sec, ok := h.requireSectionAccess(c)
	if !ok {
		return
	}

	msgID, err := parseUintParam(c, "msg_id")
	if err != nil {
		c.String(http.StatusBadRequest, "شناسه نامعتبر است.")
		return
	}

	_ = h.Sections.DeleteMessage(msgID, sec.ID)
	c.Redirect(http.StatusFound, fmt.Sprintf("/admin/sections/%d/messages?msg=deleted", sec.ID))
}

// EquipmentList renders the equipment introduction items and create/edit form.
// Input: gin.Context with section ID and optional edit query param.
// Output: HTML page for equipment management.
func (h *SectionAdminHandler) EquipmentList(c *gin.Context) {
	user, sec, ok := h.requireSectionAccess(c)
	if !ok {
		return
	}

	items, err := h.Sections.ListEquipmentBySectionID(sec.ID)
	if err != nil {
		c.String(http.StatusInternalServerError, "خطا در دریافت لیست تجهیزات")
		return
	}

	sections, _ := h.Sections.ListSectionsByClinic(sec.ClinicID)
	nav := adminviews.BuildAdminNavWithSections(fmt.Sprintf("section_%d_equipment", sec.ID), user.Role, toSectionSummaries(sections))

	var rows []adminviews.EquipmentRow
	for _, item := range items {
		rows = append(rows, adminviews.EquipmentRow{
			ID:         item.ID,
			Title:      item.Title,
			QuoteTitle: item.QuoteTitle,
			Badge:      item.Badge,
			ImageURL:   item.ImageURL,
			Tags:       item.Tags,
			SortOrder:  item.SortOrder,
			IsActive:   item.IsActive,
		})
	}

	view := adminviews.SectionEquipmentView{
		Nav:          nav,
		SectionID:    sec.ID,
		SectionTitle: sec.Title,
		ClinicID:     sec.ClinicID,
		Items:        rows,
		Message:      sectionFlashMessage(c.Query("msg")),
	}

	editID, _ := strconv.ParseUint(c.Query("edit"), 10, 64)
	if editID > 0 {
		if eq, err := h.Sections.GetEquipmentByID(uint(editID)); err == nil && eq.SectionID == sec.ID {
			view.EditID = eq.ID
			view.EditQuoteTitle = eq.QuoteTitle
			view.EditQuoteText = eq.QuoteText
			view.EditBadge = eq.Badge
			view.EditImageURL = eq.ImageURL
			view.EditTitle = eq.Title
			view.EditSubtitle = eq.Subtitle
			view.EditDescription = eq.Description
			view.EditTags = eq.Tags
			view.EditButtonText = eq.ButtonText
			view.EditButtonURL = eq.ButtonURL
			view.EditSortOrder = eq.SortOrder
			view.EditIsActive = eq.IsActive
		}
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
	_ = adminviews.SectionEquipment(view).Render(c.Request.Context(), c.Writer)
}

// EquipmentCreate creates a new equipment introduction item.
// Input: gin.Context with form data and optional uploaded image.
// Output: redirects to equipment list.
func (h *SectionAdminHandler) EquipmentCreate(c *gin.Context) {
	_, sec, ok := h.requireSectionAccess(c)
	if !ok {
		return
	}

	title := strings.TrimSpace(c.PostForm("title"))
	description := strings.TrimSpace(c.PostForm("description"))
	if title == "" || description == "" {
		c.Redirect(http.StatusFound, fmt.Sprintf("/admin/sections/%d/equipment?msg=empty_fields", sec.ID))
		return
	}

	imageURL, err := h.saveUploadedImage(c, "image", "")
	if err != nil {
		c.Redirect(http.StatusFound, fmt.Sprintf("/admin/sections/%d/equipment?msg=invalid_image", sec.ID))
		return
	}

	quoteTitle := strings.TrimSpace(c.PostForm("quote_title"))
	quoteText := strings.TrimSpace(c.PostForm("quote_text"))
	badge := strings.TrimSpace(c.PostForm("badge"))
	if badge == "" {
		badge = "فناوری روز دنیا"
	}
	subtitle := strings.TrimSpace(c.PostForm("subtitle"))
	tags := strings.TrimSpace(c.PostForm("tags"))
	btnText := strings.TrimSpace(c.PostForm("button_text"))
	btnURL := strings.TrimSpace(c.PostForm("button_url"))
	sortOrder, _ := strconv.Atoi(c.PostForm("sort_order"))
	isActive := c.PostForm("is_active") == "1"

	eq := &models.SectionEquipment{
		SectionID:   sec.ID,
		QuoteTitle:  quoteTitle,
		QuoteText:   quoteText,
		Badge:       badge,
		ImageURL:    imageURL,
		Title:       title,
		Subtitle:    subtitle,
		Description: description,
		Tags:        tags,
		ButtonText:  btnText,
		ButtonURL:   btnURL,
		SortOrder:   sortOrder,
		IsActive:    isActive,
	}

	if err := h.Sections.CreateEquipment(eq); err != nil {
		c.Redirect(http.StatusFound, fmt.Sprintf("/admin/sections/%d/equipment?msg=error", sec.ID))
		return
	}

	c.Redirect(http.StatusFound, fmt.Sprintf("/admin/sections/%d/equipment?msg=created", sec.ID))
}

// EquipmentUpdate updates an existing equipment introduction item.
// Input: gin.Context with URL param eq_id and form data.
// Output: redirects to equipment list.
func (h *SectionAdminHandler) EquipmentUpdate(c *gin.Context) {
	_, sec, ok := h.requireSectionAccess(c)
	if !ok {
		return
	}

	eqID, err := parseUintParam(c, "eq_id")
	if err != nil {
		c.String(http.StatusBadRequest, "شناسه نامعتبر است.")
		return
	}

	existing, err := h.Sections.GetEquipmentByID(eqID)
	if err != nil || existing.SectionID != sec.ID {
		c.String(http.StatusNotFound, "آیتم مورد نظر یافت نشد.")
		return
	}

	title := strings.TrimSpace(c.PostForm("title"))
	description := strings.TrimSpace(c.PostForm("description"))
	if title == "" || description == "" {
		c.Redirect(http.StatusFound, fmt.Sprintf("/admin/sections/%d/equipment?edit=%d&msg=empty_fields", sec.ID, eqID))
		return
	}

	imageURL, err := h.saveUploadedImage(c, "image", existing.ImageURL)
	if err != nil {
		c.Redirect(http.StatusFound, fmt.Sprintf("/admin/sections/%d/equipment?edit=%d&msg=invalid_image", sec.ID, eqID))
		return
	}

	existing.QuoteTitle = strings.TrimSpace(c.PostForm("quote_title"))
	existing.QuoteText = strings.TrimSpace(c.PostForm("quote_text"))
	existing.Badge = strings.TrimSpace(c.PostForm("badge"))
	existing.ImageURL = imageURL
	existing.Title = title
	existing.Subtitle = strings.TrimSpace(c.PostForm("subtitle"))
	existing.Description = description
	existing.Tags = strings.TrimSpace(c.PostForm("tags"))
	existing.ButtonText = strings.TrimSpace(c.PostForm("button_text"))
	existing.ButtonURL = strings.TrimSpace(c.PostForm("button_url"))
	existing.SortOrder, _ = strconv.Atoi(c.PostForm("sort_order"))
	existing.IsActive = c.PostForm("is_active") == "1"

	if err := h.Sections.UpdateEquipment(existing); err != nil {
		c.Redirect(http.StatusFound, fmt.Sprintf("/admin/sections/%d/equipment?msg=error", sec.ID))
		return
	}

	c.Redirect(http.StatusFound, fmt.Sprintf("/admin/sections/%d/equipment?msg=updated", sec.ID))
}

// EquipmentDelete deletes an equipment introduction item.
// Input: gin.Context with URL param eq_id.
// Output: redirects to equipment list.
func (h *SectionAdminHandler) EquipmentDelete(c *gin.Context) {
	_, sec, ok := h.requireSectionAccess(c)
	if !ok {
		return
	}

	eqID, err := parseUintParam(c, "eq_id")
	if err != nil {
		c.String(http.StatusBadRequest, "شناسه نامعتبر است.")
		return
	}

	_ = h.Sections.DeleteEquipment(eqID, sec.ID)
	c.Redirect(http.StatusFound, fmt.Sprintf("/admin/sections/%d/equipment?msg=deleted", sec.ID))
}

// DoctorsList صفحه مدیریت پزشکان منتسب به یک بخش را رندر می‌کند.
// ورودی: gin.Context حاوی شناسه بخش. خروجی: HTML لیست و فرم افزودن پزشک تأییدشده.
func (h *SectionAdminHandler) DoctorsList(c *gin.Context) {
	user, sec, ok := h.requireSectionAccess(c)
	if !ok {
		return
	}

	assigned, err := h.Sections.ListSectionDoctorsBySectionID(sec.ID)
	if err != nil {
		c.String(http.StatusInternalServerError, "خطا در دریافت پزشکان بخش")
		return
	}
	available, err := h.Sections.ListAssignableApprovedDoctors(sec.ID, sec.ClinicID)
	if err != nil {
		c.String(http.StatusInternalServerError, "خطا در دریافت پزشکان قابل انتخاب")
		return
	}

	sections, _ := h.Sections.ListSectionsByClinic(sec.ClinicID)
	nav := adminviews.BuildAdminNavWithSections(fmt.Sprintf("section_%d_doctors", sec.ID), user.Role, toSectionSummaries(sections))

	view := adminviews.SectionDoctorsView{
		Nav:          nav,
		SectionID:    sec.ID,
		SectionTitle: sec.Title,
		ClinicID:     sec.ClinicID,
		Assigned:     toSectionDoctorRows(assigned),
		Available:    toSectionDoctorOptions(available),
		Message:      sectionFlashMessage(c.Query("msg")),
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
	_ = adminviews.SectionDoctors(view).Render(c.Request.Context(), c.Writer)
}

// DoctorAssign پزشک تأییدشدهٔ همان مرکز را به بخش اضافه می‌کند.
// ورودی: gin.Context با شناسه بخش و doctor_id فرم. خروجی: تغییر مسیر به لیست پزشکان بخش.
func (h *SectionAdminHandler) DoctorAssign(c *gin.Context) {
	_, sec, ok := h.requireSectionAccess(c)
	if !ok {
		return
	}

	doctorID, err := strconv.ParseUint(strings.TrimSpace(c.PostForm("doctor_id")), 10, 64)
	if err != nil || doctorID == 0 {
		c.Redirect(http.StatusFound, fmt.Sprintf("/admin/sections/%d/doctors?msg=doctor_invalid", sec.ID))
		return
	}

	if err := h.Sections.AssignDoctorToSection(sec.ID, sec.ClinicID, uint(doctorID), 0); err != nil {
		switch {
		case errors.Is(err, repository.ErrSectionDoctorExists):
			c.Redirect(http.StatusFound, fmt.Sprintf("/admin/sections/%d/doctors?msg=doctor_already", sec.ID))
		case errors.Is(err, repository.ErrSectionDoctorNotEligible):
			c.Redirect(http.StatusFound, fmt.Sprintf("/admin/sections/%d/doctors?msg=doctor_not_approved", sec.ID))
		default:
			c.Redirect(http.StatusFound, fmt.Sprintf("/admin/sections/%d/doctors?msg=error", sec.ID))
		}
		return
	}

	c.Redirect(http.StatusFound, fmt.Sprintf("/admin/sections/%d/doctors?msg=doctor_assigned", sec.ID))
}

// DoctorRemove پزشک را از بخش جدا می‌کند.
// ورودی: gin.Context با شناسه بخش و doctor_id. خروجی: تغییر مسیر به لیست پزشکان بخش.
func (h *SectionAdminHandler) DoctorRemove(c *gin.Context) {
	_, sec, ok := h.requireSectionAccess(c)
	if !ok {
		return
	}

	doctorID, err := parseUintParam(c, "doctor_id")
	if err != nil {
		c.String(http.StatusBadRequest, "شناسه پزشک نامعتبر است.")
		return
	}

	if err := h.Sections.UnassignDoctorFromSection(sec.ID, doctorID); err != nil {
		c.Redirect(http.StatusFound, fmt.Sprintf("/admin/sections/%d/doctors?msg=error", sec.ID))
		return
	}

	c.Redirect(http.StatusFound, fmt.Sprintf("/admin/sections/%d/doctors?msg=doctor_removed", sec.ID))
}

// toSectionDoctorRows ردیف‌های انتصاب را به مدل نمای ادمین تبدیل می‌کند.
// ورودی: اسلایس SectionDoctor. خروجی: اسلایس SectionDoctorRow.
func toSectionDoctorRows(links []models.SectionDoctor) []adminviews.SectionDoctorRow {
	out := make([]adminviews.SectionDoctorRow, 0, len(links))
	for _, link := range links {
		out = append(out, adminviews.SectionDoctorRow{
			DoctorID:      link.DoctorID,
			Name:          sectionDoctorDisplayName(link.Doctor),
			SpecialtyName: link.Doctor.Specialty.Name,
			SystemID:      link.Doctor.DoctorSystemID,
			IsApproved:    link.Doctor.IsApproved,
			IsActive:      link.Doctor.IsActive,
			SortOrder:     link.SortOrder,
		})
	}
	return out
}

// toSectionDoctorOptions پزشکان قابل انتخاب را به گزینه‌های فرم تبدیل می‌کند.
// ورودی: اسلایس Doctor. خروجی: اسلایس SectionDoctorOption.
func toSectionDoctorOptions(doctors []models.Doctor) []adminviews.SectionDoctorOption {
	out := make([]adminviews.SectionDoctorOption, 0, len(doctors))
	for _, d := range doctors {
		out = append(out, adminviews.SectionDoctorOption{
			ID:            d.ID,
			Name:          sectionDoctorDisplayName(d),
			SpecialtyName: d.Specialty.Name,
		})
	}
	return out
}

// sectionDoctorDisplayName نام نمایشی پزشک را از فیلدهای نام می‌سازد.
// ورودی: مدل پزشک. خروجی: نام کامل یا ترکیب نام و نام خانوادگی.
func sectionDoctorDisplayName(d models.Doctor) string {
	if name := strings.TrimSpace(d.Name); name != "" {
		return name
	}
	return strings.TrimSpace(d.FirstName + " " + d.LastName)
}

// requireUserClinics verifies user authorization and returns their allowed clinics.
// Input: gin.Context.
// Output: AppointmentUser pointer, slice of Clinic, and boolean success.
func (h *SectionAdminHandler) requireUserClinics(c *gin.Context) (*models.AppointmentUser, []models.Clinic, bool) {
	user, ok := auth.UserFromGin(c)
	if !ok {
		c.AbortWithStatus(http.StatusUnauthorized)
		return nil, nil, false
	}
	allowed, err := h.Scope.AllowedClinics(user)
	if err != nil {
		c.String(http.StatusInternalServerError, "خطا در دریافت لیست مراکز")
		return nil, nil, false
	}
	if len(allowed) == 0 {
		c.String(http.StatusForbidden, "هیچ مرکزی برای حساب کاربری شما تعریف نشده است.")
		return nil, nil, false
	}
	return user, allowed, true
}

// requireSectionAccess verifies user access to a specific section by URL param section_id or id.
// Input: gin.Context.
// Output: AppointmentUser pointer, ClinicSection pointer, and boolean success.
func (h *SectionAdminHandler) requireSectionAccess(c *gin.Context) (*models.AppointmentUser, *models.AppointmentClinicSection, bool) {
	user, _, ok := h.requireUserClinics(c)
	if !ok {
		return nil, nil, false
	}

	paramKey := "section_id"
	if c.Param(paramKey) == "" {
		paramKey = "id"
	}
	id, err := parseUintParam(c, paramKey)
	if err != nil {
		c.String(http.StatusBadRequest, "شناسه بخش نامعتبر است.")
		return nil, nil, false
	}

	sec, err := h.Sections.GetSectionByID(id)
	if err != nil || sec == nil {
		c.String(http.StatusNotFound, "بخش مورد نظر یافت نشد.")
		return nil, nil, false
	}

	if !h.Scope.CanAccess(user, sec.ClinicID) {
		c.String(http.StatusForbidden, "دسترسی به این بخش مجاز نیست.")
		return nil, nil, false
	}

	return user, sec, true
}

// resolveClinicID extracts and verifies the target clinic ID.
// Input: gin.Context, user, allowed clinics list.
// Output: uint clinic ID.
func (h *SectionAdminHandler) resolveClinicID(c *gin.Context, user *models.AppointmentUser, allowed []models.Clinic) uint {
	if len(allowed) == 1 {
		return allowed[0].ID
	}
	raw := c.Query("clinic_id")
	if raw == "" {
		raw = c.PostForm("clinic_id")
	}
	id, _ := strconv.ParseUint(raw, 10, 64)
	if id == 0 || !h.Scope.CanAccess(user, uint(id)) {
		return allowed[0].ID
	}
	return uint(id)
}

// toSectionRows converts model sections to view rows with public URLs.
// Input: slice of ClinicSection and clinic ID.
// Output: slice of SectionRow.
func (h *SectionAdminHandler) toSectionRows(sections []models.AppointmentClinicSection, clinicID uint) []adminviews.SectionRow {
	clinic, _ := h.Clinics.GetByID(clinicID)
	prefix := ""
	if clinic != nil && clinic.Slug != nil {
		prefix = seo.ClinicPath(*clinic.Slug)
	}

	out := make([]adminviews.SectionRow, 0, len(sections))
	for _, s := range sections {
		publicURL := fmt.Sprintf("%s/section/%s", prefix, s.Slug)
		out = append(out, adminviews.SectionRow{
			ID:        s.ID,
			ClinicID:  s.ClinicID,
			Title:     s.Title,
			Slug:      s.Slug,
			SortOrder: s.SortOrder,
			IsActive:  s.IsActive,
			PublicURL: publicURL,
		})
	}
	return out
}

// toSectionSummaries creates navigation summary slice from clinic sections.
// Input: slice of ClinicSection.
// Output: slice of SectionNavSummary.
func toSectionSummaries(sections []models.AppointmentClinicSection) []adminviews.SectionNavSummary {
	out := make([]adminviews.SectionNavSummary, 0, len(sections))
	for _, s := range sections {
		out = append(out, adminviews.SectionNavSummary{
			ID:    s.ID,
			Title: s.Title,
		})
	}
	return out
}

// saveUploadedImage saves an uploaded image file or returns existing fallback URL.
// Input: gin.Context, form field name, existing image URL fallback.
// Output: saved image URL or error.
func (h *SectionAdminHandler) saveUploadedImage(c *gin.Context, fieldName string, fallback string) (string, error) {
	file, hdr, err := c.Request.FormFile(fieldName)
	if err != nil || file == nil {
		if c.PostForm("clear_"+fieldName) == "1" {
			return "", nil
		}
		return strings.TrimSpace(fallback), nil
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(hdr.Filename))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp":
	default:
		return "", fmt.Errorf("فرمت تصویر مجاز نیست.")
	}
	if hdr.Size > 5<<20 {
		return "", fmt.Errorf("حجم تصویر نباید بیش از ۵ مگابایت باشد.")
	}
	if err := os.MkdirAll(sectionUploadDir, 0o755); err != nil {
		return "", fmt.Errorf("خطا در ایجاد پوشه ذخیره‌سازی")
	}
	name := fmt.Sprintf("%d%s", time.Now().UnixNano(), ext)
	destPath := filepath.Join(sectionUploadDir, name)
	out, err := os.Create(destPath)
	if err != nil {
		return "", fmt.Errorf("خطا در ایجاد فایل")
	}
	defer out.Close()
	if _, err := io.Copy(out, file); err != nil {
		return "", fmt.Errorf("خطا در ذخیره فایل")
	}
	return "/static/uploads/sections/" + name, nil
}

// sanitizeSlug sanitizes a URL slug input string.
// Input: raw slug string.
// Output: trimmed and sanitized slug.
func sanitizeSlug(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, " ", "-")
	return s
}

// parsePercentForm parses a 0..100 integer from a form value.
// Input: raw form string and fallback. Output: clamped percent.
func parsePercentForm(raw string, fallback int) int {
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return models.ClampPercent(fallback, fallback)
	}
	return models.ClampPercent(n, fallback)
}

// parseServicesLines splits services text by newline and limits to maxCount.
// Input: raw string and maxCount.
// Output: slice of trimmed service strings.
func parseServicesLines(raw string, maxCount int) []string {
	lines := strings.Split(raw, "\n")
	var out []string
	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if trimmed != "" {
			out = append(out, trimmed)
			if len(out) >= maxCount {
				break
			}
		}
	}
	return out
}

// UpdateQuota سقف مجاز ایجاد بخش برای یک مرکز را توسط سوپر ادمین به‌روزرسانی می‌کند.
// ورودی: gin.Context شامل فیلدهای فرم (clinic_id، max_sections).
// خروجی: تغییر مسیر به صفحه لیست بخش‌ها همراه با پیام موفقیت یا خطا.
func (h *SectionAdminHandler) UpdateQuota(c *gin.Context) {
	user, allowed, ok := h.requireUserClinics(c)
	if !ok {
		return
	}
	if constants.UserRole(user.Role) != constants.UserRoleSuperAdmin {
		c.String(http.StatusForbidden, "تنها سوپر ادمین مجاز به تنظیم سقف مجاز بخش‌ها است.")
		return
	}

	clinicID := h.resolveClinicID(c, user, allowed)
	if clinicID == 0 {
		c.String(http.StatusBadRequest, "شناسه مرکز نامعتبر است.")
		return
	}

	maxSections, _ := strconv.Atoi(c.PostForm("max_sections"))
	if maxSections <= 0 {
		maxSections = 5
	}

	if err := h.Sections.SetClinicSectionQuota(clinicID, maxSections); err != nil {
		c.Redirect(http.StatusFound, fmt.Sprintf("/admin/sections?clinic_id=%d&msg=error", clinicID))
		return
	}

	c.Redirect(http.StatusFound, fmt.Sprintf("/admin/sections?clinic_id=%d&msg=quota_saved", clinicID))
}

// toSectionClinicOptions converts models.Clinic slice into adminviews.ClinicOption slice for the select dropdown.
// Input: slice of Clinic.
// Output: slice of ClinicOption.
func toSectionClinicOptions(clinics []models.Clinic) []adminviews.ClinicOption {
	opts := make([]adminviews.ClinicOption, 0, len(clinics))
	for _, c := range clinics {
		opts = append(opts, adminviews.ClinicOption{
			ID:   c.ID,
			Name: c.Name,
		})
	}
	return opts
}

// sectionFlashMessage translates query msg codes into Persian alert texts.
// Input: msg code string.
// Output: Persian message string.
func sectionFlashMessage(code string) string {
	switch code {
	case "created":
		return "بخش جدید با موفقیت ایجاد شد."
	case "updated":
		return "تغییرات با موفقیت ذخیره شد."
	case "deleted":
		return "بخش و اطلاعات مربوط به آن با موفقیت حذف شد."
	case "saved":
		return "اطلاعات با موفقیت ذخیره شد."
	case "empty_fields":
		return "لطفاً تمامی فیلدهای الزامی را پر کنید."
	case "duplicate_slug":
		return "شناسه روت (Slug) تکراری است. لطفاً شناسه دیگری انتخاب کنید."
	case "quota_exceeded":
		return "سقف مجاز ایجاد بخش برای این مرکز تکمیل شده است. برای افزایش سقف مجاز، با سوپر ادمین تماس بگیرید."
	case "quota_saved":
		return "سقف مجاز تعداد بخش‌های این مرکز با موفقیت ذخیره شد."
	case "msg_too_long":
		return "متن پیام نمی‌تواند بیش از ۸۰۰ کاراکتر باشد."
	case "empty_content":
		return "متن پیام نمی‌تواند خالی باشد."
	case "invalid_image":
		return "تصویر ارسالی نامعتبر یا حجم آن بیش از ۵ مگابایت است."
	case "doctor_assigned":
		return "پزشک تأییدشده با موفقیت به این بخش اضافه شد."
	case "doctor_removed":
		return "پزشک از این بخش حذف شد."
	case "doctor_already":
		return "این پزشک از قبل به بخش اضافه شده است."
	case "doctor_not_approved":
		return "فقط پزشکان تأییدشده همین مرکز را می‌توان به بخش افزود."
	case "doctor_invalid":
		return "لطفاً یک پزشک معتبر انتخاب کنید."
	case "error":
		return "خطایی در انجام عملیات رخ داد."
	default:
		return ""
	}
}
