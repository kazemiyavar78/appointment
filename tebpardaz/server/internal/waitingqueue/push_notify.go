package waitingqueue

import (
	"log"

	"tebpardaz/server/internal/cache"
	"tebpardaz/server/internal/webpush"
)

// PushNotifier بعد از تازه‌شدن صف مرکز، فقط برای بیمارانی که نفرات جلوترشان کم شده اعلان می‌فرستد.
type PushNotifier struct {
	Subs   *cache.PushSubscriptionCache
	Queues *cache.WaitingQueueCache
	Send   *webpush.Sender
}

// NewPushNotifier سازنده اعلان‌دهنده است.
// ورودی: کش اشتراک، کش صف، فرستنده Web Push.
// خروجی: اشاره‌گر PushNotifier.
func NewPushNotifier(subs *cache.PushSubscriptionCache, queues *cache.WaitingQueueCache, send *webpush.Sender) *PushNotifier {
	return &PushNotifier{Subs: subs, Queues: queues, Send: send}
}

// OnQueueUpdated اشتراک‌های یک مرکز را با صف تازه مقایسه می‌کند و کاهش موقعیت را پوش می‌کند.
// ورودی: شناسه مرکز. خروجی: ندارد. خطای شبکه همان موقعیت را برای تلاش بعدی نگه می‌دارد.
func (n *PushNotifier) OnQueueUpdated(clinicID uint) {
	if n == nil || n.Subs == nil || n.Queues == nil || n.Send == nil || clinicID == 0 {
		return
	}
	patients := n.Subs.ListPatients(clinicID)
	for _, patient := range patients {
		status := n.Queues.LookupPatient(clinicID, patient.NationalID, patient.AdmissionNo)
		jobs := n.Subs.CollectDecreases(clinicID, patient.AdmissionNo, patient.NationalID, status.Found, status.AheadCount)
		for _, job := range jobs {
			drop, err := n.Send.SendDecrease(job)
			if drop {
				n.Subs.RemoveEndpoint(job.ClinicID, job.AdmissionNo, job.NationalID, job.Endpoint)
				continue
			}
			if err != nil {
				log.Printf("waiting-queue push clinic %d admission %d: %v", clinicID, job.AdmissionNo, err)
				continue
			}
			n.Subs.MarkNotified(job)
		}
	}
}
