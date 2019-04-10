package destination

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const prometheusDestinationNamespace = "destination"

// Global Instrumentation Metrics
// TODO: Rewrite how the destination package is implemented to derive Conn struct from Destination, then use a more accesible metricStore inside Destination
var errCounter = promauto.NewCounterVec(prometheus.CounterOpts{
	Namespace: prometheusDestinationNamespace,
	Name:      "error_total",
	Help:      "The total number of error received",
}, []string{"id", "error"})

var droppedMetricsCounter = promauto.NewCounterVec(prometheus.CounterOpts{
	Namespace: prometheusDestinationNamespace,
	Name:      "metrics_dropped",
	Help:      "The count of metrics dropped",
}, []string{"id", "reason"})

type Datapoint struct {
	Name string
	Val  float64
	Time uint32
}

func ParseDataPoint(buf []byte) (*Datapoint, error) {
	str := strings.TrimSpace(string(buf))
	elements := strings.Fields(str)
	if len(elements) != 3 {
		return nil, fmt.Errorf("%q doesn't have three fields", str)
	}
	name := elements[0]
	val, err := strconv.ParseFloat(elements[1], 64)
	if err != nil {
		return nil, err
	}
	timestamp, err := strconv.ParseUint(elements[2], 10, 32)
	if err != nil {
		return nil, err
	}
	return &Datapoint{name, val, uint32(timestamp)}, nil
}
