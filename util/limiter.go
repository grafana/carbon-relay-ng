package util

// Limiter limits concurrency
// Users need to call Add() before starting work and Done()
// when the work is done.
// Add() will block if the Limiters limit has already been reached and
// unblock when another thread calls Done()
type Limiter chan struct{}

// NewLimiter creates a limiter with l slots
func NewLimiter(limit int) Limiter {
	return make(chan struct{}, limit)
}

func (l Limiter) Add() {
	l <- struct{}{}
}

func (l Limiter) Done() { <-l }
