package input

import (
	"bufio"
	"bytes"
	"io"
	"time"

	"github.com/graphite-ng/carbon-relay-ng/cfg"
	"github.com/jpillora/backoff"
	"github.com/streadway/amqp"
)

type Amqp struct {
	uri     amqp.URI
	conn    *amqp.Connection
	channel *amqp.Channel

	config     cfg.Config
	dispatcher Dispatcher
}

func (a *Amqp) close() {
	a.channel.Close()
	a.conn.Close()
}

func StartAMQP(config cfg.Config, dispatcher Dispatcher) {
	uri := amqp.URI{
		Scheme:   "amqp",
		Host:     config.Amqp.Amqp_host,
		Port:     config.Amqp.Amqp_port,
		Username: config.Amqp.Amqp_user,
		Password: config.Amqp.Amqp_password,
		Vhost:    config.Amqp.Amqp_vhost,
	}

	a := &Amqp{
		uri:        uri,
		config:     config,
		dispatcher: dispatcher,
	}

	b := &backoff.Backoff{
		Min: 500 * time.Millisecond,
	}
	for {
		c, err := connectAMQP(a)
		if err != nil {
			// failed to connect; backoff and try again
			log.Error("connectAMQP: %v", err)

			d := b.Duration()
			log.Info("retrying in %v", d)

			select {
			case <-shutdown:
				log.Info("shutting down AMQP client")
				return
			case <-time.After(d):
			}
		} else {
			// connected successfully; reset backoff
			b.Reset()

			// blocks until channel is closed
			consumeAMQP(a, c)
			log.Notice("consumeAMQP: channel closed")

			// reconnect immediately
			a.close()

			select {
			case <-shutdown:
				log.Info("shutting down AMQP client")
				return
			default:
			}
		}
	}
}

func connectAMQP(a *Amqp) (<-chan amqp.Delivery, error) {
	log.Notice("dialing AMQP: %v", a.uri)
	conn, err := amqp.Dial(a.uri.String())
	if err != nil {
		return nil, err
	}
	a.conn = conn

	amqpChan, err := conn.Channel()
	if err != nil {
		a.conn.Close()
		return nil, err
	}
	a.channel = amqpChan

	// queue name will be random, as in the python implementation
	q, err := amqpChan.QueueDeclare(a.config.Amqp.Amqp_queue, a.config.Amqp.Amqp_durable, false, a.config.Amqp.Amqp_exclusive, false, nil)
	if err != nil {
		a.close()
		return nil, err
	}

	err = amqpChan.QueueBind(q.Name, a.config.Amqp.Amqp_key, a.config.Amqp.Amqp_exchange, false, nil)
	if err != nil {
		a.close()
		return nil, err
	}

	c, err := amqpChan.Consume(q.Name, "carbon-relay-ng", true, a.config.Amqp.Amqp_exclusive, true, false, nil)
	if err != nil {
		a.close()
		return nil, err
	}

	return c, nil
}

func consumeAMQP(a *Amqp, c <-chan amqp.Delivery) {
	log.Notice("consuming AMQP messages")
	for {
		select {
		case m := <-c:
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
		case <-shutdown:
			return
		}
	}
}
