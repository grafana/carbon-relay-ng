package input

import (
	"bufio"
	"fmt"
	"io"

	"github.com/graphite-ng/carbon-relay-ng/formats"
)

type Input interface {
	Name() string
	Format() formats.FormatName
	Handler() formats.FormatHandler
	Start(d Dispatcher) error
	Stop() error
}

type BaseInput struct {
	Dispatcher Dispatcher
	name       string
	handler    formats.FormatHandler
}

func (b *BaseInput) Name() string {
	return b.name
}
func (b *BaseInput) Handler() formats.FormatHandler {
	return b.handler
}
func (b *BaseInput) Format() formats.FormatName {
	return b.handler.Kind()
}

func (b *BaseInput) handleReader(r io.Reader) error {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		b.handle(scanner.Bytes())
	}
	return scanner.Err()
}

func (b *BaseInput) handle(msg []byte) error {
	d, err := b.handler.Process(msg)
	if err != nil {
		return fmt.Errorf("error while processing `%s`: %s", err)
	}
	b.Dispatcher.Dispatch(d)
	return nil
}

type Dispatcher interface {
	// Dispatch runs data validation and processing
	// implementations must not reuse buf after returning
	Dispatch(dp formats.Datapoint)
	// IncNumInvalid marks protocol-level decoding failures
	// does not apply to carbon as the protocol is trivial and any parse failure
	// is a message failure (handled in Dispatch)
	IncNumInvalid()
}
