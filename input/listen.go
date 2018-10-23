package input

import (
	"bytes"
	"net"
	"sync"
	"time"

	"github.com/jpillora/backoff"
	log "github.com/sirupsen/logrus"
)

// Listener takes care of TCP/UDP networking
// and relies on the Handler to take care of reading data
type Listener struct {
	wg          sync.WaitGroup
	kind        string // the kind of associated handler
	addr        string
	readTimeout time.Duration
	tcpList     *net.TCPListener
	udpConn     *net.UDPConn
	Handler     Handler
	shutdown    chan struct{}
	HandleConn  func(l *Listener, c net.Conn)
	HandleData  func(l *Listener, data []byte, src net.Addr)
}

// NewListener creates a new listener.
func NewListener(addr string, readTimeout time.Duration, handler Handler) *Listener {
	return &Listener{
		kind:        handler.Kind(),
		addr:        addr,
		readTimeout: readTimeout,
		Handler:     handler,
		shutdown:    make(chan struct{}),
		HandleConn:  handleConn,
		HandleData:  handleData,
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

func (l *Listener) run(proto string, consume func(), listen func() error, listener Closable) {
	defer l.wg.Done()

	backoffCounter := &backoff.Backoff{
		Min: 500 * time.Millisecond,
		Max: time.Minute,
	}

	go func() {
		<-l.shutdown
		log.Infof("shutting down %v/%s, closing socket", l.addr, proto)
		listener.Close()
	}()

	for {
		log.Infof("listening on %v/%s", l.addr, proto)

		consume()

		select {
		case <-l.shutdown:
			return
		default:
		}
		for {
			log.Infof("reopening %v/%s", l.addr, proto)
			err := listen()
			if err == nil {
				backoffCounter.Reset()
				break
			}

			select {
			case <-l.shutdown:
				log.Infof("shutting down %v/%s, closing socket", l.addr, proto)
				return
			default:
			}
			dur := backoffCounter.Duration()
			log.Errorf("error listening on %v/%s, retrying after %v: %s", l.addr, proto, dur, err)
			time.Sleep(dur)
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
				log.Errorf("error accepting on %v/tcp, closing connection: %s", l.addr, err)
				l.tcpList.Close()
				return
			}
		}

		l.wg.Add(1)
		go l.acceptTcpConn(c)
	}
}

func (l *Listener) acceptTcpConn(c net.Conn) {
	defer l.wg.Done()
	connClose := make(chan struct{})
	defer close(connClose)

	go func() {
		select {
		case <-l.shutdown:
			c.Close()
		case <-connClose:
		}
	}()

	l.HandleConn(l, NewTimeoutConn(c, l.readTimeout))
	c.Close()
}

// handleConn does the necessary logging and invocation of the handler
func handleConn(l *Listener, c net.Conn) {
	log.Debugf("%s handler: new tcp connection from %v", l.kind, c.RemoteAddr())

	err := l.Handler.Handle(c)

	var remoteInfo string

	rAddr := c.RemoteAddr()
	if rAddr != nil {
		remoteInfo = " for " + rAddr.String()
	}
	if err != nil {
		log.Warnf("%s handler%s returned: %s. closing conn", l.kind, remoteInfo, err)
		return
	}
	log.Debugf("%s handler%s returned. closing conn", l.kind, remoteInfo)
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
				log.Errorf("error reading packet on %v/udp, closing connection: %s", l.addr, err)
				l.udpConn.Close()
				return
			}
		}
		l.HandleData(l, buffer[:b], src)
	}
}

// handleData does the necessary logging and invocation of the handler
func handleData(l *Listener, data []byte, src net.Addr) {
	log.Debugf("%s handler: udp packet from %v (length: %d)", l.kind, src, len(data))

	err := l.Handler.Handle(bytes.NewReader(data))

	if err != nil {
		log.Warnf("%s handler: %s", l.kind, err)
		return
	}
	log.Debugf("%s handler finished", l.kind)
}

func (l *Listener) Name() string {
	return l.kind
}

func (l *Listener) Stop() bool {
	close(l.shutdown)
	l.wg.Wait()
	return true
}
