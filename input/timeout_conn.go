package input

import (
	"net"
	"time"
)

// TimeoutConn automatically applies a read deadline on a conn upon every read
type TimeoutConn struct {
	conn        net.Conn
	readTimeout time.Duration
}

func NewTimeoutConn(conn net.Conn, readTimeout time.Duration) TimeoutConn {
	return TimeoutConn{
		conn:        conn,
		readTimeout: readTimeout,
	}
}

func (t TimeoutConn) Read(p []byte) (n int, err error) {
	if t.readTimeout > 0 {
		err = t.conn.SetReadDeadline(time.Now().Add(t.readTimeout))
		if err != nil {
			return 0, err
		}
	}
	return t.conn.Read(p)
}
