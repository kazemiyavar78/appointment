package websocket

// SyncScheduler periodically requests doctor/appointment sync from connected clinics.
// TODO: tick on interval and enqueue sync messages via Hub.
type SyncScheduler struct {
	Hub *Hub
}

// NewSyncScheduler constructs a SyncScheduler.
func NewSyncScheduler(hub *Hub) *SyncScheduler {
	return &SyncScheduler{Hub: hub}
}

// Start begins the scheduling loop.
// TODO: implement ticker + clinic fan-out.
func (s *SyncScheduler) Start() {
	// TODO: implement sync schedule
}
