package input

import (
	"bufio"
	"io"

	"github.com/graphite-ng/carbon-relay-ng/badmetrics"
	"github.com/graphite-ng/carbon-relay-ng/cfg"
	"github.com/graphite-ng/carbon-relay-ng/table"
	"github.com/graphite-ng/carbon-relay-ng/validate"
	m20 "github.com/metrics20/go-metrics20/carbon20"
)

type Plain struct {
	config cfg.Config
	bad    *badmetrics.BadMetrics
	table  *table.Table
}

func NewPlain(config cfg.Config, addr string, tbl *table.Table, badMetrics *badmetrics.BadMetrics) error {
	return listen(addr, &Plain{config, badMetrics, tbl})
}

func (p *Plain) Handle(c io.Reader) {
	scanner := bufio.NewScanner(c)
	for scanner.Scan() {
		// Note that everything in this loop should proceed as fast as it can
		// so we're not blocked and can keep processing
		// so the validation, the pipeline initiated via table.Dispatch(), etc
		// must never block.

		buf := scanner.Bytes()
		numIn.Inc(1)

		key, val, ts, err := m20.ValidatePacket(buf, p.config.Validation_level_legacy.Level, p.config.Validation_level_m20.Level)
		if err != nil {
			log.Debug("plain.go: Bad Line: %q", buf)
			p.bad.Add(key, buf, err)
			numInvalid.Inc(1)
			continue
		}

		if p.config.Validate_order {
			err = validate.Ordered(key, ts)
			if err != nil {
				log.Debug("plain.go: Out of Order Line: %q", buf)
				p.bad.Add(key, buf, err)
				numOutOfOrder.Inc(1)
				continue
			}
		}

		log.Debug("plain.go: Received Line: %q", buf)

		p.table.Dispatch(buf, val, ts)
	}
	if err := scanner.Err(); err != nil {
		log.Error(err.Error())
	}
}
