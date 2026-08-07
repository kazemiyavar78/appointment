package services

import (
	"tebpardaz/client/internal/localdb"
	"tebpardaz/shared/protocol"
)

// AppointmentService لیست نوبت/اسلات را از HIS محلی به سایت مرکزی پوش می‌کند.
// سرور این داده را در دیتابیس ذخیره نمی‌کند؛ فقط در کش با انقضای ۴ ساعت نگه می‌دارد.
type AppointmentService struct {
	Store  *localdb.Store
	Sender Sender
}

// NewAppointmentService سازنده AppointmentService است.
// ورودی: store (دسترسی HIS محلی)، sender (خروجی WebSocket).
// خروجی: اشاره‌گر به AppointmentService.
func NewAppointmentService(store *localdb.Store, sender Sender) *AppointmentService {
	return &AppointmentService{Store: store, Sender: sender}
}

// PushAppointmentsForDoctor نوبت‌های یک پزشک را می‌خواند و به سایت پوش می‌کند.
// ورودی: requestID، doctorExternalID (کد خارجی پزشک در HIS)، fullReplace.
// خروجی: خطا از خواندن محلی یا ارسال WebSocket.
// کاربرد: پاسخ به درخواست سرور با scope=one برای بروزرسانی یک پزشک.
func (s *AppointmentService) PushAppointmentsForDoctor(requestID, doctorExternalID string, fullReplace bool) error {
	doctorCode, err := s.Store.FindDoctorByExternalID(doctorExternalID)
	if err != nil {
		return err
	}

	rows, err := s.Store.ListAppointmentsByDoctor(doctorCode)
	if err != nil {
		return err
	}
	return s.push(requestID, protocol.ScopeOneDoctor, doctorExternalID, rows, fullReplace)
}

// PushAppointmentsForAllDoctors نوبت‌های همه پزشکان سینک‌شده را پوش می‌کند.
// ورودی: requestID، fullReplace.
// خروجی: خطا از خواندن محلی یا ارسال WebSocket.
// کاربرد: سینک خودکار هر ۱ دقیقه و پاسخ به درخواست سرور با scope=all.
func (s *AppointmentService) PushAppointmentsForAllDoctors(requestID string, fullReplace bool) error {
	rows, err := s.Store.ListAppointmentsForDoctors()
	if err != nil {
		return err
	}
	return s.push(requestID, protocol.ScopeAllDoctors, "", rows, fullReplace)
}

// HandleListRequest به appointment.list.request سمت سرور پاسخ می‌دهد.
// ورودی: requestID، req (محدوده یک پزشک یا همه پزشکان).
// خروجی: خطا از کمک‌کننده‌های پوش.
// سرور می‌تواند بروزرسانی یک پزشک (scope=one) یا همه پزشکان (scope=all) را درخواست کند.
func (s *AppointmentService) HandleListRequest(requestID string, req *protocol.AppointmentListRequest) error {
	if req == nil {
		return nil
	}
	switch req.Scope {
	case protocol.ScopeOneDoctor:
		// بروزرسانی فقط یک پزشک
		return s.PushAppointmentsForDoctor(requestID, req.DoctorExternalID, true)
	default:
		// بروزرسانی همه پزشکان مرکز
		return s.PushAppointmentsForAllDoctors(requestID, true)
	}
}

// push ردیف‌های محلی را به DTO پروتکل نگاشت کرده و appointment.list.push می‌فرستد.
func (s *AppointmentService) push(requestID string, scope protocol.AppointmentListScope, doctorExternalID string, rows []localdb.LocalAppointment, fullReplace bool) error {
	items := make([]protocol.AppointmentDTO, 0, len(rows))
	for _, row := range rows {
		items = append(items, protocol.AppointmentDTO{
			ExternalSlotID:   row.ExternalSlotID,
			DoctorExternalID: row.DoctorExternalID,
			StartsAtUnix:     row.StartsAt.Unix(),
			EndsAtUnix:       row.EndsAt.Unix(),
			Capacity:         row.Capacity,
			BookedCount:      row.BookedCount,
			IsAvailable:      row.IsAvailable,
		})
	}
	env, err := protocol.NewEnvelope(protocol.TypeAppointmentListPush, requestID, protocol.AppointmentListPush{
		Scope:            scope,
		DoctorExternalID: doctorExternalID,
		Appointments:     items,
		FullReplace:      fullReplace,
	})
	if err != nil {
		return err
	}
	return s.Sender.SendEnvelope(env)
}
