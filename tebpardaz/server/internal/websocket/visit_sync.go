package websocket

import (
	"log"
	"strings"
	"time"

	"tebpardaz/server/internal/visitcheck"
	"tebpardaz/shared/protocol"
)

// runVisitSync loads unchecked past appointments for one clinic and stores the admission counts.
// Inputs: clinicID of the connected client. Output: none. Overlapping runs for the same clinic are skipped.
func (h *Handlers) runVisitSync(clinicID uint) {
	if h == nil || h.Hub == nil || h.Appointments == nil || clinicID == 0 {
		return
	}
	if !h.beginVisitSync(clinicID) {
		return
	}
	defer h.endVisitSync(clinicID)

	now := time.Now()
	rows, err := h.Appointments.ListUnreviewedVisits(clinicID, visitcheck.StartOfToday(now), visitcheck.BatchLimit())
	if err != nil {
		log.Printf("visit sync clinic %d: %v", clinicID, err)
		return
	}
	items := make([]protocol.VisitCheckItem, 0, len(rows))
	for i := range rows {
		item, err := visitcheck.PrepareAppointment(&rows[i], now)
		if err != nil {
			continue
		}
		items = append(items, item)
	}
	if len(items) == 0 {
		return
	}
	resp, err := h.Hub.RequestVisitCheck(clinicID, visitCheckTimeout(len(items)), protocol.VisitCheckRequest{Items: items})
	if err != nil {
		log.Printf("visit sync clinic %d: %v", clinicID, err)
		return
	}
	if err := h.applyVisitResults(resp); err != nil {
		log.Printf("visit sync clinic %d save: %v", clinicID, err)
	}
}

// beginVisitSync marks a clinic as busy so a second daily pass does not overlap the first.
// Inputs: clinicID. Output: false when a pass is already running.
func (h *Handlers) beginVisitSync(clinicID uint) bool {
	h.visitMu.Lock()
	defer h.visitMu.Unlock()
	if h.visitBusy == nil {
		h.visitBusy = make(map[uint]bool)
	}
	if h.visitBusy[clinicID] {
		return false
	}
	h.visitBusy[clinicID] = true
	return true
}

// endVisitSync clears the busy flag for a clinic after the admission pass finishes.
// Inputs: clinicID. Output: none.
func (h *Handlers) endVisitSync(clinicID uint) {
	h.visitMu.Lock()
	defer h.visitMu.Unlock()
	delete(h.visitBusy, clinicID)
}

// applyVisitResults writes visited or absent for each successful count. Rows with a read error stay unchecked.
// Inputs: client response. Output: the first database error, after attempting every result.
func (h *Handlers) applyVisitResults(resp *protocol.VisitCheckResponse) error {
	if h == nil || h.Appointments == nil || resp == nil {
		return nil
	}
	checkedAt := time.Now()
	var first error
	for _, result := range resp.Results {
		if result.Kind != visitcheck.KindAppointment || result.ID == 0 || strings.TrimSpace(result.Error) != "" {
			continue
		}
		status := visitcheck.StatusFromCount(result.Count)
		if err := h.Appointments.MarkVisitStatus(result.ID, status, checkedAt); err != nil && first == nil {
			first = err
		}
	}
	return first
}

// visitCheckTimeout gives the clinic time to read each admission row, capped at two minutes.
// Inputs: number of items. Output: wait duration.
func visitCheckTimeout(n int) time.Duration {
	if n < 1 {
		n = 1
	}
	d := time.Duration(n) * 2 * time.Second
	if d < 20*time.Second {
		return 20 * time.Second
	}
	if d > 2*time.Minute {
		return 2 * time.Minute
	}
	return d
}
