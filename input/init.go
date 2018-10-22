package input

import "io"

type Plugin interface {
	Name() string
	Start() error
	Stop() bool
}

type Handler interface {
	// Handle reads input of the network/socket, it calls:
	// Dispatcher.IncNumInvalid upon protocol errors
	// Dispatcher.Dispatch to process data that's protocol-valid
	Handle(io.Reader)
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
