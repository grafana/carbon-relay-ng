package input

import (
	"net"

	metrics "github.com/Dieterbe/go-metrics"
	"github.com/graphite-ng/carbon-relay-ng/stats"
	logging "github.com/op/go-logging"
)

var (
	log           = logging.MustGetLogger("input") // for tests. overridden by main
	numIn         metrics.Counter
	numInvalid    metrics.Counter
	numOutOfOrder metrics.Counter
)

func InitMetrics() {
	numIn = stats.Counter("unit=Metric.direction=in")
	numInvalid = stats.Counter("unit=Err.type=invalid")
	numOutOfOrder = stats.Counter("unit=Err.type=out_of_order")
}

func SetLogger(l *logging.Logger) {
	log = l
}

type Handler interface {
	Handle(net.Conn)
}
