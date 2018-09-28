package input

import (
	"io"
	"sync"
	"time"

	logging "github.com/op/go-logging"
)

var (
	log             = logging.MustGetLogger("input") // for tests. overridden by main
	socketWg        sync.WaitGroup
	shutdown        = make(chan struct{})
	shutdownTimeout = time.Second * 30 // how long to wait for shutdown
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
func Shutdown() bool {
	close(shutdown)
	shutdownComplete := make(chan struct{})

	go func() {
		socketWg.Wait()
		close(shutdownComplete)
	}()

	select {
	case <-shutdownComplete:
		return true
	case <-time.After(shutdownTimeout):
		return false
	}
}
