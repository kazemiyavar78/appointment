package websocket

import (
	"fmt"
	"net/http"
	"strconv"

	"tebpardaz/server/internal/cache"
	"tebpardaz/server/internal/models"
	"tebpardaz/server/internal/repository"
	"tebpardaz/shared/protocol"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"gorm.io/gorm"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// Handlers نقاط HTTP ارتقای WebSocket کلینیک و فریم‌های ورودی را در اختیار می‌گذارد.
type Handlers struct {
	Hub     *Hub
	Clinics *repository.ClinicRepo
	Doctors *repository.DoctorRepo
	// Slots کش اسلات‌های رزرو (بدون دیتابیس؛ TTL چهار ساعته)
	Slots *cache.SlotCache
	// PendingDoctors کش پزشکان تأییدنشده (بدون دیتابیس)
	PendingDoctors *cache.DoctorCache
	// WeeklyReserves کش لیست نوبت هفتگی پزشکان (بدون دیتابیس)
	WeeklyReserves *cache.WeeklyReserveCache
	// WaitingQueues کش صف انتظار بیماران (بدون دیتابیس)
	WaitingQueues *cache.WaitingQueueCache
}

// NewHandlers سازنده هندلرهای HTTP مربوط به WebSocket است.
// ورودی: hub، ریپوی مراکز، ریپوی پزشکان، کش اسلات‌ها، کش پزشکان تأییدنشده، کش نوبت هفتگی، کش صف انتظار.
// خروجی: اشاره‌گر به Handlers.
func NewHandlers(
	hub *Hub,
	clinics *repository.ClinicRepo,
	doctors *repository.DoctorRepo,
	slots *cache.SlotCache,
	pending *cache.DoctorCache,
	weekly *cache.WeeklyReserveCache,
	waiting *cache.WaitingQueueCache,
) *Handlers {
	return &Handlers{
		Hub:            hub,
		Clinics:        clinics,
		Doctors:        doctors,
		Slots:          slots,
		PendingDoctors: pending,
		WeeklyReserves: weekly,
		WaitingQueues:  waiting,
	}
}

// ServeWS اتصال را ارتقا می‌دهد و کلاینت مرکز را ثبت می‌کند.
func (h *Handlers) ServeWS(c *gin.Context) {
	clinic, err := h.authenticate(c.Request)
	if err != nil {
		// لاگ شکست احراز هویت مرکز
		LogClinicBehavior(0, ActionAuthFail, "", "", "احراز هویت مرکز ناموفق", err)
		c.String(http.StatusUnauthorized, err.Error())
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		// لاگ شکست ارتقای اتصال به WebSocket
		LogClinicBehavior(clinic.ID, ActionUpgradeFail, "", "", "ارتقای WebSocket ناموفق", err)
		return
	}

	client := NewClientConn(h.Hub, clinic.ID, conn)
	h.Hub.Register(client)
	go client.WritePump()
	client.ReadPump(h.handleFrame)
}

// authenticate کلینیک را از query/header کلید کلاینت WebSocket تشخیص می‌دهد.
func (h *Handlers) authenticate(r *http.Request) (*models.Clinic, error) {
	key := r.URL.Query().Get("clinic_key")
	if key == "" {
		key = r.Header.Get("X-Clinic-Key")
	}
	if key == "" {
		return nil, fmt.Errorf("missing clinic_key")
	}
	if h.Clinics == nil || h.Clinics.DB == nil {
		// حالت توسعه: اگر DB مدیریت در دسترس نباشد، clinic id عددی پذیرفته می‌شود
		if id, err := strconv.ParseUint(key, 10, 64); err == nil {
			return &models.Clinic{Model: gorm.Model{ID: uint(id)}, WSClientKey: key}, nil
		}
		return nil, fmt.Errorf("clinic lookup unavailable")
	}
	return h.Clinics.GetByWSClientKey(key)
}

// handleFrame یک envelope پروتکل ورودی از کلاینت مرکز را مسیر‌دهی می‌کند.
func (h *Handlers) handleFrame(client *ClientConn, raw []byte) error {
	env, err := DecodeEnvelope(raw)
	if err != nil {
		LogClinicBehavior(client.ClinicID, ActionError, "", "", "decode envelope ناموفق", err)
		return err
	}
	// لاگ دریافت پیام از مرکز
	LogClinicBehavior(client.ClinicID, ActionRecv, string(env.Type), env.RequestID, "پیام از مرکز دریافت شد", nil)

	switch env.Type {
	case protocol.TypePing:
		pong, err := protocol.NewEnvelope(protocol.TypePong, env.RequestID, nil)
		if err != nil {
			return err
		}
		return client.SendEnvelope(pong)
	case protocol.TypeDoctorListPush:
		var push protocol.DoctorListPush
		if err := env.DecodePayload(&push); err != nil {
			return err
		}
		return h.handleDoctorListPush(client, env, &push)
	case protocol.TypeAppointmentListPush:
		var push protocol.AppointmentListPush
		if err := env.DecodePayload(&push); err != nil {
			return err
		}
		return h.handleAppointmentListPush(client, env, &push)
	case protocol.TypeWeeklyReserveListPush:
		var push protocol.WeeklyReserveListPush
		if err := env.DecodePayload(&push); err != nil {
			return err
		}
		return h.handleWeeklyReserveListPush(client, env, &push)
	case protocol.TypeWaitingQueueListPush:
		var push protocol.WaitingQueueListPush
		if err := env.DecodePayload(&push); err != nil {
			return err
		}
		return h.handleWaitingQueueListPush(client, env, &push)
	case protocol.TypeBookingCreateAck, protocol.TypeBookingCancelAck, protocol.TypeTestResultResponse:
		_ = h.Hub.DeliverReply(client.ClinicID, env)
		return nil
	default:
		return nil
	}
}

// handleDoctorListPush پزشکان تأییدشده را بر اساس کد ملی+کلینیک بروزرسانی می‌کند؛
// پزشکان تأییدنشده فقط در کش نگه داشته می‌شوند (بدون درج در دیتابیس) و ACK برمی‌گردد.
// ورودی: کلاینت مرکز، envelope، payload پوش پزشکان.
// خروجی: خطا در صورت شکست سینک یا ارسال ACK.
func (h *Handlers) handleDoctorListPush(client *ClientConn, env *protocol.Envelope, push *protocol.DoctorListPush) error {
	if h.Doctors == nil {
		return fmt.Errorf("doctor repo unavailable")
	}
	results, pending, err := h.Doctors.SyncFromClinic(client.ClinicID, push.Doctors)
	if err != nil {
		return err
	}
	if h.PendingDoctors != nil {
		h.PendingDoctors.ReplacePending(client.ClinicID, pending)
	}
	ack := protocol.DoctorListAck{
		Accepted: len(results),
		Results:  make([]protocol.DoctorSyncResult, 0, len(results)),
	}
	for _, r := range results {
		if r.Pending {
			ack.Pending++
		}
		ack.Results = append(ack.Results, protocol.DoctorSyncResult{
			LocalCode:  r.LocalCode,
			ExternalID: r.ExternalID,
			DoctorID:   r.DoctorID,
		})
	}
	out, err := protocol.NewEnvelope(protocol.TypeDoctorListAck, env.RequestID, ack)
	if err != nil {
		return err
	}
	if err := client.SendEnvelope(out); err != nil {
		return err
	}
	// تکمیل waiter مربوط به doctor.list.request (دریافت لیست زنده ادمین)
	_ = h.Hub.DeliverReply(client.ClinicID, env)
	return nil
}

// handleAppointmentListPush اسلات‌های سینک‌شده را در کش می‌نویسد (نه دیتابیس) و ACK می‌فرستد.
// ورودی: کلاینت مرکز، envelope، payload پوش نوبت‌ها.
// خروجی: خطا در صورت شکست نوشتن کش یا ارسال ACK.
func (h *Handlers) handleAppointmentListPush(client *ClientConn, env *protocol.Envelope, push *protocol.AppointmentListPush) error {
	accepted := 0
	if h.Slots != nil && h.Doctors != nil {
		doctors, err := h.Doctors.ListApprovedByClinic(client.ClinicID)
		if err != nil {
			return err
		}
		// ذخیره فقط در کش با انقضای ۴ ساعت
		accepted, err = h.Slots.UpsertFromPush(client.ClinicID, push, doctors)
		if err != nil {
			return err
		}
	} else if push != nil {
		accepted = len(push.Appointments)
	}

	ack := protocol.AppointmentListAck{Accepted: accepted, OK: true}
	out, err := protocol.NewEnvelope(protocol.TypeAppointmentListAck, env.RequestID, ack)
	if err != nil {
		return err
	}
	if err := client.SendEnvelope(out); err != nil {
		return err
	}
	// تکمیل waiter مربوط به appointment.list.request (بروزرسانی یک/همه پزشکان از سمت سرور)
	_ = h.Hub.DeliverReply(client.ClinicID, env)
	return nil
}

// handleWeeklyReserveListPush لیست نوبت هفتگی را در کش می‌نویسد (نه دیتابیس) و ACK می‌فرستد.
// ورودی: کلاینت مرکز، envelope، payload پوش نوبت هفتگی.
// خروجی: خطا در صورت شکست نوشتن کش یا ارسال ACK.
func (h *Handlers) handleWeeklyReserveListPush(client *ClientConn, env *protocol.Envelope, push *protocol.WeeklyReserveListPush) error {
	accepted := 0
	if h.WeeklyReserves != nil {
		accepted = h.WeeklyReserves.ReplaceFromPush(client.ClinicID, push)
	} else if push != nil {
		accepted = len(push.Reserves)
	}

	ack := protocol.WeeklyReserveListAck{Accepted: accepted, OK: true}
	out, err := protocol.NewEnvelope(protocol.TypeWeeklyReserveListAck, env.RequestID, ack)
	if err != nil {
		return err
	}
	if err := client.SendEnvelope(out); err != nil {
		return err
	}
	_ = h.Hub.DeliverReply(client.ClinicID, env)
	return nil
}

// handleWaitingQueueListPush صف انتظار را در کش می‌نویسد (نه دیتابیس) و ACK می‌فرستد.
// ورودی: کلاینت مرکز، envelope، payload پوش صف.
// خروجی: خطا در صورت شکست نوشتن کش یا ارسال ACK.
func (h *Handlers) handleWaitingQueueListPush(client *ClientConn, env *protocol.Envelope, push *protocol.WaitingQueueListPush) error {
	accepted := 0
	if h.WaitingQueues != nil {
		accepted = h.WaitingQueues.ReplaceFromPush(client.ClinicID, push)
	} else if push != nil {
		for _, doc := range push.Doctors {
			accepted += len(doc.Patients)
		}
	}

	ack := protocol.WaitingQueueListAck{Accepted: accepted, OK: true}
	out, err := protocol.NewEnvelope(protocol.TypeWaitingQueueListAck, env.RequestID, ack)
	if err != nil {
		return err
	}
	if err := client.SendEnvelope(out); err != nil {
		return err
	}
	_ = h.Hub.DeliverReply(client.ClinicID, env)
	return nil
}
