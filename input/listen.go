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
	name     string
	addr     string
	tcpList  *net.TCPListener
	udpConn  *net.UDPConn
	handler  Handler
	shutdown chan struct{}
}

func NewListener(name, addr string, handler Handler) *Listener {
	return &Listener{
		name:     name,
		addr:     addr,
		handler:  handler,
		shutdown: make(chan struct{}),
	}
}

func (l *Listener) Start() error {
	// listeners are set up outside of accept* here so they can interrupt startup
	err := l.listenTcp()
	if err != nil {
		return err
	}

	err = l.listenUdp()
	if err != nil {
		return err
	}

	l.wg.Add(2)
	go l.run("tcp", l.acceptTcp, l.listenTcp, l.tcpList)
	go l.run("udp", l.consumeUdp, l.listenUdp, l.udpConn)

	return nil
}

func (l *Listener) run(proto string, consume func(), reconnect func() error, listener Closable) {
	defer l.wg.Done()

	backoffCounter := &backoff.Backoff{
		Min: 500 * time.Millisecond,
		Max: time.Minute,
	}

	go func() {
		<-l.shutdown
		log.Info("shutting down %v/%s, closing socket", l.addr, proto)
		listener.Close()
	}()

	for {
		log.Notice("listening on %v/%s", l.addr, proto)

		consume()

		select {
		case <-l.shutdown:
			return
		default:
		}
		for {
			log.Notice("reopening %v/%s", l.addr, proto)
			err := reconnect()
			if err == nil {
				backoffCounter.Reset()
				break
			}

			select {
			case <-l.shutdown:
				log.Info("shutting down %v/%s, closing socket", l.addr, proto)
				return
			default:
			}
			backoffDuration := backoffCounter.Duration()
			log.Error("error listening on %v/%s, retrying after %v: %s", l.addr, proto, backoffDuration, err)
			<-time.After(backoffDuration)
		}
	}
}

func (l *Listener) listenTcp() error {
	laddr, err := net.ResolveTCPAddr("tcp", l.addr)
	if err != nil {
		return err
	}
	l.tcpList, err = net.ListenTCP("tcp", laddr)
	if err != nil {
		return err
	}
	return nil
}

func (l *Listener) acceptTcp() {
	for {
		c, err := l.tcpList.AcceptTCP()
		if err != nil {
			select {
			case <-l.shutdown:
				return
			default:
				log.Error("error accepting on %v/tcp, closing connection: %s", l.addr, err)
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

func (l *Listener) acceptTcpConn(c net.Conn) {
	defer l.wg.Done()

	go func() {
		<-l.shutdown
		c.Close()
	}()

	l.handler.Handle(c)
}

func (l *Listener) listenUdp() error {
	udp_addr, err := net.ResolveUDPAddr("udp", l.addr)
	if err != nil {
		return err
	}
	l.udpConn, err = net.ListenUDP("udp", udp_addr)
	if err != nil {
		return err
	}
	return nil
}

func (l *Listener) consumeUdp() {
	buffer := make([]byte, 65535)
	for {
		// read a packet into buffer
		b, src, err := l.udpConn.ReadFrom(buffer)
		if err != nil {
			select {
			case <-l.shutdown:
				return
			default:
				log.Error("error reading packet on %v/udp, closing connection: %s", l.addr, err)
				l.udpConn.Close()
				return
			}
		}

		// handle the packet
		log.Debug("listen.go: udp packet from %v (length: %d)", src, b)
		l.handler.Handle(bytes.NewReader(buffer[:b]))
	}
}

func (l *Listener) Name() string {
	return l.name
}

func (l *Listener) Stop() bool {
	close(l.shutdown)
	l.wg.Wait()
	return true
}
