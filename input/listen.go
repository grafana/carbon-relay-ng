package input

import (
	"bytes"
	"net"
	"sync"
	"time"

	"github.com/graphite-ng/carbon-relay-ng/encoding"
	"go.uber.org/zap"

	"github.com/jpillora/backoff"
	reuse "github.com/libp2p/go-reuseport"
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
	logger      *zap.Logger
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
		logger:      zap.L().With(zap.String("localAddress", addr), zap.String("kind", handler.KindS())),
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

	runLogger := l.logger.With(zap.String("protocol", worker.protocol()))

	go func() {
		<-l.shutdown
		runLogger.Info("shutting down, closing socket")
		worker.close()
	}()

	for {
		runLogger.Info("listening")

		worker.consume(l)

		select {
		case <-l.shutdown:
			return
		default:
		}
		for {
			runLogger.Info("reopening")
			err := worker.listen(l)
			if err == nil {
				backoffCounter.Reset()
				break
			}

			select {
			case <-l.shutdown:
				runLogger.Info("shutting down, closing socket")
				return
			default:
			}
			dur := backoffCounter.Duration()
			runLogger.Error("error listening, retrying after backoff", zap.Duration("backoff", dur), zap.Error(err))
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
				l.logger.Error("error accepting tcp connection, closing connection", zap.Error(err))
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
	handleConnLogger := l.logger.With(zap.Stringer("remoteAddress", c.RemoteAddr()))
	l.logger.Debug("handleConn: new tcp connection")

	err := l.handleReader(c)
	if err != nil {
		handleConnLogger.Warn("handleConn returned an error. closing conn", zap.Error(err))
		return
	}
	handleConnLogger.Debug("handleConn returned. closing conn")
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
				l.logger.Error("error reading udp packet, closing connection", zap.Error(err))
				w.packetConn.Close()
				return
			}
		}
		data := buffer[:b]
		l.logger.Debug("handler: udp packet", zap.Stringer("remoteAddress", src), zap.ByteString("payload", data))

		reader.Reset(data)
		readErr := l.handleReader(reader)
		if readErr != nil {
			l.logger.Debug("handleReader error", zap.Error(readErr))
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
