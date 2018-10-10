package input

import (
	"os"
	"sync"
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
	delivery := make(chan amqp.Delivery)
	mockConn := &MockClosable{}
	mockChan := &MockClosable{}
	mockConnector := func(a *Amqp) error {
		a.channel = mockChan
		a.conn = mockConn
		a.delivery = delivery
		return nil
	}
	return delivery, mockConn, mockChan, mockConnector
}

type mockDispatcher struct {
	sync.Mutex
	data []byte
}

func (m *mockDispatcher) Dispatch(buf []byte) {
	m.Lock()
	m.data = append(m.data, buf...)
	m.Unlock()
}

func (m *mockDispatcher) String() string {
	m.Lock()
	defer m.Unlock()
	return string(m.data)
}

func (m *mockDispatcher) IncNumInvalid() {}

func TestMain(m *testing.M) {
	res := m.Run()
	os.Exit(res)
}

func TestAmqpSuccessfulShutdown(t *testing.T) {
	dispatcher := mockDispatcher{}
	delivery, mockConn, mockChan, mockConnector := getMockConnector()
	a := NewAMQP(config, &dispatcher, mockConnector)
	go a.Start()

	testContent := "a.b.c 1 2"

	delivery <- amqp.Delivery{
		Body: []byte(testContent),
	}

	results := make(chan bool)
	go func() {
		results <- a.Stop()
	}()
	select {
	case <-time.After(time.Second):
		t.Fatalf("Shutdown timed out after a second")
	case res := <-results:
		if !res {
			t.Fatalf("Expected shutdown to be successful, but it was not")
		}
	}

	if !mockConn.closed || !mockChan.closed {
		t.Fatalf("Expected channel and connection to be closed, but they were not")
	}

	received := dispatcher.String()
	if received != testContent {
		t.Fatalf("Received unexpected content in handler. Expected \"%s\" got \"%s\"", testContent, received)
	}
}
