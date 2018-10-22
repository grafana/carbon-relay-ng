package input

import (
	"net"
	"time"
)

// TimeoutConn automatically applies a read deadline on a conn upon every read
type TimeoutConn struct {
	conn    net.Conn
	timeout time.Duration
}

func NewTimeoutConn(conn net.Conn, timeout time.Duration) TimeoutConn {
	return TimeoutConn{
		conn:    conn,
		timeout: timeout,
	}
}

func (t TimeoutConn) Read(p []byte) (n int, err error) {
	err = t.conn.SetReadDeadline(time.Now().Add(t.timeout))
	if err != nil {
		return 0, err
	}
	return t.conn.Read(p)
}
