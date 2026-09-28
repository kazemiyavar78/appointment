package booking

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"tebpardaz/server/internal/cache"
	"tebpardaz/server/internal/models"
	"tebpardaz/server/internal/repository"
	"tebpardaz/server/internal/statusnotify"
	"tebpardaz/server/internal/websocket"
	"tebpardaz/shared/protocol"

	"github.com/google/uuid"
)

var (
	// ErrClinicOffline is returned when the clinic WebSocket is not connected.
	ErrClinicOffline = errors.New("clinic offline")
	// ErrSlotUnavailable is returned when the selected slot is missing or full in cache.
	ErrSlotUnavailable = errors.New("slot unavailable")
	// ErrClinicRejected is returned when the clinic client rejects the booking.
	ErrClinicRejected = errors.New("clinic rejected")
)

const clinicBookingTimeout = 45 * time.Second
const maxAppointmentIPLen = 45

// ProgressFn receives progress events during Book (optional; may be nil).
type ProgressFn func(ProgressEvent)

// BookRequest is the input for a public website booking (raw form fields).
type BookRequest struct {
	ClinicID   uint
	ClinicCode int
	ClinicName string
	Doctor     *models.Doctor
	ClientIP   string
	SlotID     string
	FirstName  string
	LastName   string
	NationalID string
	Mobile     string
	BirthDate  string
	Sex        string
	OTPTicket  string // one-time ticket from OTP verify step
}

// BookResult is the outcome of a booking attempt.
type BookResult struct {
	OK           bool
	Message      string
	ExternalID   string
	TrackingCode string
	Duplicate    bool
}

// Service coordinates validation, persistence, and clinic WebSocket dispatch.
type Service struct {
	Hub          *websocket.Hub
	Slots        *cache.SlotCache
	Appointments *repository.AppointmentRepo
	Guard        *Guard
	OTP          *OTPStore
	Notifier     *Notifier
	Status       *statusnotify.Notifier
}

// NewService constructs a booking Service.
// Inputs: hub, slot cache, appointment repo, abuse/rate guard, OTP store, notifier.
// Output: pointer to Service.
func NewService(
	hub *websocket.Hub,
	slots *cache.SlotCache,
	appointments *repository.AppointmentRepo,
	guard *Guard,
	otp *OTPStore,
	notifier *Notifier,
	status *statusnotify.Notifier,
) *Service {
	return &Service{
		Hub:          hub,
		Slots:        slots,
		Appointments: appointments,
		Guard:        guard,
		OTP:          otp,
		Notifier:     notifier,
		Status:       status,
	}
}

