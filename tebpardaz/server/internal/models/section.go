package models

import (
	"time"

	"gorm.io/gorm"
)

// ClinicSection represents an independent department or section defined by a clinic/center.
// Each section has its own dedicated public route and 4 fixed sub-sections:
// Banner, Working Hours, Messages to Patients, and Equipment Introductions.
type AppointmentClinicSection struct {
	gorm.Model
	ClinicID  uint   `gorm:"index;not null" json:"clinic_id"`
	Title     string `gorm:"type:nvarchar(100);not null" json:"title"`
	Slug      string `gorm:"type:nvarchar(100);not null;index" json:"slug"`
	SortOrder int    `gorm:"type:int;not null;default:0" json:"sort_order"`
	IsActive  bool   `gorm:"type:bit;not null;default:true" json:"is_active"`

	Banner    *SectionBanner     `gorm:"foreignKey:SectionID" json:"banner,omitempty"`
	Schedules []SectionSchedule  `gorm:"foreignKey:SectionID" json:"schedules,omitempty"`
	Messages  []SectionMessage   `gorm:"foreignKey:SectionID" json:"messages,omitempty"`
	Equipment []SectionEquipment `gorm:"foreignKey:SectionID" json:"equipment,omitempty"`
}

// TableName specifies the database table name for ClinicSection.
func (AppointmentClinicSection) TableName() string {
	return "appointment_clinic_sections"
}

// SectionBanner holds the header banner configuration for a clinic section.
type SectionBanner struct {
	gorm.Model
	SectionID       uint   `gorm:"uniqueIndex;not null" json:"section_id"`
	Slogan          string `gorm:"type:nvarchar(255);not null;default:''" json:"slogan"`
	Description     string `gorm:"type:nvarchar(500);not null;default:''" json:"description"`
	Services        string `gorm:"type:nvarchar(1000);not null;default:''" json:"services"`
	BackgroundColor string `gorm:"type:nvarchar(50);not null;default:'#0a2e2e'" json:"background_color"`
	ImageURL        string `gorm:"type:nvarchar(500);not null;default:''" json:"image_url"`
}

// TableName specifies the database table name for SectionBanner.
func (SectionBanner) TableName() string {
	return "section_banners"
}

// SectionSchedule defines working hours / shifts for a day of the week in a section.
// Days are indexed 0 (شنبه) to 6 (جمعه).
type SectionSchedule struct {
	gorm.Model
	SectionID  uint   `gorm:"index;not null" json:"section_id"`
	DayOfWeek  int    `gorm:"type:int;not null" json:"day_of_week"`
	DayName    string `gorm:"type:nvarchar(20);not null" json:"day_name"`
	IsOpen     bool   `gorm:"type:bit;not null;default:true" json:"is_open"`
	Shift1Start string `gorm:"type:nvarchar(10);not null;default:'08:00'" json:"shift1_start"`
	Shift1End   string `gorm:"type:nvarchar(10);not null;default:'14:00'" json:"shift1_end"`
	Shift2Start string `gorm:"type:nvarchar(10);not null;default:'16:00'" json:"shift2_start"`
	Shift2End   string `gorm:"type:nvarchar(10);not null;default:'20:00'" json:"shift2_end"`
	HasShift2   bool   `gorm:"type:bit;not null;default:false" json:"has_shift2"`
}

// TableName specifies the database table name for SectionSchedule.
func (SectionSchedule) TableName() string {
	return "section_schedules"
}

// SectionMessage holds a text message to patients with max 800 characters length.
type SectionMessage struct {
	gorm.Model
	SectionID   uint   `gorm:"index;not null" json:"section_id"`
	Title       string `gorm:"type:nvarchar(100);not null;default:''" json:"title"`
	Content     string `gorm:"type:nvarchar(800);not null" json:"content"`
	SenderTitle string `gorm:"type:nvarchar(100);not null;default:''" json:"sender_title"`
	SortOrder   int    `gorm:"type:int;not null;default:0" json:"sort_order"`
	IsActive    bool   `gorm:"type:bit;not null;default:true" json:"is_active"`
}

// TableName specifies the database table name for SectionMessage.
func (SectionMessage) TableName() string {
	return "section_messages"
}

// SectionEquipment represents an equipment introduction item.
// It consists of two parts: a centered quote box and a 2-column image/content layout.
type SectionEquipment struct {
	gorm.Model
	SectionID   uint   `gorm:"index;not null" json:"section_id"`
	QuoteTitle  string `gorm:"type:nvarchar(200);not null;default:''" json:"quote_title"`
	QuoteText   string `gorm:"type:nvarchar(1500);not null;default:''" json:"quote_text"`
	Badge       string `gorm:"type:nvarchar(100);not null;default:'فناوری روز دنیا'" json:"badge"`
	ImageURL    string `gorm:"type:nvarchar(500);not null;default:''" json:"image_url"`
	Title       string `gorm:"type:nvarchar(200);not null" json:"title"`
	Subtitle    string `gorm:"type:nvarchar(200);not null;default:''" json:"subtitle"`
	Description string `gorm:"type:nvarchar(2000);not null" json:"description"`
	Tags        string `gorm:"type:nvarchar(500);not null;default:''" json:"tags"`
	ButtonText  string `gorm:"type:nvarchar(100);not null;default:''" json:"button_text"`
	ButtonURL   string `gorm:"type:nvarchar(500);not null;default:''" json:"button_url"`
	SortOrder   int    `gorm:"type:int;not null;default:0" json:"sort_order"`
	IsActive    bool   `gorm:"type:bit;not null;default:true" json:"is_active"`
}

// TableName specifies the database table name for SectionEquipment.
func (SectionEquipment) TableName() string {
	return "section_equipments"
}




type Section struct {
	gorm.Model
	ID      uint     `gorm:"primaryKey"`
	Name    string   `gorm:"type:nvarchar(100);not null"`
	

	// ✅ رابطه صحیح
	ClinicSections []ClinicSection `gorm:"foreignKey:SectionID"`
}

// جدول واسط برای ارتباط many-to-many بین Clinic و Section
type ClinicSection struct {
	ID        uint `gorm:"primaryKey"`
	ClinicID  uint `gorm:"not null;index"`
	SectionID uint `gorm:"not null;index"`

	AddedBy   uint      `gorm:"not null"`
	AddedAt   time.Time `gorm:"not null;default:GETDATE()"`
	DeletedAt *time.Time

	Clinic  Clinic  `gorm:"foreignKey:ClinicID"`
	Section Section `gorm:"foreignKey:SectionID"`
}

// ClinicSectionQuota سقف مجاز تعریف بخش برای هر مرکز را که توسط سوپرادمین تنظیم می‌شود نگه می‌دارد.
type ClinicSectionQuota struct {
	gorm.Model
	ClinicID    uint `gorm:"uniqueIndex;not null" json:"clinic_id"`
	MaxSections int  `gorm:"type:int;not null;default:5" json:"max_sections"`
}

// TableName نام جدول مربوط به سقف بخش‌های کلینیک را برمی‌گرداند.
// ورودی: ریسیور ClinicSectionQuota. خروجی: نام جدول در دیتابیس.
func (ClinicSectionQuota) TableName() string {
	return "clinic_section_quotas"
}

