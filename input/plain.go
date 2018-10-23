package input

import (
	"bufio"
	"io"

	log "github.com/sirupsen/logrus"
)

type Plain struct {
	dispatcher Dispatcher
}

func NewPlain(dispatcher Dispatcher) *Plain {
	return &Plain{dispatcher}
}

func (p *Plain) Kind() string {
	return "plain"
}

func (p *Plain) Handle(c io.Reader) error {
	scanner := bufio.NewScanner(c)
	for scanner.Scan() {
		// Note that everything in this loop should proceed as fast as it can
		// so we're not blocked and can keep processing
		// so the validation, the pipeline initiated via dispatcher.Dispatch(), etc
		// must never block.

		buf := scanner.Bytes()
		log.Tracef("plain.go: Received Line: %q", buf)

		p.dispatcher.Dispatch(buf)
	}
	return scanner.Err()
}
