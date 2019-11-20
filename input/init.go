package input

import (
	"bufio"
	"fmt"
	"io"

	"github.com/graphite-ng/carbon-relay-ng/encoding"
)

type Input interface {
	Name() string
	Format() encoding.FormatName
	Handler() encoding.FormatAdapter
	Start(d Dispatcher) error
	Stop() error
}

type BaseInput struct {
	Dispatcher Dispatcher
	name       string
	handler    encoding.FormatAdapter
}

func (b *BaseInput) Name() string {
	return b.name
}
func (b *BaseInput) Handler() encoding.FormatAdapter {
	return b.handler
}
func (b *BaseInput) Format() encoding.FormatName {
	return b.handler.Kind()
}

func (b *BaseInput) handleReader(r io.Reader, metadata map[string]string) error {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		b.handle(scanner.Bytes(), metadata)
	}
	return scanner.Err()
}

func (b *BaseInput) handle(msg []byte, metadata map[string]string) error {
	if len(msg) == 0 {
		return nil
	}
	d, err := b.handler.Load(msg)
	for key, value := range metadata {
		d.Metadata[key] = value
	}
	if err != nil {
		return fmt.Errorf("error while processing `%s`: %s", string(msg), err)
	}
	b.Dispatcher.Dispatch(d)
	return nil
}

type Dispatcher interface {
	// Dispatch runs data validation and processing
	// implementations must not reuse buf after returning
	Dispatch(dp encoding.Datapoint)
	// IncNumInvalid marks protocol-level decoding failures
	// does not apply to carbon as the protocol is trivial and any parse failure
	// is a message failure (handled in Dispatch)
	IncNumInvalid()
}