// Book runs the end-to-end public booking flow and reports progress via emit.
// Inputs: req (clinic/doctor/raw form fields), emit progress callback (may be nil).
// Output: BookResult and error (transport/validation); business failures are in BookResult.
func (s *Service) Book(req BookRequest, emit ProgressFn) (*BookResult, error) {
	if emit == nil {
		emit = func(ProgressEvent) {}
	}
	if req.Doctor == nil {
		return &BookResult{OK: false, Message: "درخواست نامعتبر است"}, fmt.Errorf("missing doctor")
	}

	emit(NewStepLoading(StepReceived))
	emit(NewStepDone(StepReceived))

	emit(NewStepLoading(StepValidated))
	form, err := ValidatePatientForm(req.FirstName, req.LastName, req.NationalID, req.Mobile, req.BirthDate, req.Sex)
	if err != nil {
		msg := err.Error()
		if i := strings.Index(msg, ": "); i >= 0 {
			msg = msg[i+2:]
		}
		return &BookResult{OK: false, Message: msg}, ErrValidation
	}
	form.SlotID = strings.TrimSpace(req.SlotID)
	if form.SlotID == "" {
		return &BookResult{OK: false, Message: "نوبت انتخاب نشده است"}, ErrValidation
	}

	if s.OTP != nil {
		if err := s.OTP.ConsumeTicket(req.OTPTicket, form.Mobile, form.NationalID); err != nil {
			msg := "ابتدا شماره موبایل را با کد تایید کنید"
			switch {
			case errors.Is(err, ErrOTPExpired):
				msg = "اعتبار تایید موبایل تمام شده؛ دوباره کد بگیرید"
			case errors.Is(err, ErrOTPSessionExhausted):
				msg = "سقف ۵ نوبت با این موبایل پر شده؛ دوباره کد تایید بگیرید"
			}
			return &BookResult{OK: false, Message: msg}, ErrOTPNotVerified
		}
	}

	slot, ok := s.findSlot(req.ClinicID, req.Doctor.ID, form.SlotID)
	if !ok {
		return &BookResult{OK: false, Message: "نوبت انتخاب‌شده در دسترس نیست"}, ErrSlotUnavailable
	}
	emit(NewStepDone(StepValidated))

	if s.Hub == nil || !s.Hub.IsOnline(req.ClinicID) {
		return &BookResult{OK: false, Message: "کلینیک در حال حاضر آنلاین نیست؛ بعداً تلاش کنید"}, ErrClinicOffline
	}

	patient := &models.Patient{
		NationalID: form.NationalID,
		FirstName:  form.FirstName,
		LastName:   form.LastName,
		Mobile:     form.Mobile,
		BirthDate:  form.BirthDate,
		Sex:        models.Sex(form.Sex),
	}
	var patientID uint
	if s.Appointments != nil {
		saved, err := s.Appointments.UpsertPatient(patient)
		if err != nil {
			return &BookResult{OK: false, Message: "خطا در ذخیره اطلاعات بیمار"}, err
		}
		patientID = saved.ID
	}

	idem := uuid.NewString()
	appt := &models.PatientAppointment{
		ClinicID:       req.ClinicID,
		PatientID:      patientID,
		DoctorID:       req.Doctor.ID,
		Status:         "pending",
		IdempotencyKey: idem,
		StartsAt:       slot.StartsAt,
		EndsAt:         slot.EndsAt,
		Source:         "website",
		IPAddress:      normalizeAppointmentIP(req.ClientIP),
	}
	if s.Appointments != nil && patientID != 0 {
		if err := s.Appointments.CreatePending(appt); err != nil {
			return &BookResult{OK: false, Message: "خطا در ایجاد نوبت"}, err
		}
		if s.OTP != nil {
			_ = s.OTP.MarkBooked(form.Mobile)
		}
	}

	payload := protocol.BookingCreate{
		IdempotencyKey:    idem,
		SiteAppointmentID: appt.ID,
		DoctorExternalID:  req.Doctor.ExternalID,
		ExternalSlotID:    slot.ExternalSlotID,
		StartsAtUnix:      slot.StartsAt.Unix(),
		EndsAtUnix:        slot.EndsAt.Unix(),
		StartDateTime:     slot.StartsAt,
		NationalID:        form.NationalID,
		FirstName:         form.FirstName,
		LastName:          form.LastName,
		Mobile:            form.Mobile,
		BirthDate:         form.BirthJalali,
		Sex:               form.SexCode,
	}

	emit(NewStepLoading(StepSentClinic))
	ack, err := s.Hub.RequestBookingCreate(req.ClinicID, clinicBookingTimeout, payload)
	if err != nil {
		if s.Appointments != nil && appt.ID != 0 {
			_ = s.Appointments.MarkFailed(appt.ID, err.Error())
		}
		if errors.Is(err, websocket.ErrClinicOffline) {
			emit(NewStepDone(StepSentClinic))
			return &BookResult{OK: false, Message: "کلینیک در حال حاضر آنلاین نیست"}, ErrClinicOffline
		}
		if errors.Is(err, websocket.ErrRequestTimeout) {
			emit(NewStepDone(StepSentClinic))
			return &BookResult{OK: false, Message: "پاسخ کلینیک به‌موقع دریافت نشد"}, err
		}
		emit(NewStepDone(StepSentClinic))
		return &BookResult{OK: false, Message: "خطا در ارسال به کلینیک"}, err
	}
	emit(NewStepDone(StepSentClinic))

	emit(NewStepLoading(StepClinicAck))
	if ack == nil || !ack.OK {
		msg := "ثبت نوبت در کلینیک ناموفق بود"
		code := ""
		if ack != nil {
			if ack.Message != "" {
				msg = ack.Message
			}
			code = ack.ErrorCode
		}
		duplicate := code == string(protocol.ErrCodeConflict) || code == string(protocol.ErrCodeAlreadyBooked)
		if duplicate && s.Guard != nil {
			s.Guard.RecordDuplicateAttempt(req.ClientIP)
		}
		if clinicRejectedForCapacity(code, msg) && s.Slots != nil {
			s.Slots.DropSlot(req.ClinicID, req.Doctor.ID, slot.ExternalSlotID, slot.StartsAt)
		}
		if s.Appointments != nil && appt.ID != 0 {
			_ = s.Appointments.MarkFailed(appt.ID, msg)
		}
		return &BookResult{OK: false, Message: msg, Duplicate: duplicate}, ErrClinicRejected
	}

	if s.Appointments != nil && appt.ID != 0 {
		_ = s.Appointments.MarkConfirmed(appt.ID, ack.ExternalID, time.Now())
	}
	emit(NewStepDone(StepClinicAck))

	doctorName := ""
	if req.Doctor != nil {
		doctorName = strings.TrimSpace(req.Doctor.Name)
		if doctorName == "" {
			doctorName = strings.TrimSpace(req.Doctor.FirstName + " " + req.Doctor.LastName)
		}
	}
	if s.Notifier != nil {
		_ = s.Notifier.NotifyBookingConfirmed(context.Background(), BookingNotifyParams{
			Phone:        form.Mobile,
			FirstName:    form.FirstName,
			LastName:     form.LastName,
			NationalID:   form.NationalID,
			ClinicCode:   req.ClinicCode,
			ClinicName:   req.ClinicName,
			DoctorName:   doctorName,
			StartsAt:     slot.StartsAt,
			TrackingCode: FormatTrackingCode(appt.ID),
			ClientIP:     req.ClientIP,
		})
	}
	if s.Status != nil {
		s.Status.NotifyBookingSuccess(
			req.ClinicID,
			req.ClinicCode,
			req.ClinicName,
			form.FirstName,
			form.LastName,
			form.NationalID,
			form.Mobile,
			doctorName,
			FormatTrackingCode(appt.ID),
			req.ClientIP,
		)
	}

	return &BookResult{
		OK:           true,
		Message:      "نوبت شما با موفقیت ثبت شد؛ پیامک تایید ارسال شد",
		ExternalID:   ack.ExternalID,
		TrackingCode: FormatTrackingCode(appt.ID),
	}, nil
}

