package input

import (
	"bytes"
	"net"
	"sync"
	"time"

	"github.com/jpillora/backoff"
)

type Listener struct {
	wg       sync.WaitGroup
	tcpList  *net.TCPListener
	udpConn  *net.UDPConn
	handler  Handler
	shutdown chan struct{}
}

func NewListener(handler Handler) *Listener {
	return &Listener{
		handler:  handler,
		shutdown: make(chan struct{}),
	}
}

func (l *Listener) listen(addr string) error {
	// listeners are set up outside of accept* here so they can interrupt startup
	err := l.listenTcp(addr)
	if err != nil {
		return err
	}

	err = l.listenUdp(addr)
	if err != nil {
		return err
	}

	go l.acceptTcp(addr)
	go l.acceptUdp(addr)

	return nil
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
	var err error
	backoffCounter := &backoff.Backoff{
		Min: 500 * time.Millisecond,
		Max: time.Minute,
	}

	l.wg.Add(1)
	defer l.wg.Done()

	go func() {
		<-l.shutdown
		l.tcpList.Close()
	}()

	for {
		log.Notice("listening on %v/tcp", addr)

	ACCEPT:
		for {
			c, err := l.tcpList.AcceptTCP()
			if err != nil {
				select {
				// socket has been closed due to shut down, log and return
				case <-l.shutdown:
					log.Info("shutting down %v/tcp, closing socket", addr)
					return
				default:
					log.Error("error accepting on %v/tcp, closing connection: %s", addr, err)
					l.tcpList.Close()
					break ACCEPT
				}
			}

			// handle the connection
			log.Debug("listen.go: tcp connection from %v", c.RemoteAddr())
			go l.acceptTcpConn(c)
		}

		for {
			log.Notice("reopening %v/tcp", addr)
			err = l.listenTcp(addr)
			if err == nil {
				backoffCounter.Reset()
				break
			}

			backoffDuration := backoffCounter.Duration()
			log.Error("error listening on %v/tcp, retrying after %v: %s", addr, backoffDuration, err)

			// continue looping once the backoff duration is over,
			// unless a shutdown event gets triggered first
			select {
			case <-l.shutdown:
				log.Info("shutting down %v/tcp, closing socket", addr)
				return
			case <-time.After(backoffDuration):
			}
		}
	}
}

func (l *Listener) acceptTcpConn(c net.Conn) {
	l.wg.Add(1)
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

func (l *Listener) acceptUdp(addr string) {
	var err error
	buffer := make([]byte, 65535)
	backoffCounter := &backoff.Backoff{
		Min: 500 * time.Millisecond,
		Max: time.Minute,
	}

	l.wg.Add(1)
	defer l.wg.Done()

	go func() {
		<-l.shutdown
		l.udpConn.Close()
	}()

	for {
		log.Notice("listening on %v/udp", addr)

	READ:
		for {
			// read a packet into buffer
			b, src, err := l.udpConn.ReadFrom(buffer)
			if err != nil {
				select {
				// socket has been closed due to shut down, log and return
				case <-l.shutdown:
					log.Info("shutting down %v/udp, closing socket", addr)
					return
				default:
					log.Error("error reading packet on %v/udp, closing connection: %s", addr, err)
					l.udpConn.Close()
					break READ
				}
			}

			// handle the packet
			log.Debug("listen.go: udp packet from %v (length: %d)", src, b)
			l.handler.Handle(bytes.NewReader(buffer[:b]))
		}

		for {
			log.Notice("reopening %v/udp", addr)

			err = l.listenUdp(addr)
			if err == nil {
				backoffCounter.Reset()
				break
			}

			backoffDuration := backoffCounter.Duration()

			log.Error("error listening on %v/udp, retrying after %v: %s", addr, backoffDuration, err)

			// continue looping once the backoff duration is over,
			// unless a shutdown event gets triggered first
			select {
			case <-l.shutdown:
				log.Info("shutting down %v/udp, closing socket", addr)
				return
			case <-time.After(backoffDuration):
			}
		}
	}
}

// returns true if the shutdown was clean, otherwise false
func (l *Listener) Shutdown() bool {
	close(l.shutdown)
	shutdownComplete := make(chan struct{})

	go func() {
		l.wg.Wait()
		close(shutdownComplete)
	}()

	select {
	case <-shutdownComplete:
		return true
	case <-time.After(shutdownTimeout):
		return false
	}
}
