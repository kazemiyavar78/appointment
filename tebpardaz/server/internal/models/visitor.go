package models

import "time"

// VisitorIP is one unique client IP and its aggregate visit counters.
type VisitorIP struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	IPAddress   string    `gorm:"type:nvarchar(45);not null;uniqueIndex" json:"ip_address"`
	VisitCount  uint      `gorm:"not null;default:1" json:"visit_count"`
	LastVisitAt time.Time `gorm:"type:datetime;not null" json:"last_visit_at"`
	CreatedAt   time.Time `gorm:"type:datetime;not null" json:"created_at"`
}

// TableName returns the database table for VisitorIP.
func (VisitorIP) TableName() string {
	return "visitor_ips"
}

// VisitDetail is a single buffered page visit attributed to a VisitorIP.
type VisitDetail struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	VisitorIPID  uint      `gorm:"not null;index" json:"visitor_ip_id"`
	Host         string    `gorm:"type:nvarchar(255);not null" json:"host"`
	ClinicID     *uint     `gorm:"index" json:"clinic_id"`
	Path         string    `gorm:"type:nvarchar(max);not null" json:"path"`
	FullURL      string    `gorm:"type:nvarchar(max);not null" json:"full_url"`
	Browser      string    `gorm:"type:nvarchar(50);not null" json:"browser"`
	OS           string    `gorm:"type:nvarchar(50);not null" json:"os"`
	Referrer     string    `gorm:"type:nvarchar(max);not null;default:''" json:"referrer"`
	IsFromGoogle bool      `gorm:"type:bit;not null;default:false" json:"is_from_google"`
	CreatedAt    time.Time `gorm:"type:datetime;not null;index" json:"created_at"`

	VisitorIP VisitorIP `gorm:"foreignKey:VisitorIPID" json:"-"`
}

// TableName returns the database table for VisitDetail.
func (VisitDetail) TableName() string {
	return "visitor_visit_details"
}
