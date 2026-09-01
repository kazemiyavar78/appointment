package websocket

import (
	"encoding/json"
	"errors"
	"sync"
	"time"

	"tebpardaz/shared/protocol"

	"github.com/google/uuid"
)

var (
	// ErrClinicOffline is returned when no WebSocket client is registered for the clinic.
	ErrClinicOffline = errors.New("clinic offline")
	// ErrRequestTimeout is returned when the clinic does not answer in time.
	ErrRequestTimeout = errors.New("clinic request timeout")
)

// Hub tracks connected clinic clients and fans out protocol messages.
type Hub struct {
	mu         sync.RWMutex
	clients    map[uint]*ClientConn
	register   chan *ClientConn
	unregister chan *ClientConn
	pending    map[string]chan *protocol.Envelope
}

// NewHub constructs an empty Hub.
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[uint]*ClientConn),
		register:   make(chan *ClientConn),
		unregister: make(chan *ClientConn),
		pending:    make(map[string]chan *protocol.Envelope),
	}
}

// Run starts the hub event loop (register / unregister).
func (h *Hub) Run() {
	for {
		select {
		case c := <-h.register:
			h.mu.Lock()
			if old, ok := h.clients[c.ClinicID]; ok && old != c {
				_ = old.Close()
			}
			h.clients[c.ClinicID] = c
			h.mu.Unlock()
			// لاگ شروع اتصال مرکز
			LogClinicBehavior(c.ClinicID, ActionConnect, "", "", "مرکز متصل شد", nil)
		case c := <-h.unregister:
			h.mu.Lock()
			if cur, ok := h.clients[c.ClinicID]; ok && cur == c {
				delete(h.clients, c.ClinicID)
				_ = c.Close()
			}
			h.mu.Unlock()
			// لاگ قطع اتصال مرکز
			LogClinicBehavior(c.ClinicID, ActionDisconnect, "", "", "مرکز قطع شد", nil)
		}
	}
}

// Register enqueues a client connection onto the hub.
func (h *Hub) Register(c *ClientConn) {
	h.register <- c
}

// Unregister enqueues removal of a client connection.
func (h *Hub) Unregister(c *ClientConn) {
	h.unregister <- c
}

// OnlineClinicIDs returns IDs of clinics currently connected over WebSocket.
func (h *Hub) OnlineClinicIDs() []uint {
	h.mu.RLock()
	defer h.mu.RUnlock()
	ids := make([]uint, 0, len(h.clients))
	for id := range h.clients {
		ids = append(ids, id)
	}
	return ids
}

// IsOnline reports whether a clinic has an active WebSocket connection.
func (h *Hub) IsOnline(clinicID uint) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	_, ok := h.clients[clinicID]
	return ok
}

// SendToClinic sends a typed protocol payload to one connected clinic.
// Inputs: clinicID, message type, payload.
// Output: false when clinic is offline or marshal/write fails.
func (h *Hub) SendToClinic(clinicID uint, typ protocol.MessageType, payload any) bool {
	env, err := protocol.NewEnvelope(typ, "", payload)
	if err != nil {
		return false
	}
	return h.sendEnvelope(clinicID, env)
}

// RequestDoctorList asks a clinic for its live HIS doctor roster and waits for the push reply.
// Inputs: clinicID, timeout.
// Output: DoctorListPush from the clinic, or ErrClinicOffline / ErrRequestTimeout.
func (h *Hub) RequestDoctorList(clinicID uint, timeout time.Duration) (*protocol.DoctorListPush, error) {
	requestID := uuid.NewString()
	replyCh := make(chan *protocol.Envelope, 1)

	h.mu.Lock()
	h.pending[requestID] = replyCh
	h.mu.Unlock()
	defer func() {
		h.mu.Lock()
		delete(h.pending, requestID)
		h.mu.Unlock()
	}()

	env, err := protocol.NewEnvelope(protocol.TypeDoctorListRequest, requestID, protocol.DoctorListRequest{})
	if err != nil {
		return nil, err
	}
	if !h.sendEnvelope(clinicID, env) {
		return nil, ErrClinicOffline
	}

	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case reply := <-replyCh:
		var push protocol.DoctorListPush
		if err := reply.DecodePayload(&push); err != nil {
			return nil, err
		}
		return &push, nil
	case <-timer.C:
		// لاگ اتمام مهلت رفت‌وبرگشت لیست پزشکان
		LogClinicBehavior(clinicID, ActionTimeout, string(protocol.TypeDoctorListRequest), requestID, "مهلت پاسخ لیست پزشکان تمام شد", ErrRequestTimeout)
		return nil, ErrRequestTimeout
	}
}

