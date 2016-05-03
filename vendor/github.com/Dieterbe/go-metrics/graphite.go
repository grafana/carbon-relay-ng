package metrics

import (
	"bufio"
	"fmt"
	m2 "github.com/metrics20/go-metrics20"
	"log"
	"net"
	"strconv"
	"strings"
	"time"
)

// GraphiteConfig provides a container with configuration parameters for
// the Graphite exporter
type GraphiteConfig struct {
	Addr          *net.TCPAddr  // Network address to connect to
	Registry      Registry      // Registry to be exported
	FlushInterval time.Duration // Flush interval
	DurationUnit  time.Duration // Time conversion unit for durations
	Prefix        string        // Prefix to be prepended to metric names. will be used for both legacy and metrics2.0
	Percentiles   []float64     // Percentiles to export from timers and histograms
}

// Graphite is a blocking exporter function which reports metrics in r
// to a graphite server located at addr, flushing them every d duration
// and prepending metric names with prefix.
func Graphite(r Registry, d time.Duration, prefix string, addr *net.TCPAddr) {
	GraphiteWithConfig(GraphiteConfig{
		Addr:          addr,
		Registry:      r,
		FlushInterval: d,
		DurationUnit:  time.Nanosecond,
		Prefix:        prefix,
		Percentiles:   []float64{0.5, 0.75, 0.95, 0.99, 0.999},
	})
}

// GraphiteWithConfig is a blocking exporter function just like Graphite,
// but it takes a GraphiteConfig instead.
func GraphiteWithConfig(c GraphiteConfig) {
	for _ = range time.Tick(c.FlushInterval) {
		if err := graphite(&c); nil != err {
			log.Println(err)
		}
	}
}

// GraphiteOnce performs a single submission to Graphite, returning a
// non-nil error on failed connections. This can be used in a loop
// similar to GraphiteWithConfig for custom error handling.
func GraphiteOnce(c GraphiteConfig) error {
	return graphite(&c)
}

func graphite(c *GraphiteConfig) error {
	now := time.Now().Unix()
	du := float64(c.DurationUnit)
	conn, err := net.DialTCP("tcp", nil, c.Addr)
	if nil != err {
		return err
	}
	defer conn.Close()
	w := bufio.NewWriter(conn)
	c.Registry.Each(func(name string, i interface{}) {
		k := c.Prefix + name
		switch metric := i.(type) {
		case Counter:
			fmt.Fprintf(w, "%s %d %d\n", m2.Counter(k, ""), metric.Count(), now)
		case Gauge:
			fmt.Fprintf(w, "%s %d %d\n", m2.Gauge(k, ""), metric.Value(), now)
		case GaugeFloat64:
			fmt.Fprintf(w, "%s %f %d\n", m2.Gauge(k, ""), metric.Value(), now)
		case Histogram:
			h := metric.Snapshot()
			ps := h.Percentiles(c.Percentiles)
			fmt.Fprintf(w, "%s %d %d\n", m2.CountMetric(k, ""), h.Count(), now)
			fmt.Fprintf(w, "%s %d %d\n", m2.Min(k, "", "", ""), h.Min(), now)
			fmt.Fprintf(w, "%s %d %d\n", m2.Max(k, "", "", ""), h.Max(), now)
			fmt.Fprintf(w, "%s %.2f %d\n", m2.Mean(k, "", "", ""), h.Mean(), now)
			fmt.Fprintf(w, "%s %.2f %d\n", m2.Std(k, "", "", ""), h.StdDev(), now)
			for psIdx, psKey := range c.Percentiles {
				pct := strings.Replace(strconv.FormatFloat(psKey*100.0, 'f', -1, 64), ".", "", 1)
				fmt.Fprintf(w, "%s %.2f %d\n", m2.Max(k, "", pct, ""), ps[psIdx], now)
			}
		case Meter:
			m := metric.Snapshot()
			fmt.Fprintf(w, "%s %d %d\n", m2.CountMetric(k, ""), m.Count(), now)
			fmt.Fprintf(w, "%s %.2f %d\n", m2.Mean(k, "1m", "", "60"), m.Rate1(), now)
			fmt.Fprintf(w, "%s %.2f %d\n", m2.Mean(k, "5m", "", "300"), m.Rate5(), now)
			fmt.Fprintf(w, "%s %.2f %d\n", m2.Mean(k, "15m", "", "900"), m.Rate15(), now)
			fmt.Fprintf(w, "%s %.2f %d\n", m2.Mean(k, "start", "", "start"), m.RateMean(), now)
		case Timer:
			t := metric.Snapshot()
			ps := t.Percentiles(c.Percentiles)
			fmt.Fprintf(w, "%s %d %d\n", m2.CountMetric(k, ""), t.Count(), now)
			fmt.Fprintf(w, "%s %d %d\n", m2.Min(k, "", "", ""), t.Min()/int64(du), now)
			fmt.Fprintf(w, "%s %d %d\n", m2.Max(k, "", "", ""), t.Max()/int64(du), now)
			fmt.Fprintf(w, "%s %.2f %d\n", m2.Mean(k, "", "", ""), t.Mean()/du, now)
			fmt.Fprintf(w, "%s %.2f %d\n", m2.Std(k, "", "", ""), t.StdDev()/du, now)
			for psIdx, psKey := range c.Percentiles {
				pct := strings.Replace(strconv.FormatFloat(psKey*100.0, 'f', -1, 64), ".", "", 1)
				fmt.Fprintf(w, "%s %.2f %d\n", m2.Max(k, "", pct, ""), ps[psIdx], now)
			}
			fmt.Fprintf(w, "%s %.2f %d\n", m2.Mean(k, "1m", "", "60"), t.Rate1(), now)
			fmt.Fprintf(w, "%s %.2f %d\n", m2.Mean(k, "5m", "", "300"), t.Rate5(), now)
			fmt.Fprintf(w, "%s %.2f %d\n", m2.Mean(k, "15m", "", "900"), t.Rate15(), now)
			fmt.Fprintf(w, "%s %.2f %d\n", m2.Mean(k, "start", "", "start"), t.RateMean(), now)
		}
		w.Flush()
	})
	return nil
}
