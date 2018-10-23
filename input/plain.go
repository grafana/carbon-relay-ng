package input

import (
	"bufio"
	"io"
	"net"
	"time"

	log "github.com/sirupsen/logrus"
)

type Plain struct {
	dispatcher Dispatcher
}

func NewPlain(addr string, readTimeout time.Duration, dispatcher Dispatcher) *Listener {
	return NewListener("plain", addr, readTimeout, &Plain{dispatcher})
}

func (p *Plain) HandleData(c io.Reader) {
	err := p.Handle(c)
	if err != nil {
		log.Warnf("plain handler: %s", err)
		return
	}
	log.Debug("plain handler finished")
}

func (p *Plain) HandleConn(c net.Conn) {
	log.Debugf("plain handler: new tcp connection from %v", c.RemoteAddr())
	err := p.Handle(c)

	var remoteInfo string

	rAddr := c.RemoteAddr()
	if rAddr != nil {
		remoteInfo = " for " + rAddr.String()
	}
	if err != nil {
		log.Warnf("plain handler%s returned: %s. closing conn", remoteInfo, err)
		return
	}
	log.Debugf("plain handler%s returned. closing conn", remoteInfo)
}

// Handle is the "core" processing function.
// It is exported such that 3rd party embedders can reuse it.
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
