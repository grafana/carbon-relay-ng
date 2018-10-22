package input

import (
	"bufio"
	"io"
	"time"

	log "github.com/sirupsen/logrus"
)

type Plain struct {
	dispatcher Dispatcher
}

func NewPlain(addr string, readTimeout time.Duration, dispatcher Dispatcher) *Listener {
	return NewListener("plain", addr, readTimeout, &Plain{dispatcher})
}

func (p *Plain) Handle(c io.Reader) {
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
	if err := scanner.Err(); err != nil {
		log.Error(err.Error())
	}
}
