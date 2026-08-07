package protocol

// DoctorListRequest asks the clinic client to push its local doctor roster.
// Direction: server → client (optional; client may also push proactively).
type DoctorListRequest struct {
	SinceUnix int64 `json:"since_unix,omitempty"`
}

// DoctorDTO is one doctor row synced from the clinic HIS.
type DoctorDTO struct {
	ExternalID     string `json:"external_id,omitempty"`
	LocalCode      int    `json:"local_code,omitempty"`
	NationalID     string `json:"national_id,omitempty"`
	FirstName      string `json:"first_name,omitempty"`
	LastName       string `json:"last_name,omitempty"`
	Name           string `json:"name"`
	Mobile         string `json:"mobile,omitempty"`
	DoctorSystemID int    `json:"doctor_system_id,omitempty"`
	SpecialtyCode  string `json:"specialty_code,omitempty"`
	PhotoURL       string `json:"photo_url,omitempty"`
	IsActive       bool   `json:"is_active"`
}

// DoctorListPush carries doctor records from the clinic client to the server.
type DoctorListPush struct {
	Doctors []DoctorDTO `json:"doctors"`
}

// DoctorSyncResult maps one HIS doctor to the server doctor record (ExternalID sent only after approval).
type DoctorSyncResult struct {
	LocalCode  int    `json:"local_code"`
	ExternalID string `json:"external_id,omitempty"`
	DoctorID   uint   `json:"doctor_id"`
}

// DoctorListAck acknowledges receipt of a doctor list push.
type DoctorListAck struct {
	Accepted int                `json:"accepted"`
	Pending  int                `json:"pending"`
	Results  []DoctorSyncResult `json:"results,omitempty"`
}

// DoctorCodeUpdate asks the central site to set/update ExternalID for an approved doctor.
type DoctorCodeUpdate struct {
	DoctorID   uint   `json:"doctor_id"`
	ExternalID string `json:"external_id"`
}

// DoctorCodeUpdateAck confirms the site-side doctor code update.
type DoctorCodeUpdateAck struct {
	DoctorID   uint   `json:"doctor_id"`
	ExternalID string `json:"external_id"`
	OK         bool   `json:"ok"`
}

// DoctorApprovalNotify informs the client that a doctor was approved or rejected on the site.
type DoctorApprovalNotify struct {
	DoctorID   uint   `json:"doctor_id"`
	LocalCode  int    `json:"local_code,omitempty"`
	ExternalID string `json:"external_id"`
	IsApproved bool   `json:"is_approved"`
	Status     string `json:"status"`
}
