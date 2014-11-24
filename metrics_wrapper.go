package main

import (
	"fmt"
	"github.com/Dieterbe/go-metrics"
	"strings"
)

func Counter(key string) metrics.Counter {
	c := metrics.NewCounter()
	metrics.Register(expandKey(key), c)
	return c
}

func Gauge(key string) metrics.Gauge {
	g := metrics.NewGauge()
	metrics.Register(expandKey(key), g)
	return g
}

func Timer(key string) metrics.Timer {
	//t := metrics.NewTimer()
	//default is NewExpDecaySample(1028, 0.015)
	//histogram: NewHistogram(NewExpDecaySample(1028, 0.015)),
	histogram := metrics.NewHistogram(metrics.NewWindowSample())
	meter := metrics.NewMeter()
	t := metrics.NewCustomTimer(histogram, meter)
	metrics.Register(expandKey(key), t)
	return t
}

func Histogram(key string) metrics.Histogram {
	h := metrics.NewHistogram(metrics.NewWindowSample())
	metrics.Register(expandKey(key), h)
	return h
}

func expandKey(key string) string {
	if instance == "" {
		panic("instance must be set in graphite expandKey!")
	}
	key = fmt.Sprintf("service=%s.instance=%s.%s", service, instance, key)
	return strings.Replace(key, "=", "_is_", -1)
}
