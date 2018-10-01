package input

import (
	"bytes"
	"net"
	"time"

	"github.com/jpillora/backoff"
)

type Listener struct {
	workerPool
	tcpList *net.TCPListener
	udpConn *net.UDPConn
	handler Handler
}

func NewListener(handler Handler) *Listener {
	return &Listener{
		handler: handler,
	}
}

func (l *Listener) listen(addr string) error {
	l.startStoppable()

	// listeners are set up outside of accept* here so they can interrupt startup
	err := l.listenTcp(addr)
	if err != nil {
		return err
	}

	err = l.listenUdp(addr)
	if err != nil {
		return err
	}

	l.wg.Add(2)
	go l.accept(addr, "tcp")
	go l.accept(addr, "udp")

	return nil
}

func (l *Listener) accept(addr, proto string) {
	defer l.wg.Done()

	var consume func(string)
	var reconnect func(string) error
	var listener Closable

	switch proto {
	case "tcp":
		consume = l.acceptTcp
		reconnect = l.listenTcp
		listener = l.tcpList
	case "udp":
		consume = l.consumeUdp
		reconnect = l.listenUdp
		listener = l.udpConn
	default:
		return
	}

	backoffCounter := &backoff.Backoff{
		Min: 500 * time.Millisecond,
		Max: time.Minute,
	}

	go func() {
		<-l.shutdown
		log.Info("shutting down %v/%s, closing socket", addr, proto)
		listener.Close()
	}()

	for {
		log.Notice("listening on %v/%s", addr, proto)

		consume(addr)

		select {
		case <-l.shutdown:
			return
		default:
			for {
				log.Notice("reopening %v/%s", addr, proto)
				err := reconnect(addr)
				if err == nil {
					backoffCounter.Reset()
					break
				}

				backoffDuration := backoffCounter.Duration()
				log.Error("error listening on %v/%s, retrying after %v: %s", addr, proto, backoffDuration, err)
				if !l.backoffOrShutdown(backoffDuration) {
					log.Info("shutting down %v/%s, closing socket", addr, proto)
					return
				}
			}
		}
	}
}

func (l *Listener) listenTcp(addr string) error {
	laddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		return err
	}
	l.tcpList, err = net.ListenTCP("tcp", laddr)
	if err != nil {
		return err
	}
	return nil
}

func (l *Listener) acceptTcp(addr string) {
	for {
		c, err := l.tcpList.AcceptTCP()
		if err != nil {
			select {
			// socket has been closed due to shut down, log and return
			case <-l.shutdown:
				return
			default:
				log.Error("error accepting on %v/tcp, closing connection: %s", addr, err)
				l.tcpList.Close()
				return
			}
		}

		// handle the connection
		log.Debug("listen.go: tcp connection from %v", c.RemoteAddr())
		l.wg.Add(1)
		go l.acceptTcpConn(c)
	}
}

func (l *Listener) reconnectTcp(addr string) {
	backoffCounter := &backoff.Backoff{
		Min: 500 * time.Millisecond,
		Max: time.Minute,
	}

	for {
		log.Notice("reopening %v/tcp", addr)
		err := l.listenTcp(addr)
		if err == nil {
			backoffCounter.Reset()
			break
		}

		backoffDuration := backoffCounter.Duration()
		log.Error("error listening on %v/tcp, retrying after %v: %s", addr, backoffDuration, err)
		if !l.backoffOrShutdown(backoffDuration) {
			log.Info("shutting down %v/tcp, closing socket", addr)
			return
		}
	}
}

func (l *Listener) acceptTcpConn(c net.Conn) {
	defer l.wg.Done()

	go func() {
		<-l.shutdown
		c.Close()
	}()

	l.handler.Handle(c)
}

func (l *Listener) listenUdp(addr string) error {
	udp_addr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return err
	}
	l.udpConn, err = net.ListenUDP("udp", udp_addr)
	if err != nil {
		return err
	}
	return nil
}

func (l *Listener) consumeUdp(addr string) {
	buffer := make([]byte, 65535)
	for {
		// read a packet into buffer
		b, src, err := l.udpConn.ReadFrom(buffer)
		if err != nil {
			select {
			// socket has been closed due to shut down, log and return
			case <-l.shutdown:
				return
			default:
				log.Error("error reading packet on %v/udp, closing connection: %s", addr, err)
				l.udpConn.Close()
				return
			}
		}

		// handle the packet
		log.Debug("listen.go: udp packet from %v (length: %d)", src, b)
		l.handler.Handle(bytes.NewReader(buffer[:b]))
	}
}

// returns true if backoff expired, or false for shutdown
func (l *Listener) backoffOrShutdown(d time.Duration) bool {
	select {
	case <-l.shutdown:
		return false
	case <-time.After(d):
		return true
	}
}
