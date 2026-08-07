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
}

// NewHandlers سازنده هندلرهای HTTP مربوط به WebSocket است.
// ورودی: hub، ریپوی مراکز، ریپوی پزشکان، کش اسلات‌ها.
// خروجی: اشاره‌گر به Handlers.
func NewHandlers(hub *Hub, clinics *repository.ClinicRepo, doctors *repository.DoctorRepo, slots *cache.SlotCache) *Handlers {
	return &Handlers{Hub: hub, Clinics: clinics, Doctors: doctors, Slots: slots}
}

// ServeWS اتصال را ارتقا می‌دهد و کلاینت مرکز را ثبت می‌کند.
func (h *Handlers) ServeWS(c *gin.Context) {
	clinic, err := h.authenticate(c.Request)
	if err != nil {
		c.String(http.StatusUnauthorized, err.Error())
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
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
		return err
	}
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
	case protocol.TypeBookingCreateAck, protocol.TypeBookingCancelAck:
		_ = h.Hub.DeliverReply(env)
		return nil
	default:
		return nil
	}
}

// handleDoctorListPush پزشکان را upsert کرده و ACK می‌فرستد؛ ExternalID تا تأیید ادمین خالی می‌ماند.
func (h *Handlers) handleDoctorListPush(client *ClientConn, env *protocol.Envelope, push *protocol.DoctorListPush) error {
	if h.Doctors == nil {
		return fmt.Errorf("doctor repo unavailable")
	}
	results, err := h.Doctors.UpsertFromSync(client.ClinicID, push.Doctors)
	if err != nil {
		return err
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
	// بعد از upsert تا لیست ادمین بتواند ردیف‌های سرور را فوراً ببیند
	_ = h.Hub.DeliverReply(env)
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
	_ = h.Hub.DeliverReply(env)
	return nil
}
