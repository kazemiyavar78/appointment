package services

import (
	"log"
	"strings"

	"tebpardaz/client/internal/localdb"
	"tebpardaz/shared/protocol"
)

// Sender sends a protocol envelope to the central server over WebSocket.
type Sender interface {
	// SendEnvelope writes one outbound protocol frame.
	// Inputs: env (fully built envelope).
	// Output: network/encode error.
	SendEnvelope(env *protocol.Envelope) error
}

// DoctorService handles doctor roster sync and site-side code updates.
type DoctorService struct {
	Store  *localdb.Store
	Sender Sender
}

// NewDoctorService constructs a DoctorService.
// Inputs: store (local HIS access), sender (WebSocket outbound).
// Output: pointer to DoctorService.
func NewDoctorService(store *localdb.Store, sender Sender) *DoctorService {
	return &DoctorService{Store: store, Sender: sender}
}

// PushDoctorList reads local HIS doctors and pushes them to the central server for
// approved updates (by national_id) and pending cache refresh.
// Inputs: requestID (correlation id for the envelope).
// Output: error from local read or WebSocket send.
func (s *DoctorService) PushDoctorList(requestID string) error {
	rows, err := s.Store.ListDoctors()
	if err != nil {
		return err
	}
	doctors := make([]protocol.DoctorDTO, 0, len(rows))
	for _, row := range rows {
		name := row.Name
		if name == "" {
			name = strings.TrimSpace(row.FirstName + " " + row.LastName)
		}
		doctors = append(doctors, protocol.DoctorDTO{
			ExternalID:     row.ExternalID,
			LocalCode:      row.Code,
			FirstName:      row.FirstName,
			LastName:       row.LastName,
			NationalID:     row.NationalID,
			Mobile:         row.Mobile,
			Name:           name,
			DoctorSystemID: row.DoctorSystemID,
			SpecialtyCode:  row.SpecialtyCode,
			PhotoURL:       row.PhotoURL,
			IsActive:       row.IsActive,
		})
	}
	log.Printf("doctor sync: pushing %d doctor(s) to server (request=%s)", len(doctors), requestID)
	env, err := protocol.NewEnvelope(protocol.TypeDoctorListPush, requestID, protocol.DoctorListPush{
		Doctors: doctors,
	})
	if err != nil {
		return err
	}
	return s.Sender.SendEnvelope(env)
}

// HandleDoctorListAck processes server acknowledgement; ExternalID is written only on approval notify.
func (s *DoctorService) HandleDoctorListAck(ack *protocol.DoctorListAck) error {
	if ack == nil {
		return nil
	}
	log.Printf("doctor sync: server accepted %d doctor(s), pending=%d", ack.Accepted, ack.Pending)
	return nil
}

// UpdateApprovedDoctorCode asks the site to update ExternalID for an approved doctor.
// Inputs: requestID, doctorID (site id), externalID (HIS code).
// Output: error from WebSocket send.
func (s *DoctorService) UpdateApprovedDoctorCode(requestID string, doctorID uint, externalID string) error {
	env, err := protocol.NewEnvelope(protocol.TypeDoctorCodeUpdate, requestID, protocol.DoctorCodeUpdate{
		DoctorID:   doctorID,
		ExternalID: externalID,
	})
	if err != nil {
		return err
	}
	return s.Sender.SendEnvelope(env)
}

// HandleApprovalNotify writes ExternalID to local HIS after server approval when not already set.
// Inputs: notify payload from the server.
// Output: error from local write, if any.
func (s *DoctorService) HandleApprovalNotify(notify *protocol.DoctorApprovalNotify) error {
	if s.Store == nil || notify == nil {
		return nil
	}
	if notify.ExternalID == "" {
		return nil
	}
	if notify.LocalCode > 0 {
		if err := s.Store.SetDoctorExternalIDIfEmpty(notify.LocalCode, notify.ExternalID); err != nil {
			return err
		}
		log.Printf("doctor approved: external_id=%s saved for user_code=%d", notify.ExternalID, notify.LocalCode)
		return nil
	}
	// Fallback: already registered under this ExternalID — no-op.
	code, err := s.Store.FindDoctorByExternalID(notify.ExternalID)
	if err != nil {
		return err
	}
	if code > 0 {
		return nil
	}
	return nil
}
