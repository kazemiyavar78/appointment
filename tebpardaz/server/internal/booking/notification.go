package booking

// Notifier sends booking confirmations/cancellations to patients.
// TODO: integrate SMS/email providers; keep transport swappable.
type Notifier struct{}

// NewNotifier constructs a Notifier.
func NewNotifier() *Notifier {
	return &Notifier{}
}

// NotifyBookingConfirmed notifies the patient of a confirmed booking.
// TODO: implement notification delivery.
func (n *Notifier) NotifyBookingConfirmed(_ string) error {
	return nil
}
