package event

// Event represent a event
type Event interface {
	// Exchange returns the exchange where event should be push
	Exchange() string
}
