package input

import (
	"bytes"
	"net"
	"time"
)

func listen(addr string, handler Handler) error {
	// listeners are set up outside of accept* here so they can interrupt startup
	listener, err := listenTcp(addr)
	if err != nil {
		return err
	}

	udp_conn, err := listenUdp(addr)
	if err != nil {
		return err
	}

	go acceptTcp(addr, listener, handler)
	go acceptUdp(addr, udp_conn, handler)

	return nil
}

func listenTcp(addr string) (*net.TCPListener, error) {
	laddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		return nil, err
	}
	return net.ListenTCP("tcp", laddr)
}

func acceptTcp(addr string, listener *net.TCPListener, handler Handler) {
	var err error
	l := listener
	for {
		log.Notice("listening on %v/tcp", addr)

		for {
			// wait for a tcp connection
			c, err := l.AcceptTCP()
			if err != nil {
				log.Error(err.Error())
				break
			}
			log.Debug("listen.go: tcp connection from %v", c.RemoteAddr())
			// handle the connection
			go acceptTcpConn(c, handler)
		}

		log.Notice("error accepting on %v/tcp, closing connection", addr)
		l.Close()

		backoff := time.Duration(time.Second)
		for {
			log.Notice("reopening %v/tcp", addr)

			l, err = listenTcp(addr)
			if err == nil {
				break
			}

			log.Error(err.Error())

			log.Notice("error listening on %v/tcp, retrying after %v", addr, backoff)
			time.Sleep(backoff)

			backoff *= 2
		}
	}
}

func acceptTcpConn(c net.Conn, handler Handler) {
	defer c.Close()
	handler.Handle(c)
}

func listenUdp(addr string) (*net.UDPConn, error) {
	udp_addr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, err
	}
	return net.ListenUDP("udp", udp_addr)
}

func acceptUdp(addr string, udp_conn *net.UDPConn, handler Handler) {
	var err error
	l := udp_conn
	buffer := make([]byte, 65535)
	for {
		log.Notice("listening on %v/udp", addr)

		for {
			// read a packet into buffer
			b, src, err := l.ReadFrom(buffer)
			if err != nil {
				log.Error(err.Error())
				break
			}
			log.Debug("listen.go: udp packet from %v (length: %d)", src, b)
			// handle the packet
			handler.Handle(bytes.NewReader(buffer[:b]))
		}

		log.Notice("error accepting on %v/udp, closing connection", addr)
		l.Close()

		backoff := time.Duration(time.Second)
		for {
			log.Notice("reopening %v/udp", addr)

			l, err = listenUdp(addr)
			if err == nil {
				break
			}

			log.Error(err.Error())

			log.Notice("error listening on %v/udp, retrying after %v", addr, backoff)
			time.Sleep(backoff)

			backoff *= 2
		}
	}
}
