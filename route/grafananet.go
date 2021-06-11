package route

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Dieterbe/go-metrics"
	"github.com/golang/snappy"
	dest "github.com/grafana/carbon-relay-ng/destination"
	"github.com/grafana/carbon-relay-ng/matcher"
	"github.com/grafana/carbon-relay-ng/persister"
	"github.com/grafana/carbon-relay-ng/stats"
	"github.com/grafana/carbon-relay-ng/util"
	"github.com/jpillora/backoff"
	log "github.com/sirupsen/logrus"

	"github.com/grafana/metrictank/schema"
	"github.com/grafana/metrictank/schema/msg"
)

type GrafanaNetConfig struct {
	// mandatory
	Addr        string
	ApiKey      string
	SchemasFile string

	// optional
	BufSize      int           // amount of messages we can buffer up.
	FlushMaxNum  int           // flush after this many metrics seen
	FlushMaxWait time.Duration // flush after this much time passed
	Timeout      time.Duration // timeout for http operations
	Concurrency  int           // number of concurrent connections to tsdb-gw
	OrgID        int
	SSLVerify    bool
	Blocking     bool
	Spool        bool // ignored for now

	// optional http backoff params for posting metrics and schemas
	ErrBackoffMin    time.Duration
	ErrBackoffFactor float64
}

func NewGrafanaNetConfig(addr, apiKey, schemasFile string) (GrafanaNetConfig, error) {

	u, err := url.Parse(addr)
	if err != nil || !u.IsAbs() || u.Host == "" { // apparently "http://" is a valid absolute URL (with empty host), but we don't want that
		return GrafanaNetConfig{}, fmt.Errorf("NewGrafanaNetConfig: invalid addr %q. need an absolute http[s] url", addr)
	}

	if apiKey == "" {
		return GrafanaNetConfig{}, errors.New("NewGrafanaNetConfig: invalid apiKey")
	}

	_, err = getSchemas(schemasFile)
	if err != nil {
		return GrafanaNetConfig{}, fmt.Errorf("NewGrafanaNetConfig: could not read schemasFile %q: %s", schemasFile, err.Error())
	}

	return GrafanaNetConfig{
		Addr:        addr,
		ApiKey:      apiKey,
		SchemasFile: schemasFile,

		BufSize:      1e7, // since a message is typically around 100B this is 1GB
		FlushMaxNum:  5000,
		FlushMaxWait: time.Second / 2,
		Timeout:      10 * time.Second,
		Concurrency:  100,
		OrgID:        1,
		SSLVerify:    true,
		Blocking:     false,
		Spool:        false,

		ErrBackoffMin:    100 * time.Millisecond,
		ErrBackoffFactor: 1.5,
	}, nil
}

type GrafanaNet struct {
	baseRoute
	cfg        GrafanaNetConfig
	schemas    persister.WhisperSchemas
	schemasStr string

	dispatch func(chan []byte, []byte, metrics.Gauge, metrics.Counter)
	in       []chan []byte
	shutdown chan struct{}
	wg       *sync.WaitGroup
	client   *http.Client

	numErrFlush       metrics.Counter
	numOut            metrics.Counter   // metrics successfully written to our buffered conn (no flushing yet)
	numDropBuffFull   metrics.Counter   // metric drops due to queue full
	durationTickFlush metrics.Timer     // only updated after successful flush
	durationManuFlush metrics.Timer     // only updated after successful flush. not implemented yet
	tickFlushSize     metrics.Histogram // only updated after successful flush
	manuFlushSize     metrics.Histogram // only updated after successful flush. not implemented yet
	numBuffered       metrics.Gauge
	bufferSize        metrics.Gauge
}

