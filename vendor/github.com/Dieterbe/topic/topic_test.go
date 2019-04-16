package topic_test

import (
	"github.com/Dieterbe/topic"
	"runtime"
	"testing"
)

func TestAudienceEmpty(t *testing.T) {
	top := topic.New()
	defer close(top.Broadcast)
	top.Broadcast <- "crickets"
}

func TestAudienceOne(t *testing.T) {
	top := topic.New()
	defer close(top.Broadcast)
	consumer := make(chan interface{}, 1)
	top.Register(consumer)

	top.Broadcast <- "hello"
	msg := <-consumer
	got, ok := msg.(string)
	if !ok {
		t.Fatalf("Message is wrong type: %v", msg)
	}
	if got != "hello" {
		t.Errorf("Message has wrong content: %v", got)
	}
}

func TestUnregister(t *testing.T) {
	top := topic.New()
	defer close(top.Broadcast)
	consumer := make(chan interface{}, 1)
	top.Register(consumer)
	top.Unregister(consumer)

	top.Broadcast <- "hello"
	msg, ok := <-consumer
	if ok {
		t.Fatalf("Received after unregistering: %v", msg)
	}
}

func TestClose(t *testing.T) {
	top := topic.New()
	consumer := make(chan interface{}, 1)
	top.Register(consumer)

	close(top.Broadcast)

	for msg := range consumer {
		t.Fatalf("Received after unregistering: %v", msg)
	}
}

func TestSlowConsumerIsClosed(t *testing.T) {
	top := topic.New()
	defer close(top.Broadcast)

	consumer := make(chan interface{}, 1)
	top.Register(consumer)

	top.Broadcast <- "one"
	top.Broadcast <- "two"
	top.Broadcast <- "three"

	// consumer can queue at most one message, run() may be
	// processing at most one at a time; we send three messages to
	// guarantee an overflow, then wait until run() has drained
	// its queue ("three" might still be in processing, but it
	// must have processed "two", and thus triggered slow consumer
	// logic)
	for len(top.Broadcast) > 0 {
		runtime.Gosched()
	}

	one := <-consumer
	two := <-consumer
	three, ok := <-consumer
	if ok {
		t.Fatalf("Should have been closed as too slow already: %v %v %v", one, two, three)
	}

	// but worst case the consumer was just about to unregister then
	top.Unregister(consumer)
}
