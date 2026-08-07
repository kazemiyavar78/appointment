package booking

// ConflictResolver decides what to do when clinic and server slot state diverge.
// TODO: define strategies for double-book, stale slot, and partial failure.
type ConflictResolver struct{}

// NewConflictResolver constructs a ConflictResolver.
func NewConflictResolver() *ConflictResolver {
	return &ConflictResolver{}
}

// Resolve applies a conflict policy and returns the outcome.
// TODO: implement conflict resolution rules.
func (r *ConflictResolver) Resolve(_ error) error {
	return nil
}