// RequestAppointmentList از کلاینت مرکز نوبت/اسلات می‌خواهد و منتظر پوش پاسخ می‌ماند.
// ورودی: clinicID، timeout، payload درخواست (scope=one برای یک پزشک یا scope=all برای همه).
// خروجی: AppointmentListPush از کلاینت، یا ErrClinicOffline / ErrRequestTimeout.
// کاربرد: بروزرسانی درخواستی از سمت سرور؛ داده دریافتی در کش (نه دیتابیس) ذخیره می‌شود.
func (h *Hub) RequestAppointmentList(clinicID uint, timeout time.Duration, req protocol.AppointmentListRequest) (*protocol.AppointmentListPush, error) {
	requestID := uuid.NewString()
	replyCh := make(chan *protocol.Envelope, 1)

	h.mu.Lock()
	h.pending[requestID] = replyCh
	h.mu.Unlock()
	defer func() {
		h.mu.Lock()
		delete(h.pending, requestID)
		h.mu.Unlock()
	}()

	env, err := protocol.NewEnvelope(protocol.TypeAppointmentListRequest, requestID, req)
	if err != nil {
		return nil, err
	}
	if !h.sendEnvelope(clinicID, env) {
		return nil, ErrClinicOffline
	}

	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case reply := <-replyCh:
		var push protocol.AppointmentListPush
		if err := reply.DecodePayload(&push); err != nil {
			return nil, err
		}
		return &push, nil
	case <-timer.C:
		// لاگ اتمام مهلت رفت‌وبرگشت لیست نوبت
		LogClinicBehavior(clinicID, ActionTimeout, string(protocol.TypeAppointmentListRequest), requestID, "مهلت پاسخ لیست نوبت تمام شد", ErrRequestTimeout)
		return nil, ErrRequestTimeout
	}
}

// RequestBookingCreate sends a booking.create frame to the clinic and waits for booking.create.ack.
// Inputs: clinicID, timeout, BookingCreate payload.
// Output: BookingCreateAck or ErrClinicOffline / ErrRequestTimeout / decode error.
func (h *Hub) RequestBookingCreate(clinicID uint, timeout time.Duration, req protocol.BookingCreate) (*protocol.BookingCreateAck, error) {
	requestID := uuid.NewString()
	replyCh := make(chan *protocol.Envelope, 1)

	h.mu.Lock()
	h.pending[requestID] = replyCh
	h.mu.Unlock()
	defer func() {
		h.mu.Lock()
		delete(h.pending, requestID)
		h.mu.Unlock()
	}()

	env, err := protocol.NewEnvelope(protocol.TypeBookingCreate, requestID, req)
	if err != nil {
		return nil, err
	}
	if !h.sendEnvelope(clinicID, env) {
		return nil, ErrClinicOffline
	}

	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case reply := <-replyCh:
		if reply.Error != nil {
			return &protocol.BookingCreateAck{
				IdempotencyKey:    req.IdempotencyKey,
				SiteAppointmentID: req.SiteAppointmentID,
				OK:                false,
				ErrorCode:         string(reply.Error.Code),
				Message:           reply.Error.Message,
			}, nil
		}
		var ack protocol.BookingCreateAck
		if err := reply.DecodePayload(&ack); err != nil {
			return nil, err
		}
		return &ack, nil
	case <-timer.C:
		// لاگ اتمام مهلت رفت‌وبرگشت ایجاد نوبت
		LogClinicBehavior(clinicID, ActionTimeout, string(protocol.TypeBookingCreate), requestID, "مهلت پاسخ ایجاد نوبت تمام شد", ErrRequestTimeout)
		return nil, ErrRequestTimeout
	}
}

// RequestWaitingQueueList از مرکز صف انتظار می‌خواهد و منتظر پوش پاسخ می‌ماند.
// ورودی: clinicID، timeout، requestID از پیش‌ساخته.
// خروجی: WaitingQueueListPush یا ErrClinicOffline / ErrRequestTimeout.
func (h *Hub) RequestWaitingQueueList(clinicID uint, timeout time.Duration, requestID string) (*protocol.WaitingQueueListPush, error) {
	if requestID == "" {
		requestID = uuid.NewString()
	}
	replyCh := make(chan *protocol.Envelope, 1)

	h.mu.Lock()
	h.pending[requestID] = replyCh
	h.mu.Unlock()
	defer func() {
		h.mu.Lock()
		delete(h.pending, requestID)
		h.mu.Unlock()
	}()

	env, err := protocol.NewEnvelope(protocol.TypeWaitingQueueListRequest, requestID, protocol.WaitingQueueListRequest{})
	if err != nil {
		return nil, err
	}
	if !h.sendEnvelope(clinicID, env) {
		return nil, ErrClinicOffline
	}

	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case reply := <-replyCh:
		var push protocol.WaitingQueueListPush
		if err := reply.DecodePayload(&push); err != nil {
			return nil, err
		}
		return &push, nil
	case <-timer.C:
		// لاگ اتمام مهلت رفت‌وبرگشت صف انتظار
		LogClinicBehavior(clinicID, ActionTimeout, string(protocol.TypeWaitingQueueListRequest), requestID, "مهلت پاسخ صف انتظار تمام شد", ErrRequestTimeout)
		return nil, ErrRequestTimeout
	}
}

