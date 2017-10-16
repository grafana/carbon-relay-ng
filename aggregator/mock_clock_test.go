package aggregator

import (
	"sync/atomic"
	"time"
)

// mockTickr is a mock ticker,
// which exactly on the interval
// it needs to be told about time advancement
type mockTick struct {
	C        chan time.Time
	last     int64
	interval int64
}

func NewMockTick(interval int64) *mockTick {
	return &mockTick{
		C:        make(chan time.Time),
		interval: interval,
	}
}

func (m *mockTick) Set(t int64) {
	// already ticked recently enough?
	if (t - m.last) < m.interval {
		return
	}
	// first time this ticker runs, let it tick once.
	if m.last == 0 {
		m.last = t - m.interval
	}
	// note that this will tick in very quick succession
	// and if multiple, ticks are likely to be dropped
	// for our unit tests this is ok as we'll only tick
	// once each time.
	for ; m.last <= t; m.last += m.interval {
		select {
		case m.C <- time.Unix(m.last, 0):
		default:
		}
	}
}

// mockClock simple fake clock with some tickchans
// (we can't provide actual time.Ticker's because they're not
// an interface, and if we use a concrete one, we can't write
// to their C channel.
type mockClock struct {
	now   int64
	ticks []*mockTick
}

func NewMockClock(t int64) *mockClock {
	m := &mockClock{}
	atomic.StoreInt64(&m.now, t)
	return m
}

func (m *mockClock) Set(t int64) {
	atomic.StoreInt64(&m.now, t)
	for _, tick := range m.ticks {
		tick.Set(t)
	}
}

func (m *mockClock) Now() time.Time {
	return time.Unix(atomic.LoadInt64(&m.now), 0)
}

func (m *mockClock) AddTick(mt *mockTick) {
	m.ticks = append(m.ticks, mt)
}
