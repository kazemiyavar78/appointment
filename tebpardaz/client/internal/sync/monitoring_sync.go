package sync

import "tebpardaz/client/internal/services"

// MonitoringSync پوشش نازک دور سرویس صف انتظار برای سینک دوره‌ای است.
type MonitoringSync struct {
	Service *services.MonitoringService
}

// NewMonitoringSync سازنده worker سینک صف انتظار است.
// ورودی: service هماهنگی صف انتظار.
// خروجی: اشاره‌گر به MonitoringSync.
func NewMonitoringSync(service *services.MonitoringService) *MonitoringSync {
	return &MonitoringSync{Service: service}
}

// RunOnce یک چرخه سینک صف انتظار را اجرا می‌کند.
// ورودی: ندارد.
// خروجی: خطا از PushWaitingQueue.
func (s *MonitoringSync) RunOnce() error {
	if s == nil || s.Service == nil {
		return nil
	}
	return s.Service.PushWaitingQueue("monitoring-sync")
}
