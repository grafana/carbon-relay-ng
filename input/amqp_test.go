package input

import (
	"os"
	"testing"
	"time"

	"github.com/streadway/amqp"
)

type MockClosable struct {
	closed bool
}

func (m *MockClosable) Close() error {
	m.closed = true
	return nil
}

func getMockConnector() (chan amqp.Delivery, *MockClosable, *MockClosable, amqpConnector) {
	c := make(chan amqp.Delivery)
	mockConn := &MockClosable{}
	mockChan := &MockClosable{}
	return c, mockConn, mockChan, func(a *Amqp) (<-chan amqp.Delivery, error) {
		a.channel = mockChan
		a.conn = mockConn
		return c, nil
	}
}

type mockDispatcher struct {
	dispatchDuration time.Duration
}

func (m *mockDispatcher) Dispatch(buf []byte) {}
func (m *mockDispatcher) IncNumInvalid()      {}

func TestMain(m *testing.M) {
	_shutdownTimeout := shutdownTimeout
	res := m.Run()
	shutdownTimeout = _shutdownTimeout
	os.Exit(res)
}

func TestAmqpSuccessfulShutdown(t *testing.T) {
	dispatcher := mockDispatcher{}
	c, mockConn, mockChan, mockConnector := getMockConnector()
	a := NewAMQP(config, &dispatcher, mockConnector)
	go a.Start()

	dispatcher.dispatchDuration = time.Millisecond

	c <- amqp.Delivery{
		Body: []byte("a.b.c 1 2"),
	}

	shutdownTimeout = time.Second
	res := a.stop()

	if !res {
		t.Fatalf("Expected shutdown to be successful, but it was not")
	}

	if !mockConn.closed || !mockChan.closed {
		t.Fatalf("Expected channel and connection to be closed, but they were not")
	}
}

func TestAmqpFailingShutdown(t *testing.T) {
	dispatcher := mockDispatcher{}
	c, mockConn, mockChan, mockConnector := getMockConnector()
	a := NewAMQP(config, &dispatcher, mockConnector)
	go a.Start()

	dispatcher.dispatchDuration = time.Second * 5

	c <- amqp.Delivery{
		Body: []byte("a.b.c 1 3"),
	}

	shutdownTimeout = time.Millisecond * 10

	// giving the consumer thread 50ms to start
	time.Sleep(time.Millisecond * 50)
	res := a.stop()

	// if the dispatcher takes 5 seconds to process the message we pushed, but the
	// shutdownTimeout is only 10ms, then we should hit the timeout on shutdown
	if !res {
		t.Fatalf("Expected shutdown to be successful, but it was not")
	}

	// even if the shutdown timeout was hit, the conn & chan should still have
	// gotten closed
	if !mockConn.closed || !mockChan.closed {
		t.Fatalf("Expected channel and connection to be closed, but they were not")
	}
}
