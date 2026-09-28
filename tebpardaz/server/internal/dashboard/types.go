package dashboard

import "time"

// MetricID identifies a KPI so new cards can be registered without rewriting the page.
type MetricID string

const (
	MetricAppointmentsTotal     MetricID = "appointments_total"
	MetricAppointmentsConfirmed MetricID = "appointments_confirmed"
	MetricAppointmentsFailed    MetricID = "appointments_failed"
	MetricAppointmentsCancelled MetricID = "appointments_cancelled"
	MetricVisits                MetricID = "visits"
	MetricSuccessRate           MetricID = "success_rate"
	MetricConversionRate        MetricID = "conversion_rate"
	MetricActiveDoctors         MetricID = "active_doctors"
)

const (
	FormatCount   = "count"
	FormatPercent = "percent"

	SentimentPositive = "positive"
	SentimentNegative = "negative"
	SentimentNeutral  = "neutral"

	DirectionUp   = "up"
	DirectionDown = "down"
	DirectionFlat = "flat"
)

// Scope restricts dashboard queries to clinics the signed-in admin may see.
type Scope struct {
	ClinicID                uint
	ClinicIDs               []uint
	IncludeNullClinicVisits bool
}

// KPI is one card at the top of the dashboard.
type KPI struct {
	ID           MetricID `json:"id"`
	Label        string   `json:"label"`
	Icon         string   `json:"icon"`
	Format       string   `json:"format"`
	Value        float64  `json:"value"`
	Previous     float64  `json:"previous"`
	ValueText    string   `json:"value_text"`
	PreviousText string   `json:"previous_text"`
	ChangePct    *float64 `json:"change_pct,omitempty"`
	ChangeText   string   `json:"change_text"`
	Direction    string   `json:"direction"`
	Sentiment    string   `json:"sentiment"`
	Invert       bool     `json:"invert"`
	DetailURL    string   `json:"detail_url"`
	Hint         string   `json:"hint"`
}

// NamedCount is a ranked row (page, doctor, specialty, service).
type NamedCount struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Sub   string `json:"sub,omitempty"`
	Count int64  `json:"count"`
	Text  string `json:"text"`
	URL   string `json:"url,omitempty"`
}

// HourCount is traffic or bookings for one clock hour (0-23).
type HourCount struct {
	Hour  int    `json:"hour"`
	Label string `json:"label"`
	Count int64  `json:"count"`
	Text  string `json:"text"`
	Share int    `json:"share"`
}

// DayPoint is one day on the combined visits/appointments chart.
type DayPoint struct {
	Date        string `json:"date"`
	Label       string `json:"label"`
	Visits      int64  `json:"visits"`
	Appointments int64 `json:"appointments"`
	VisitsText  string `json:"visits_text"`
	ApptText    string `json:"appointments_text"`
}

// BookingStatus is the live mix of appointment outcomes plus open slot capacity.
type BookingStatus struct {
	Confirmed     int64        `json:"confirmed"`
	Failed        int64        `json:"failed"`
	Cancelled     int64        `json:"cancelled"`
	Pending       int64        `json:"pending"`
	ConfirmedText string       `json:"confirmed_text"`
	FailedText    string       `json:"failed_text"`
	CancelledText string       `json:"cancelled_text"`
	PendingText   string       `json:"pending_text"`
	OpenDoctors   []NamedCount `json:"open_doctors"`
}

// Snapshot is the full dashboard payload rendered as HTML and returned as JSON.
type Snapshot struct {
	Range        RangeKey       `json:"range"`
	RangeLabel   string         `json:"range_label"`
	CompareLabel string         `json:"compare_label"`
	From         time.Time      `json:"from"`
	To           time.Time      `json:"to"`
	FromDate     string         `json:"from_date"`
	ToDate       string         `json:"to_date"`
	UpdatedAt    time.Time      `json:"updated_at"`
	UpdatedText  string         `json:"updated_text"`
	ClinicID     uint           `json:"clinic_id"`
	KPIs         []KPI          `json:"kpis"`
	Daily        []DayPoint     `json:"daily"`
	TopPages     []NamedCount   `json:"top_pages"`
	TopDoctors   []NamedCount   `json:"top_doctors"`
	TopSpecialties []NamedCount `json:"top_specialties"`
	TopServices  []NamedCount   `json:"top_services"`
	PeakHours    []HourCount    `json:"peak_hours"`
	BookingHours []HourCount    `json:"booking_hours"`
	Booking      BookingStatus  `json:"booking"`
	Insights     []string       `json:"insights"`
}

// KPIByID returns the card with the given id, or nil.
func (s Snapshot) KPIByID(id MetricID) *KPI {
	for i := range s.KPIs {
		if s.KPIs[i].ID == id {
			return &s.KPIs[i]
		}
	}
	return nil
}