// NewGrafanaNet creates a special route that writes to a grafana.net datastore
// We will automatically run the route and the destination
func NewGrafanaNet(key string, matcher matcher.Matcher, cfg GrafanaNetConfig) (Route, error) {
	schemas, err := getSchemas(cfg.SchemasFile)
	if err != nil {
		return nil, err
	}

	cleanAddr := util.AddrToPath(cfg.Addr)

	r := &GrafanaNet{
		baseRoute:  baseRoute{sync.Mutex{}, atomic.Value{}, key},
		cfg:        cfg,
		schemas:    schemas,
		schemasStr: schemas.String(),

		in:       make([]chan []byte, cfg.Concurrency),
		shutdown: make(chan struct{}),
		wg:       new(sync.WaitGroup),

		numErrFlush:       stats.Counter("dest=" + cleanAddr + ".unit=Err.type=flush"),
		numOut:            stats.Counter("dest=" + cleanAddr + ".unit=Metric.direction=out"),
		durationTickFlush: stats.Timer("dest=" + cleanAddr + ".what=durationFlush.type=ticker"),
		durationManuFlush: stats.Timer("dest=" + cleanAddr + ".what=durationFlush.type=manual"),
		tickFlushSize:     stats.Histogram("dest=" + cleanAddr + ".unit=B.what=FlushSize.type=ticker"),
		manuFlushSize:     stats.Histogram("dest=" + cleanAddr + ".unit=B.what=FlushSize.type=manual"),
		numBuffered:       stats.Gauge("dest=" + cleanAddr + ".unit=Metric.what=numBuffered"),
		bufferSize:        stats.Gauge("dest=" + cleanAddr + ".unit=Metric.what=bufferSize"),
		numDropBuffFull:   stats.Counter("dest=" + cleanAddr + ".unit=Metric.action=drop.reason=queue_full"),
	}

	r.bufferSize.Update(int64(cfg.BufSize))

	if cfg.Blocking {
		r.dispatch = dispatchBlocking
	} else {
		r.dispatch = dispatchNonBlocking
	}

	r.wg.Add(cfg.Concurrency)
	for i := 0; i < cfg.Concurrency; i++ {
		r.in[i] = make(chan []byte, cfg.BufSize/cfg.Concurrency)
		go r.run(r.in[i])
	}
	r.config.Store(baseConfig{matcher, make([]*dest.Destination, 0)})

	// start off with a transport the same as Go's DefaultTransport
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
			DualStack: true,
		}).DialContext,
		MaxIdleConns:          cfg.Concurrency,
		MaxIdleConnsPerHost:   cfg.Concurrency,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	// disable http 2.0 because there seems to be a compatibility problem between nginx hosts and the golang http2 implementation
	// which would occasionally result in bogus `400 Bad Request` errors.
	transport.TLSNextProto = make(map[string]func(authority string, c *tls.Conn) http.RoundTripper)

	if !cfg.SSLVerify {
		transport.TLSClientConfig = &tls.Config{
			InsecureSkipVerify: true,
		}
	}

	r.client = &http.Client{
		Timeout:   cfg.Timeout,
		Transport: transport,
	}

	go r.updateSchemas()

	return r, nil
}

// run manages incoming and outgoing data for a shard
func (route *GrafanaNet) run(in chan []byte) {
	var metrics []*schema.MetricData
	buffer := new(bytes.Buffer)

	timer := time.NewTimer(route.cfg.FlushMaxWait)
	for {
		select {
		case buf := <-in:
			route.numBuffered.Dec(1)
			md, err := parseMetric(buf, route.schemas, route.cfg.OrgID)
			if err != nil {
				log.Errorf("RouteGrafanaNet: parseMetric failed: %s. skipping metric", err)
				continue
			}
			md.SetId()
			metrics = append(metrics, md)

			if len(metrics) == route.cfg.FlushMaxNum {
				metrics = route.retryFlush(metrics, buffer)
				// reset our timer
				if !timer.Stop() {
					<-timer.C
				}
				timer.Reset(route.cfg.FlushMaxWait)
			}
		case <-timer.C:
			timer.Reset(route.cfg.FlushMaxWait)
			metrics = route.retryFlush(metrics, buffer)
		case <-route.shutdown:
			metrics = route.retryFlush(metrics, buffer)
			return
		}
	}
	route.wg.Done()
}

func (route *GrafanaNet) retryFlush(metrics []*schema.MetricData, buffer *bytes.Buffer) []*schema.MetricData {
	if len(metrics) == 0 {
		return metrics
	}

	mda := schema.MetricDataArray(metrics)
	data, err := msg.CreateMsg(mda, 0, msg.FormatMetricDataArrayMsgp)
	if err != nil {
		panic(err)
	}
	route.numOut.Inc(int64(len(metrics)))

	buffer.Reset()
	snappyBody := snappy.NewWriter(buffer)
	snappyBody.Write(data)
	snappyBody.Close()
	body := buffer.Bytes()
	req, err := http.NewRequest("POST", route.cfg.Addr, bytes.NewReader(body))
	if err != nil {
		panic(err)
	}
	req.Header.Add("Authorization", "Bearer "+route.cfg.ApiKey)
	req.Header.Add("Content-Type", "rt-metric-binary-snappy")
	boff := &backoff.Backoff{
		Min:    route.cfg.ErrBackoffMin,
		Max:    30 * time.Second,
		Factor: route.cfg.ErrBackoffFactor,
		Jitter: true,
	}
	var dur time.Duration
	for {
		dur, err = route.flush(mda, req)
		if err == nil {
			break
		}
		route.numErrFlush.Inc(1)
		b := boff.Duration()
		log.Warnf("GrafanaNet failed to submit data to %s: %s - will try again in %s (this attempt took %s)", route.cfg.Addr, err.Error(), b, dur)
		time.Sleep(b)
		// re-instantiate body, since the previous .Do() attempt would have Read it all the way
		req.Body = ioutil.NopCloser(bytes.NewReader(body))
	}
	log.Debugf("GrafanaNet sent metrics in %s -msg size %d", dur, len(metrics))
	route.durationTickFlush.Update(dur)
	route.tickFlushSize.Update(int64(len(metrics)))
	return metrics[:0]
}

