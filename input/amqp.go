package input

import (
	"fmt"

	"github.com/graphite-ng/carbon-relay-ng/badmetrics"
	"github.com/graphite-ng/carbon-relay-ng/cfg"
	"github.com/graphite-ng/carbon-relay-ng/table"
	"github.com/graphite-ng/carbon-relay-ng/validate"
	m20 "github.com/metrics20/go-metrics20/carbon20"
	"github.com/streadway/amqp"
)

type Amqp struct {
	config cfg.Config
	bad    *badmetrics.BadMetrics
	table  *table.Table
}

func StartAMQP(config cfg.Config, amqpcfg cfg.Amqp, tbl *table.Table, bad *badmetrics.BadMetrics) error {
	a := &Amqp{
		config: config,
		bad:    bad,
		table:  tbl,
	}

	uri := amqp.URI{
		Scheme:   "amqp",
		Host:     amqpcfg.Amqp_host,
		Port:     amqpcfg.Amqp_port,
		Username: amqpcfg.Amqp_user,
		Password: amqpcfg.Amqp_password,
		Vhost:    amqpcfg.Amqp_vhost,
	}
	log.Notice("dialing AMQP: %v", uri)
	conn, err := amqp.Dial(uri.String())
	if err != nil {
		return err
	}
	defer conn.Close()

	amqpChan, err := conn.Channel()
	if err != nil {
		return err
	}
	defer amqpChan.Close()

	// queue name will be random, as in the python implementation
	q, err := amqpChan.QueueDeclare("", false, false, true, false, nil)
	if err != nil {
		return err
	}

	err = amqpChan.QueueBind(q.Name, "#", amqpcfg.Amqp_exchange, false, nil)
	if err != nil {
		return err
	}

	c, err := amqpChan.Consume(q.Name, "carbon-relay-ng", true, true, true, false, nil)
	if err != nil {
		return err
	}

	log.Notice("consuming AMQP messages")
	for m := range c {
		a.dispatch(m.Body)
	}
	return fmt.Errorf("AMQP channel closed")
}

func (a *Amqp) dispatch(buf []byte) {
	numIn.Inc(1)
	log.Debug("dispatching message: %s", buf)

	key, _, ts, err := m20.ValidatePacket(buf, a.config.Validation_level_legacy.Level, a.config.Validation_level_m20.Level)
	if err != nil {
		a.bad.Add(key, buf, err)
		numInvalid.Inc(1)
		return
	}

	if a.config.Validate_order {
		err = validate.Ordered(key, ts)
		if err != nil {
			a.bad.Add(key, buf, err)
			numOutOfOrder.Inc(1)
			return
		}
	}

	a.table.Dispatch(buf)
}