// clinicRejectedForCapacity تشخیص می‌دهد مرکز به‌خاطر تکمیل ظرفیت، نوبت را رد کرده است.
// ورودی: کد خطای پروتکل و پیام متنی ack. خروجی: true برای no_capacity یا پیام ظرفیت تکمیل‌شده.
func clinicRejectedForCapacity(code, message string) bool {
	if code == string(protocol.ErrCodeNoCapacity) {
		return true
	}
	return strings.Contains(message, "ظرفیت این نوبت تکمیل شده")
}

func (s *Service) findSlot(clinicID, doctorID uint, slotKey string) (models.DoctorSlot, bool) {
	slotKey = strings.TrimSpace(slotKey)
	if s.Slots == nil || slotKey == "" {
		return models.DoctorSlot{}, false
	}
	slots := s.Slots.AvailableForDoctor(clinicID, doctorID, time.Now(), 0)
	for _, slot := range slots {
		ext := strings.TrimSpace(slot.ExternalSlotID)
		rfc := slot.StartsAt.UTC().Format(time.RFC3339)
		if slotKey == ext || slotKey == rfc {
			return slot, true
		}
	}
	return models.DoctorSlot{}, false
}

// normalizeAppointmentIP trims a client IP and caps it to the PatientAppointment column length.
// Inputs: raw IP from the booking request.
// Output: stored IP string, possibly empty.
func normalizeAppointmentIP(ip string) string {
	ip = strings.TrimSpace(ip)
	if len(ip) > maxAppointmentIPLen {
		return ip[:maxAppointmentIPLen]
	}
	return ip
}
