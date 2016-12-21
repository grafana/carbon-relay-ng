package stats

import (
	"expvar"
	"fmt"
	"strings"

	"github.com/Dieterbe/go-metrics"
	"github.com/Dieterbe/go-metrics/exp"
)

var (
	instance = "default"
	service  = "carbon-relay-ng"
)

func New(inst string) {
	instance = inst

	exp.Exp(metrics.DefaultRegistry)

	expvar.NewString("instance").Set(instance)
	expvar.NewString("service").Set(service)
}

// note in metrics2.0 counter is a type of gauge that only increases
// in go-metrics a counter can also decrease (!) -> so just don't do this.
// and can't be set to a value -> you can clear + inc(val))

func Counter(key string) metrics.Counter {
	c := metrics.NewCounter()
	return metrics.GetOrRegister(expandKey("mtype=counter."+key), c).(metrics.Counter)
}

func Gauge(key string) metrics.Gauge {
	g := metrics.NewGauge()
	return metrics.GetOrRegister(expandKey("mtype=gauge."+key), g).(metrics.Gauge)
}

func Timer(key string) metrics.Timer {
	//t := metrics.NewTimer()
	//default is NewExpDecaySample(1028, 0.015)
	//histogram: NewHistogram(NewExpDecaySample(1028, 0.015)),
	histogram := metrics.NewHistogram(metrics.NewWindowSample())
	meter := metrics.NewMeter()
	t := metrics.NewCustomTimer(histogram, meter)
	return metrics.GetOrRegister(expandKey("mtype=gauge.unit=ns."+key), t).(metrics.Timer)
}

func Histogram(key string) metrics.Histogram {
	h := metrics.NewHistogram(metrics.NewWindowSample())
	return metrics.GetOrRegister(expandKey("mtype=gauge."+key), h).(metrics.Histogram)
}

func expandKey(key string) string {
	if instance == "" {
		panic("instance must be set in graphite expandKey!")
	}
	key = fmt.Sprintf("service=%s.instance=%s.%s", service, instance, key)
	return strings.Replace(key, "=", "_is_", -1)
}
