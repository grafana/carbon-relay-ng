package input

import "io"

type Plugin interface {
	Name() string
	Start() error
	Stop() bool
}

// Handler is responsible for reading input.
// It should call:
// Dispatcher.IncNumInvalid upon protocol errors
// Dispatcher.Dispatch to process data that's protocol-valid
type Handler interface {
	Kind() string
	Handle(io.Reader) error
}

type Dispatcher interface {
	// Dispatch runs data validation and processing
	// implementations must not reuse buf after returning
	Dispatch(buf []byte)
	// IncNumInvalid marks protocol-level decoding failures
	// does not apply to carbon as the protocol is trivial and any parse failure
	// is a message failure (handled in Dispatch)
	IncNumInvalid()
}
