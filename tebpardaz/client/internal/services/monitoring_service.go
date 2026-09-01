package services

import (
	"tebpardaz/client/internal/localdb"
	"tebpardaz/shared/protocol"
)

// MonitoringService صف انتظار بیماران را از HIS محلی به سرور مرکزی پوش می‌کند.
type MonitoringService struct {
	Store  *localdb.Store
	Sender Sender
}

// NewMonitoringService سازنده MonitoringService است.
// ورودی: store (دسترسی HIS)، sender (خروجی WebSocket).
// خروجی: اشاره‌گر به MonitoringService.
func NewMonitoringService(store *localdb.Store, sender Sender) *MonitoringService {
	return &MonitoringService{Store: store, Sender: sender}
}

// PushWaitingQueue صف انتظار را از HIS می‌خواند و به سرور پوش می‌کند.
// ورودی: requestID برای همبستگی با درخواست سرور.
// خروجی: خطا از خواندن محلی یا ارسال WebSocket.
func (s *MonitoringService) PushWaitingQueue(requestID string) error {
	if s == nil || s.Store == nil || s.Sender == nil {
		return nil
	}
	groups, err := s.Store.GetWaitePatientPezeshkListSite()
	if err != nil {
		return err
	}

	doctors := make([]protocol.WaitingQueueDoctorDTO, 0, len(groups))
	accepted := 0
	for _, group := range groups {
		patients := make([]protocol.WaitingQueuePatientDTO, 0, len(group.ListPatient))
		for _, p := range group.ListPatient {
			patients = append(patients, protocol.WaitingQueuePatientDTO{
				PatientName:       p.PatientName,
				PatientCode:       p.PatientCode,
				NationalID:        p.PatientCmeli,
				VisitTime:         p.VisitTime,
				DoctorCode:        group.DoctorCode,
				DoctorName:        p.DoctorName,
				ReceptionUserName: p.ReceptionUserName,
			})
			accepted++
		}
		doctors = append(doctors, protocol.WaitingQueueDoctorDTO{
			DoctorName: group.DoctorName,
			DoctorCode: group.DoctorCode,
			Patients:   patients,
		})
	}

	env, err := protocol.NewEnvelope(protocol.TypeWaitingQueueListPush, requestID, protocol.WaitingQueueListPush{
		Doctors: doctors,
	})
	if err != nil {
		return err
	}
	_ = accepted
	return s.Sender.SendEnvelope(env)
}

// HandleListRequest به waiting_queue.list.request سمت سرور پاسخ می‌دهد.
// ورودی: requestID، req (بدون فیلد).
// خروجی: خطا از PushWaitingQueue.
func (s *MonitoringService) HandleListRequest(requestID string, req *protocol.WaitingQueueListRequest) error {
	_ = req
	return s.PushWaitingQueue(requestID)
}
