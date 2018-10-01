package input

import (
	"sync"
	"time"
)

type workerPool struct {
	wg       sync.WaitGroup
	shutdown chan struct{}
}

// registers this instance in the list of stoppables
// also initializes the shutdown channel
func (w *workerPool) startStoppable() {
	stoppables = append(stoppables, w)
	w.shutdown = make(chan struct{})
}

func (w *workerPool) stop() bool {
	close(w.shutdown)
	shutdownComplete := make(chan struct{})

	go func() {
		w.wg.Wait()
		close(shutdownComplete)
	}()

	select {
	case <-shutdownComplete:
		return true
	case <-time.After(shutdownTimeout):
		return false
	}
}
