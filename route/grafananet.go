package route

import (
	"bytes"
	"crypto/tls"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Dieterbe/go-metrics"
	dest "github.com/graphite-ng/carbon-relay-ng/destination"
	"github.com/graphite-ng/carbon-relay-ng/matcher"
	"github.com/graphite-ng/carbon-relay-ng/stats"
	"github.com/graphite-ng/carbon-relay-ng/util"
	"github.com/jpillora/backoff"

	"github.com/lomik/go-carbon/persister"
	"gopkg.in/raintank/schema.v1"
	"gopkg.in/raintank/schema.v1/msg"
)

type GrafanaNet struct {
	baseRoute
	addr    string
	apiKey  string
	buf     chan []byte
	schemas persister.WhisperSchemas

	bufSize      int // amount of messages we can buffer up before providing backpressure. each message is about 100B. so 1e7 is about 1GB.
	flushMaxNum  int
	flushMaxWait time.Duration
	timeout      time.Duration
	sslVerify    bool

	numErrFlush       metrics.Counter
	numOut            metrics.Counter   // metrics successfully written to our buffered conn (no flushing yet)
	durationTickFlush metrics.Timer     // only updated after successful flush
	durationManuFlush metrics.Timer     // only updated after successful flush. not implemented yet
	tickFlushSize     metrics.Histogram // only updated after successful flush
	manuFlushSize     metrics.Histogram // only updated after successful flush. not implemented yet
	numBuffered       metrics.Gauge
}

// NewGrafanaNet creates a special route that writes to a grafana.net datastore
// We will automatically run the route and the destination
// ignores spool for now
func NewGrafanaNet(key, prefix, sub, regex, addr, apiKey, schemasFile string, spool, sslVerify bool, bufSize, flushMaxNum, flushMaxWait, timeout int) (Route, error) {
	m, err := matcher.New(prefix, sub, regex)
	if err != nil {
		return nil, err
	}
	schemas, err := getSchemas(schemasFile)
	if err != nil {
		return nil, err
	}

	cleanAddr := util.AddrToPath(addr)

	r := &GrafanaNet{
		baseRoute: baseRoute{sync.Mutex{}, atomic.Value{}, key},
		addr:      addr,
		apiKey:    apiKey,
		buf:       make(chan []byte, bufSize), // takes about 228MB on 64bit
		schemas:   schemas,

		bufSize:      bufSize,
		flushMaxNum:  flushMaxNum,
		flushMaxWait: time.Duration(flushMaxWait) * time.Millisecond,
		timeout:      time.Duration(timeout) * time.Millisecond,
		sslVerify:    sslVerify,

		numErrFlush:       stats.Counter("dest=" + cleanAddr + ".unit=Err.type=flush"),
		numOut:            stats.Counter("dest=" + cleanAddr + ".unit=Metric.direction=out"),
		durationTickFlush: stats.Timer("dest=" + cleanAddr + ".what=durationFlush.type=ticker"),
		durationManuFlush: stats.Timer("dest=" + cleanAddr + ".what=durationFlush.type=manual"),
		tickFlushSize:     stats.Histogram("dest=" + cleanAddr + ".unit=B.what=FlushSize.type=ticker"),
		manuFlushSize:     stats.Histogram("dest=" + cleanAddr + ".unit=B.what=FlushSize.type=manual"),
		numBuffered:       stats.Gauge("dest=" + cleanAddr + ".unit=Metric.what=numBuffered"),
	}

	r.config.Store(baseConfig{*m, make([]*dest.Destination, 0)})
	go r.run()
	return r, nil
}

func (route *GrafanaNet) run() {
	metrics := make([]*schema.MetricData, 0, route.flushMaxNum)
	ticker := time.NewTicker(route.flushMaxWait)
	client := &http.Client{
		Timeout: route.timeout,
	}
	if !route.sslVerify {
		// this transport should be the equivalent of Go's DefaultTransport
		client.Transport = &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			Dial: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).Dial,
			TLSHandshakeTimeout: 10 * time.Second,
			// except we removed ExpectContinueTimeout cause it doesn't seem very useful, requires go 1.6, and prevents using http2.
			// and except for this
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		}
	}

	b := &backoff.Backoff{
		Min:    100 * time.Millisecond,
		Max:    time.Minute,
		Factor: 1.5,
		Jitter: true,
	}

	flush := func() {
		if len(metrics) == 0 {
			return
		}

		mda := schema.MetricDataArray(metrics)
		data, err := msg.CreateMsg(mda, 0, msg.FormatMetricDataArrayMsgp)
		if err != nil {
			panic(err)
		}

		for {
			pre := time.Now()
			req, err := http.NewRequest("POST", route.addr, bytes.NewBuffer(data))
			if err != nil {
				panic(err)
			}
			req.Header.Add("Authorization", "Bearer "+route.apiKey)
			req.Header.Add("Content-Type", "rt-metric-binary")
			resp, err := client.Do(req)
			diff := time.Since(pre)
			if err == nil && resp.StatusCode >= 200 && resp.StatusCode < 300 {
				b.Reset()
				log.Info("GrafanaNet sent %d metrics in %s -msg size %d", len(metrics), diff, len(data))
				route.numOut.Inc(int64(len(metrics)))
				route.tickFlushSize.Update(int64(len(data)))
				route.durationTickFlush.Update(diff)
				metrics = metrics[:0]
				resp.Body.Close()
				break
			}
			route.numErrFlush.Inc(1)
			dur := b.Duration()
			if err != nil {
				log.Warning("GrafanaNet failed to submit data: %s will try again in %s (this attempt took %s)", err, dur, diff)
			} else {
				buf := make([]byte, 300)
				n, _ := resp.Body.Read(buf)
				log.Warning("GrafanaNet failed to submit data: http %d - %s will try again in %s (this attempt took %s)", resp.StatusCode, buf[:n], dur, diff)
				resp.Body.Close()
			}

			time.Sleep(dur)
		}
	}
	for {
		select {
		case buf := <-route.buf:
			route.numBuffered.Dec(1)
			md, err := parseMetric(buf, route.schemas)
			if err != nil {
				log.Error("RouteGrafanaNet: %s, err")
				continue
			}
			md.SetId()
			metrics = append(metrics, md)
			if len(metrics) == route.flushMaxNum {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}

func (route *GrafanaNet) Dispatch(buf []byte) {
	//conf := route.config.Load().(Config)
	// should return as quickly as possible
	log.Info("route %s sending to dest %s: %s", route.key, route.addr, buf)
	route.numBuffered.Inc(1)
	route.buf <- buf
}

func (route *GrafanaNet) Flush() error {
	//conf := route.config.Load().(Config)
	// no-op. Flush() is currently not called by anything.
	return nil
}

func (route *GrafanaNet) Shutdown() error {
	//conf := route.config.Load().(Config)
	return nil
}

func (route *GrafanaNet) Snapshot() Snapshot {
	return makeSnapshot(&route.baseRoute, "GrafanaNet")
}
