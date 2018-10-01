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
	shutdownTimeout = time.Second * 30               // how long to wait for shutdown
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
	results := make(chan bool)
	for _, s := range stoppables {
		go func() {
			results <- s.stop()
		}()
	}

	complete := make(chan bool)
	go func() {
		count := 0
		success := true
		for res := range results {

			// one or more shutdowns failed
			if !res {
				success = false
			}

			count++
			if count >= len(stoppables) {
				complete <- success
			}
		}
	}()

	select {
	case res := <-complete:
		return res
	case <-time.After(shutdownTimeout):
		return false
	}
}
