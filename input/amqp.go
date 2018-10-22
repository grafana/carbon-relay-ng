package input

import (
	"bufio"
	"bytes"
	"io"
	"sync"
	"time"

	"github.com/graphite-ng/carbon-relay-ng/cfg"
	"github.com/jpillora/backoff"
	log "github.com/sirupsen/logrus"
	"github.com/streadway/amqp"
)

// Closable can be closed. E.g. channel, net.Conn
type Closable interface {
	Close() error
}

// Amqp is a plugin that consumes from an amqp broker
type Amqp struct {
	wg         sync.WaitGroup
	uri        amqp.URI
	conn       Closable
	channel    Closable
	delivery   <-chan amqp.Delivery
	config     cfg.Config
	dispatcher Dispatcher
	connect    amqpConnector
	shutdown   chan struct{}
}

func (a *Amqp) close() {
	a.channel.Close()
	a.conn.Close()
}

func NewAMQP(config cfg.Config, dispatcher Dispatcher, connect amqpConnector) *Amqp {
	uri := amqp.URI{
		Scheme:   "amqp",
		Host:     config.Amqp.Amqp_host,
		Port:     config.Amqp.Amqp_port,
		Username: config.Amqp.Amqp_user,
		Password: config.Amqp.Amqp_password,
		Vhost:    config.Amqp.Amqp_vhost,
	}

	return &Amqp{
		uri:        uri,
		config:     config,
		dispatcher: dispatcher,
		connect:    connect,
		shutdown:   make(chan struct{}),
	}
}

func (a *Amqp) Name() string {
	return "amqp"
}

func (a *Amqp) Start() error {
	a.wg.Add(1)
	go a.start()
	return nil
}

func (a *Amqp) start() {
	b := &backoff.Backoff{
		Min: 500 * time.Millisecond,
	}
	defer a.wg.Done()

	for {
		err := a.connect(a)
		if err != nil {

			select {
			case <-a.shutdown:
				log.Info("shutting down AMQP client")
				return
			default:
			}
			dur := b.Duration()
			log.Errorf("connectAMQP: %v. retrying in %s", err, dur)
			time.Sleep(dur)
		} else {
			// connected successfully; reset backoff
			b.Reset()

			// blocks until channel is closed
			a.consumeAMQP()
			log.Info("consumeAMQP: channel closed")

			// reconnect immediately
			a.close()

			select {
			case <-a.shutdown:
				log.Info("shutting down AMQP client")
				return
			default:
			}
		}
	}
}

func (a *Amqp) Stop() bool {
	close(a.shutdown)
	a.wg.Wait()
	return true
}

func (a *Amqp) consumeAMQP() {
	log.Info("consuming AMQP messages")
	for {
		select {
		case m := <-a.delivery:
			// note that we don't support lines longer than 4096B. that seems very reasonable..
			r := bufio.NewReaderSize(bytes.NewReader(m.Body), 4096)
			for {
				buf, _, err := r.ReadLine()

				if err != nil {
					if io.EOF != err {
						log.Error(err.Error())
					}
					break
				}

				a.dispatcher.Dispatch(buf)
			}
		case <-a.shutdown:
			return
		}
	}
}

// amqpConnector is a function that connects an instance of *Amqp so
// it will receive messages.
// It must initialize a.channel, a.conn and a.delivery
type amqpConnector func(a *Amqp) error

// AMQPConnector connects using the given configuration
func AMQPConnector(a *Amqp) error {
	log.Infof("dialing AMQP: %v", a.uri)
	conn, err := amqp.Dial(a.uri.String())
	if err != nil {
		return err
	}
	a.conn = conn

	amqpChan, err := conn.Channel()
	if err != nil {
		a.conn.Close()
		return err
	}
	a.channel = amqpChan

	// queue name will be random, as in the python implementation
	q, err := amqpChan.QueueDeclare(a.config.Amqp.Amqp_queue, a.config.Amqp.Amqp_durable, false, a.config.Amqp.Amqp_exclusive, false, nil)
	if err != nil {
		a.close()
		return err
	}

	err = amqpChan.QueueBind(q.Name, a.config.Amqp.Amqp_key, a.config.Amqp.Amqp_exchange, false, nil)
	if err != nil {
		a.close()
		return err
	}

	a.delivery, err = amqpChan.Consume(q.Name, "carbon-relay-ng", true, a.config.Amqp.Amqp_exclusive, true, false, nil)
	if err != nil {
		a.close()
	}

	return err
}
