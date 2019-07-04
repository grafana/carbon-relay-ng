package input

import (
	"bytes"
	"net"
	"sync"
	"time"

	"github.com/graphite-ng/carbon-relay-ng/encoding"

	"github.com/jpillora/backoff"
	reuse "github.com/libp2p/go-reuseport"
	log "github.com/sirupsen/logrus"
)

// Listener takes care of TCP/UDP networking
// and relies on the Handler to take care of reading data
type Listener struct {
	BaseInput
	wg          sync.WaitGroup
	kind        string // the kind of associated handler
	addr        string
	readTimeout time.Duration
	tcpWorkers  []tcpWorker
	udpWorkers  []udpWorker
	shutdown    chan struct{}
	HandleConn  func(l *Listener, c net.Conn)
}

const (
	UDPPacketSize = 65535
)

type worker interface {
	close()                 // close the listener
	consume(*Listener)      // consumer loop which forwards data to Listener.Handle
	listen(*Listener) error // create listener
	protocol() string       // returns the transport protocol
}

type tcpWorker struct {
	listener net.Listener
}

type udpWorker struct {
	packetConn net.PacketConn
}

// NewListener creates a new listener.
func NewListener(addr string, readTimeout time.Duration, TCPWorkerCount int, UDPWorkerCount int, handler encoding.FormatAdapter) *Listener {
	return &Listener{
		BaseInput:   BaseInput{handler: handler, name: addr},
		kind:        handler.KindS(),
		addr:        addr,
		readTimeout: readTimeout,
		shutdown:    make(chan struct{}),
		HandleConn:  handleConn,
		udpWorkers:  make([]udpWorker, UDPWorkerCount),
		tcpWorkers:  make([]tcpWorker, TCPWorkerCount),
	}
}

// Name returns Handler's name.
func (l *Listener) Name() string {
	return l.kind
}

// Start initiliaze the TCP and UDP workers and the consumer loop
func (l *Listener) Start(dispatcher Dispatcher) error {
	// listeners are set up outside of accept* here so they can interrupt startup

	l.Dispatcher = dispatcher
	// create TCP workers
	for i := 0; i < len(l.tcpWorkers); i++ {
		err := l.tcpWorkers[i].listen(l)
		if err != nil {
			return err
		}
	}

	// create UDP workers
	for i := 0; i < len(l.udpWorkers); i++ {
		err := l.udpWorkers[i].listen(l)
		if err != nil {
			return err
		}
	}

	// Run the TCP workers
	l.wg.Add(len(l.tcpWorkers))
	for workerID := range l.tcpWorkers {
		go l.run(&l.tcpWorkers[workerID])
	}
	// Run the UDP workers
	l.wg.Add(len(l.udpWorkers))
	for workerID := range l.udpWorkers {
		go l.run(&l.udpWorkers[workerID])
	}

	return nil
}

// Stop will close all the TCP and UDP listeners
func (l *Listener) Stop() error {
	close(l.shutdown)
	l.wg.Wait()
	return nil
}

func (l *Listener) run(worker worker) {
	defer l.wg.Done()

	backoffCounter := &backoff.Backoff{
		Min: 500 * time.Millisecond,
		Max: time.Minute,
	}

	go func() {
		<-l.shutdown
		log.Infof("shutting down %v/%s, closing socket", l.addr, worker.protocol())
		worker.close()
	}()

	for {
		log.Infof("listening on %v/%s", l.addr, worker.protocol())

		worker.consume(l)

		select {
		case <-l.shutdown:
			return
		default:
		}
		for {
			log.Infof("reopening %v/%s", l.addr, worker.protocol())
			err := worker.listen(l)
			if err == nil {
				backoffCounter.Reset()
				break
			}

			select {
			case <-l.shutdown:
				log.Infof("shutting down %v/%s, closing socket", l.addr, worker.protocol())
				return
			default:
			}
			dur := backoffCounter.Duration()
			log.Errorf("error listening on %v/%s, retrying after %v: %s", l.addr, worker.protocol(), dur, err)
			time.Sleep(dur)
		}
	}
}

func (w *tcpWorker) close() {
	w.listener.Close()
}

func (w *tcpWorker) consume(l *Listener) {
	for {
		conn, err := w.listener.Accept()
		if err != nil {
			select {
			case <-l.shutdown:
				return
			default:
				log.Errorf("error accepting on %v/tcp, closing connection: %s", w.listener.Addr().String(), err)
				w.listener.Close()
				return
			}
		}

		l.wg.Add(1)
		go w.acceptTcpConn(l, conn)
	}
}

func (w *tcpWorker) listen(l *Listener) error {
	listener, err := reuse.Listen("tcp", l.addr)
	if err != nil {
		return err
	}
	w.listener = listener
	return nil
}

func (w *tcpWorker) protocol() string {
	return "tcp"
}

func (w *tcpWorker) acceptTcpConn(l *Listener, conn net.Conn) {
	defer l.wg.Done()
	connClose := make(chan struct{})
	defer close(connClose)

	go func() {
		select {
		case <-l.shutdown:
			conn.Close()
		case <-connClose:
		}
	}()

	l.HandleConn(l, NewTimeoutConn(conn, l.readTimeout))
	conn.Close()
}

// handleConn does the necessary logging and invocation of the handler
func handleConn(l *Listener, c net.Conn) {
	log.Debugf("%s handler: new tcp connection from %v", l.kind, c.RemoteAddr())

	err := l.handleReader(c)
	var remoteInfo string
	rAddr := c.RemoteAddr()
	if rAddr != nil {
		remoteInfo = " for " + rAddr.String()
	}
	if err != nil {
		log.Warnf("%s handler[%s][%s] returned: %s. closing conn", l.Name(), l.handler.KindS(), remoteInfo, err)
		return
	}
	log.Debugf("%s handler[%s][%s] returned. closing conn", l.Name(), l.handler.KindS(), remoteInfo)
}

func (w *udpWorker) close() {
	w.packetConn.Close()
}

func (w *udpWorker) consume(l *Listener) {
	buffer := make([]byte, UDPPacketSize)
	reader := &bytes.Reader{}

	for {
		// read a packet into buffer
		b, src, err := w.packetConn.ReadFrom(buffer)
		if err != nil {
			select {
			case <-l.shutdown:
				return
			default:
				log.Errorf("error reading packet on %v/udp, closing connection: %s", w.packetConn.LocalAddr().String(), err)
				w.packetConn.Close()
				return
			}
		}
		data := buffer[:b]
		log.Debugf("%s handler: udp packet from %v (length: %d)", l.kind, src, data)

		reader.Reset(data)
		readErr := l.handleReader(reader)
		if readErr != nil {
			log.Debug(readErr)
		}
	}
}

func (w *udpWorker) listen(l *Listener) error {
	packetConn, err := reuse.ListenPacket("udp", l.addr)
	if err != nil {
		return err
	}
	w.packetConn = packetConn
	return nil
}

func (w *udpWorker) protocol() string {
	return "udp"
}
