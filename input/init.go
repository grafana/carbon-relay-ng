package input

import (
	"io"
	"time"

	logging "github.com/op/go-logging"
)

type Stoppable interface {
	stop() bool
}

var (
	log             = logging.MustGetLogger("input") // for tests. overridden by main
	shutdown        chan struct{}
	shutdownTimeout = time.Second * 30 // how long to wait for shutdown
	stoppables      []Stoppable
)

func SetLogger(l *logging.Logger) {
	log = l
}

type Handler interface {
	Handle(io.Reader)
}

type Dispatcher interface {
	Dispatch(buf []byte)
	IncNumInvalid()
}

// returns true if the shutdown was clean, otherwise false
func Stop() bool {
	success := true
	for _, s := range stoppables {
		if !s.stop() {
			success = false
		}
	}

	return success
}
