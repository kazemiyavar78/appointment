package wsclient

import (
	"encoding/json"

	"tebpardaz/client/internal/services"
	"tebpardaz/shared/protocol"
)

// Handlers dispatches inbound protocol messages from the central server.
type Handlers struct {
	Doctors        *services.DoctorService
	Appointments   *services.AppointmentService
	Bookings       *services.BookingService
	TestResults    *services.TestResultService
	WeeklyReserves *services.WeeklyReserveService
	Monitoring     *services.MonitoringService
	Sender         services.Sender
}

// NewHandlers constructs WebSocket message handlers with the given services.
// Inputs: doctors, appointments, bookings, test-results, weekly-reserves, monitoring services and a Sender for replies.
// Output: pointer to Handlers.
func NewHandlers(
	doctors *services.DoctorService,
	appointments *services.AppointmentService,
	bookings *services.BookingService,
	testResults *services.TestResultService,
	weeklyReserves *services.WeeklyReserveService,
	monitoring *services.MonitoringService,
	sender services.Sender,
) *Handlers {
	return &Handlers{
		Doctors:        doctors,
		Appointments:   appointments,
		Bookings:       bookings,
		TestResults:    testResults,
		WeeklyReserves: weeklyReserves,
		Monitoring:     monitoring,
		Sender:         sender,
	}
}

// HandleMessage decodes one inbound protocol frame and routes it by MessageType.
// Inputs: raw JSON bytes of a protocol.Envelope.
// Output: routing/handler error.
func (h *Handlers) HandleMessage(raw []byte) error {
	var env protocol.Envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return err
	}
	switch env.Type {
	case protocol.TypePing:
		return h.replyPong(env.RequestID)
	case protocol.TypeDoctorListRequest:
		if h.Doctors == nil {
			return nil
		}
		return h.Doctors.PushDoctorList(env.RequestID)
	case protocol.TypeDoctorApprovalNotify:
		var notify protocol.DoctorApprovalNotify
		if err := env.DecodePayload(&notify); err != nil {
			return err
		}
		if h.Doctors == nil {
			return nil
		}
		return h.Doctors.HandleApprovalNotify(&notify)
	// سرور از ادمین «دریافت لیست زنده» را زده → نوبت‌ها را از HIS بخوان و push کن
	case protocol.TypeAppointmentListRequest:
		var req protocol.AppointmentListRequest
		if err := env.DecodePayload(&req); err != nil {
			return err
		}
		if h.Appointments == nil {
			// اگر در main.go سرویس nil باشد، درخواست بی‌صدا نادیده گرفته می‌شود
			return nil
		}
		return h.Appointments.HandleListRequest(env.RequestID, &req)
	case protocol.TypeBookingCreate:
		var booking protocol.BookingCreate
		if err := env.DecodePayload(&booking); err != nil {
			return err
		}
		if h.Bookings == nil {
			return nil
		}
		return h.Bookings.HandleCreate(env.RequestID, &booking)
	case protocol.TypeBookingCancel:
		var cancel protocol.BookingCancel
		if err := env.DecodePayload(&cancel); err != nil {
			return err
		}
		if h.Bookings == nil {
			return nil
		}
		return h.Bookings.HandleCancel(env.RequestID, &cancel)
	case protocol.TypeTestResultRequest:
		var req protocol.TestResultRequest
		if err := env.DecodePayload(&req); err != nil {
			return err
		}
		if h.TestResults == nil {
			return nil
		}
		return h.TestResults.HandleRequest(env.RequestID, &req)
	// سرور درخواست بروزرسانی لیست نوبت هفتگی کرده → از HIS بخوان و push کن
	case protocol.TypeWeeklyReserveListRequest:
		var req protocol.WeeklyReserveListRequest
		if err := env.DecodePayload(&req); err != nil {
			// payload خالی هم معتبر است
			req = protocol.WeeklyReserveListRequest{}
		}
		if h.WeeklyReserves == nil {
			return nil
		}
		return h.WeeklyReserves.HandleListRequest(env.RequestID, &req)
	// سرور درخواست بروزرسانی صف انتظار کرده → از HIS بخوان و push کن
	case protocol.TypeWaitingQueueListRequest:
		var req protocol.WaitingQueueListRequest
		if err := env.DecodePayload(&req); err != nil {
			req = protocol.WaitingQueueListRequest{}
		}
		if h.Monitoring == nil {
			return nil
		}
		return h.Monitoring.HandleListRequest(env.RequestID, &req)
	case protocol.TypeDoctorListAck:
		var ack protocol.DoctorListAck
		if err := env.DecodePayload(&ack); err != nil {
			return err
		}
		if h.Doctors == nil {
			return nil
		}
		return h.Doctors.HandleDoctorListAck(&ack)
	case protocol.TypeDoctorCodeUpdateAck, protocol.TypeAppointmentListAck, protocol.TypeWeeklyReserveListAck, protocol.TypeWaitingQueueListAck, protocol.TypePong:
		return nil
	default:
		return nil
	}
}

// replyPong sends a pong envelope for an inbound ping.
func (h *Handlers) replyPong(requestID string) error {
	if h.Sender == nil {
		return nil
	}
	env, err := protocol.NewEnvelope(protocol.TypePong, requestID, nil)
	if err != nil {
		return err
	}
	return h.Sender.SendEnvelope(env)
}