// RequestTestResult sends test_result.request to the clinic and waits for test_result.response.
// Inputs: clinicID, timeout, admission/password lookup payload.
// Output: TestResultResponse or ErrClinicOffline / ErrRequestTimeout / decode error.
func (h *Hub) RequestTestResult(clinicID uint, timeout time.Duration, req protocol.TestResultRequest) (*protocol.TestResultResponse, error) {
	requestID := uuid.NewString()
	replyCh := make(chan *protocol.Envelope, 1)

	h.mu.Lock()
	h.pending[requestID] = replyCh
	h.mu.Unlock()
	defer func() {
		h.mu.Lock()
		delete(h.pending, requestID)
		h.mu.Unlock()
	}()

	env, err := protocol.NewEnvelope(protocol.TypeTestResultRequest, requestID, req)
	if err != nil {
		return nil, err
	}
	if !h.sendEnvelope(clinicID, env) {
		return nil, ErrClinicOffline
	}

	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case reply := <-replyCh:
		if reply.Error != nil {
			return &protocol.TestResultResponse{
				AdmissionNo: req.AdmissionNo,
				Found:       false,
				Message:     reply.Error.Message,
			}, nil
		}
		var resp protocol.TestResultResponse
		if err := reply.DecodePayload(&resp); err != nil {
			return nil, err
		}
		return &resp, nil
	case <-timer.C:
		// لاگ اتمام مهلت رفت‌وبرگشت جواب آزمایش
		LogClinicBehavior(clinicID, ActionTimeout, string(protocol.TypeTestResultRequest), requestID, "مهلت پاسخ جواب آزمایش تمام شد", ErrRequestTimeout)
		return nil, ErrRequestTimeout
	}
}

// DeliverReply fulfills a pending request-response waiter when RequestID matches.
// Inputs: clinicID (مرکز پاسخ‌دهنده), env (inbound envelope from clinic).
// Output: true when a waiter consumed the envelope.
func (h *Hub) DeliverReply(clinicID uint, env *protocol.Envelope) bool {
	if env == nil || env.RequestID == "" {
		return false
	}
	h.mu.RLock()
	ch, ok := h.pending[env.RequestID]
	h.mu.RUnlock()
	if !ok {
		return false
	}
	select {
	case ch <- env:
		// لاگ تکمیل رفت‌وبرگشت پس از دریافت پاسخ مرکز
		LogClinicBehavior(clinicID, ActionReply, string(env.Type), env.RequestID, "پاسخ مرکز تحویل waiter شد", nil)
		return true
	default:
		return false
	}
}

// sendEnvelope پیام را به مرکز آنلاین می‌فرستد و نتیجه را لاگ می‌کند.
// ورودی: clinicID و envelope پروتکل.
// خروجی: true در صورت صف‌شدن موفق برای ارسال.
func (h *Hub) sendEnvelope(clinicID uint, env *protocol.Envelope) bool {
	raw, err := env.MustMarshal()
	if err != nil {
		LogClinicBehavior(clinicID, ActionError, string(env.Type), env.RequestID, "خطا در marshal پیام خروجی", err)
		return false
	}
	h.mu.RLock()
	c, ok := h.clients[clinicID]
	h.mu.RUnlock()
	if !ok || c == nil {
		LogClinicBehavior(clinicID, ActionError, string(env.Type), env.RequestID, "مرکز آفلاین است", ErrClinicOffline)
		return false
	}
	if !c.Send(raw) {
		LogClinicBehavior(clinicID, ActionError, string(env.Type), env.RequestID, "بافر ارسال پر است", nil)
		return false
	}
	// لاگ ارسال پیام به مرکز
	LogClinicBehavior(clinicID, ActionSend, string(env.Type), env.RequestID, "پیام به مرکز ارسال شد", nil)
	return true
}

// DecodeEnvelope unmarshals raw bytes into a protocol Envelope.
func DecodeEnvelope(raw []byte) (*protocol.Envelope, error) {
	var env protocol.Envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, err
	}
	return &env, nil
}
