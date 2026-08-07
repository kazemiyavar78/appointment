package sync

import "tebpardaz/client/internal/services"

// DoctorSync is a thin wrapper around services.DoctorService for backwards-friendly naming.
type DoctorSync struct {
	Service *services.DoctorService
}

// NewDoctorSync constructs a DoctorSync worker.
// Inputs: service (doctor WS/HIS orchestration).
// Output: pointer to DoctorSync.
func NewDoctorSync(service *services.DoctorService) *DoctorSync {
	return &DoctorSync{Service: service}
}

// RunOnce performs a single doctor list push cycle.
// Inputs: none (uses a fixed request id prefix).
// Output: error from DoctorService.PushDoctorList.
func (s *DoctorSync) RunOnce() error {
	if s.Service == nil {
		return nil
	}
	return s.Service.PushDoctorList("doctor-sync")
}
