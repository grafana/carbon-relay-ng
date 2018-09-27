package input

import (
	"io"
	"sync"

	logging "github.com/op/go-logging"
)

var (
	log      = logging.MustGetLogger("input") // for tests. overridden by main
	socketWg sync.WaitGroup
	shutdown = make(chan struct{})
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

func Shutdown() {
	close(shutdown)
	socketWg.Wait()
}
