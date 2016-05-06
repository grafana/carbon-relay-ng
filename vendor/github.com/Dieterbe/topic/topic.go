// Package topic provides a publish-subscribe mechanism where
// consumers may disappear at any time.
//
// Each Topic is exactly that, a single topic. There is no filtering
// of messages or such functionality on this level. For example, you
// could create a new Topic for each chat room on your chat server,
// with every browser instance using an WebSockets API to Register
// on every room the user wants to follow.
package topic

import (
	"sync"
)

type nothing struct{}

// Topic is a pub-sub mechanism where consumers can Register to
// receive messages sent to Broadcast.
type Topic struct {
	// Producer sends messages on this channel. Close the channel
	// to shutdown the topic.
	Broadcast chan<- interface{}

	lock        sync.Mutex
	connections map[chan<- interface{}]nothing
}

// New creates a new topic. Messages can be broadcast on this topic,
// and registered consumers are guaranteed to either receive them, or
// see a channel close.
func New() *Topic {
	t := &Topic{}
	broadcast := make(chan interface{}, 100)
	t.Broadcast = broadcast
	t.connections = make(map[chan<- interface{}]nothing)
	go t.run(broadcast)
	return t
}

func (t *Topic) run(broadcast <-chan interface{}) {
	for msg := range broadcast {
		func() {
			t.lock.Lock()
			defer t.lock.Unlock()
			for ch, _ := range t.connections {
				ch <- msg
			}
		}()
	}

	t.lock.Lock()
	defer t.lock.Unlock()
	for ch, _ := range t.connections {
		delete(t.connections, ch)
		close(ch)
	}
}

// Register starts receiving messages on the given channel. If a
// channel close is seen, either the topic has been shut down, or the
// consumer was too slow, and should re-register.
func (t *Topic) Register(ch chan<- interface{}) {
	t.lock.Lock()
	defer t.lock.Unlock()
	t.connections[ch] = nothing{}
}

// Unregister stops receiving messages on this channel.
func (t *Topic) Unregister(ch chan<- interface{}) {
	t.lock.Lock()
	defer t.lock.Unlock()

	// double-close is not safe, so make sure we didn't already
	// drop this consumer as too slow
	_, ok := t.connections[ch]
	if ok {
		delete(t.connections, ch)
		close(ch)
	}
}