func (route *GrafanaNet) flush(mda schema.MetricDataArray, req *http.Request) (time.Duration, error) {
	pre := time.Now()
	resp, err := route.client.Do(req)
	dur := time.Since(pre)
	if err != nil {
		return dur, err
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		bod, err := ioutil.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			log.Warnf("GrafanaNet remote said %q, but could not read its response: %s", resp.Status, err.Error())
			return dur, nil
		}
		var mResp MetricsResponse
		err = json.Unmarshal(bod, &mResp)
		if err != nil {
			log.Warnf("GrafanaNet remote returned %q, but could not parse its response: %s", resp.Status, err.Error())
			return dur, nil
		}
		if mResp.Invalid != 0 {
			var b strings.Builder
			fmt.Fprintf(&b, "request contained %d invalid metrics that were dropped (%d valid metrics were published in this request)\n", mResp.Invalid, mResp.Published)
			for key, vErr := range mResp.ValidationErrors {
				fmt.Fprintf(&b, "  %q : %d metrics.  Examples:\n", key, vErr.Count)
				for _, idx := range vErr.ExampleIds {
					fmt.Fprintf(&b, "   - %#v\n", mda[idx])
				}
			}
			log.Warn(b.String())
		}
		return dur, nil
	}
	buf := make([]byte, 300)
	n, _ := resp.Body.Read(buf)
	ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	return dur, fmt.Errorf("http %d - %s", resp.StatusCode, buf[:n])
}

// Dispatch takes in the requested buf or drops it if blocking mode and queue of the shard is full
func (route *GrafanaNet) Dispatch(buf []byte) {
	// should return as quickly as possible
	log.Tracef("route %s sending to dest %s: %s", route.key, route.cfg.Addr, buf)
	buf = bytes.TrimSpace(buf)
	index := bytes.Index(buf, []byte(" "))
	if index == -1 {
		log.Error("RouteGrafanaNet: invalid message")
		return
	}

	key := buf[:index]
	hasher := fnv.New32a()
	hasher.Write(key)
	shard := int(hasher.Sum32() % uint32(route.cfg.Concurrency))
	route.dispatch(route.in[shard], buf, route.numBuffered, route.numDropBuffFull)
}

func (route *GrafanaNet) Flush() error {
	//conf := route.config.Load().(Config)
	// no-op. Flush() is currently not called by anything.
	return nil
}

func (route *GrafanaNet) updateSchemas() {
	for range time.Tick(6 * time.Hour) {
		route.postSchemas()
	}
}

func (route *GrafanaNet) postSchemas() {
	url := route.cfg.Addr + "/schemas"
	if strings.HasSuffix(route.cfg.Addr, "/") {
		url = route.cfg.Addr + "schemas"
	}

	boff := &backoff.Backoff{
		Min:    route.cfg.ErrBackoffMin,
		Max:    30 * time.Minute,
		Factor: route.cfg.ErrBackoffFactor,
		Jitter: true,
	}

	for {
		req, err := http.NewRequest("POST", url, strings.NewReader(route.schemasStr))
		if err != nil {
			panic(err)
		}
		req.Header.Add("Authorization", "Bearer "+route.cfg.ApiKey)
		resp, err := route.client.Do(req)
		if err != nil {
			boff.Reset()
			log.Warnf("got error for metrics/schemas: %s", err.Error())
			time.Sleep(boff.Duration())
			continue
		}
		if resp.StatusCode == http.StatusNotFound {
			// if grafana cloud is not updated yet for this new feature.
			// we are still done with our work. no need to log anything
		} else if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			// it got accepted, we're done.
			log.Info("GrafanaNet /metrics/schemas submitted")
		} else {
			// if it's neither of the above, let's log it, but make it look not too scary
			log.Infof("GrafanaNet /metrics/schemas resulted in code %s (should be harmless)", resp.Status)
		}
		ioutil.ReadAll(resp.Body)
		resp.Body.Close()
		return
	}
}

func (route *GrafanaNet) Shutdown() error {
	//conf := route.config.Load().(Config)

	// trigger all of our queues to be flushed to the tsdb-gw
	route.shutdown <- struct{}{}

	// wait for all tsdb-gw writes to complete.
	route.wg.Wait()
	return nil
}

func (route *GrafanaNet) Snapshot() Snapshot {
	snapshot := makeSnapshot(&route.baseRoute, "GrafanaNet")
	snapshot.Addr = route.cfg.Addr
	return snapshot
}
