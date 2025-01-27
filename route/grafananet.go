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
	"github.com/jpillora/backoff"
	log "github.com/sirupsen/logrus"

	dest "github.com/grafana/carbon-relay-ng/destination"
	"github.com/grafana/carbon-relay-ng/matcher"
	"github.com/grafana/carbon-relay-ng/persister"
	"github.com/grafana/carbon-relay-ng/stats"
	"github.com/grafana/carbon-relay-ng/util"

	"github.com/grafana/metrictank/schema"
	"github.com/grafana/metrictank/schema/msg"

	conf "github.com/grafana/carbon-relay-ng/pkg/mt-conf"
)

type GrafanaNetConfig struct {
	// mandatory
	Addr        string
	ApiKey      string
	SchemasFile string

	// optional
	AggregationFile string
	BufSize         int           // amount of messages we can buffer up.
	FlushMaxNum     int           // flush after this many metrics seen
	FlushMaxWait    time.Duration // flush after this much time passed
	Timeout         time.Duration // timeout for http operations
	Concurrency     int           // number of concurrent connections to tsdb-gw
	OrgID           int
	SSLVerify       bool
	Blocking        bool
	Spool           bool // ignored for now

	// optional http backoff params for posting metrics and schemas
	ErrBackoffMin    time.Duration
	ErrBackoffFactor float64
}

func NewGrafanaNetConfig(addr, apiKey, schemasFile, aggregationFile string) (GrafanaNetConfig, error) {

	u, err := url.Parse(addr)
	if err != nil || !u.IsAbs() || u.Host == "" { // apparently "http://" is a valid absolute URL (with empty host), but we don't want that
		return GrafanaNetConfig{}, fmt.Errorf("NewGrafanaNetConfig: invalid value for 'addr': %q. need an absolute http[s] url", addr)
	}
	if !strings.HasSuffix(u.Path, "/metrics") && !strings.HasSuffix(u.Path, "/metrics/") {
		return GrafanaNetConfig{}, fmt.Errorf("NewGrafanaNetConfig: invalid value for 'addr': %q. needs to be a /metrics endpoint", addr)
	}

	if apiKey == "" {
		return GrafanaNetConfig{}, errors.New("NewGrafanaNetConfig: invalid value for 'apiKey'. value must be set to non-empty string")
	}

	if schemasFile == "" {
		return GrafanaNetConfig{}, errors.New("NewGrafanaNetConfig: invalid value for 'schemasFile'. value must be set to the path to your storage-schemas.conf file")
	}

	_, err = getSchemas(schemasFile)
	if err != nil {
		return GrafanaNetConfig{}, fmt.Errorf("NewGrafanaNetConfig: could not read schemasFile %q: %s", schemasFile, err.Error())
	}

	if aggregationFile != "" {
		_, err = conf.ReadAggregations(aggregationFile)
		if err != nil {
			return GrafanaNetConfig{}, fmt.Errorf("NewGrafanaNetConfig: could not read aggregationFile %q: %s", aggregationFile, err.Error())
		}
	}

	return GrafanaNetConfig{
		Addr:            addr,
		ApiKey:          apiKey,
		SchemasFile:     schemasFile,
		AggregationFile: aggregationFile,

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
	Cfg             GrafanaNetConfig
	schemas         persister.WhisperSchemas
	aggregation     conf.Aggregations
	schemasStr      string
	aggregationStr  string
	addrMetrics     string
	addrSchemas     string
	addrAggregation string

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

// getGrafanaNetAddr returns the metrics, schemas and aggregation address (URL) for a given config URL
// The URL we instruct customers to use is the url to post metrics to, so that one is obvious
// but we support posting both to both /graphite/metrics and /metrics , whereas the schemas and
// aggregation URL should always get the /graphite prefix.
func getGrafanaNetAddr(addr string) (string, string, string) {

	if strings.HasSuffix(addr, "/") {
		addr = addr[:len(addr)-1]
	}
	if !strings.HasSuffix(addr, "/metrics") {
		panic("getAddr called on an addr that does not end on /metrics or /metrics/ - this is not supported. Normally NewGrafanaNetConfig would already have validated this")
	}
	addrMetrics := addr

	baseAddr := strings.TrimSuffix(addrMetrics, "/metrics")
	if strings.HasSuffix(baseAddr, "/graphite") {
		baseAddr = strings.TrimSuffix(baseAddr, "/graphite")
	}

	addrSchemas := baseAddr + "/graphite/config/storageSchema"
	addrAggregation := baseAddr + "/graphite/config/storageAggregation"
	return addrMetrics, addrSchemas, addrAggregation
}

// NewGrafanaNet creates a special route that writes to a grafana.net datastore
// We will automatically run the route and the destination
func NewGrafanaNet(key string, matcher matcher.Matcher, cfg GrafanaNetConfig) (Route, error) {
	schemas, err := getSchemas(cfg.SchemasFile)
	if err != nil {
		return nil, err
	}
	schemasStr := schemas.String()

	var aggregation conf.Aggregations
	var aggregationStr string
	if cfg.AggregationFile != "" {
		aggregation, err = conf.ReadAggregations(cfg.AggregationFile)
		if err != nil {
			return nil, err
		}
		aggregationStr = aggregation.String()
	}

	cleanAddr := util.AddrToPath(cfg.Addr)

	r := &GrafanaNet{
		baseRoute:      baseRoute{"GrafanaNet", sync.Mutex{}, atomic.Value{}, key},
		Cfg:            cfg,
		schemas:        schemas,
		schemasStr:     schemasStr,
		aggregation:    aggregation,
		aggregationStr: aggregationStr,

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

	r.addrMetrics, r.addrSchemas, r.addrAggregation = getGrafanaNetAddr(cfg.Addr)

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

	if cfg.AggregationFile != "" {
		go r.updateAggregation()
	}

	return r, nil
}

// run manages incoming and outgoing data for a shard
func (route *GrafanaNet) run(in chan []byte) {
	var metrics []*schema.MetricData
	buffer := new(bytes.Buffer)

	timer := time.NewTimer(route.Cfg.FlushMaxWait)
	for {
		select {
		case buf := <-in:
			route.numBuffered.Dec(1)
			md, err := parseMetric(buf, route.schemas, route.Cfg.OrgID)
			if err != nil {
				log.Errorf("RouteGrafanaNet: parseMetric failed: %s. skipping metric", err)
				continue
			}
			md.SetId()
			metrics = append(metrics, md)

			if len(metrics) == route.Cfg.FlushMaxNum {
				metrics = route.retryFlush(metrics, buffer)
				// reset our timer
				if !timer.Stop() {
					<-timer.C
				}
				timer.Reset(route.Cfg.FlushMaxWait)
			}
		case <-timer.C:
			timer.Reset(route.Cfg.FlushMaxWait)
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
	req, err := http.NewRequest("POST", route.addrMetrics, bytes.NewReader(body))
	if err != nil {
		panic(err)
	}
	req.Header.Add("Authorization", "Bearer "+route.Cfg.ApiKey)
	req.Header.Add("Content-Type", "rt-metric-binary-snappy")
	req.Header.Add("User-Agent", UserAgent)
	req.Header.Add("Carbon-Relay-NG-Instance", Instance)
	boff := &backoff.Backoff{
		Min:    route.Cfg.ErrBackoffMin,
		Max:    30 * time.Second,
		Factor: route.Cfg.ErrBackoffFactor,
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
		log.Warnf("GrafanaNet failed to submit data to %s: %s - will try again in %s (this attempt took %s)", route.addrMetrics, err.Error(), b, dur)
		time.Sleep(b)
		// re-instantiate body, since the previous .Do() attempt would have Read it all the way
		req.Body = ioutil.NopCloser(bytes.NewReader(body))
	}
	log.Debugf("GrafanaNet sent %d metrics in %s -msg size %d", len(metrics), dur, len(body))
	route.durationTickFlush.Update(dur)
	route.tickFlushSize.Update(int64(len(body)))
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
	log.Tracef("route %s sending to dest %s: %s", route.key, route.addrMetrics, buf)
	buf = bytes.TrimSpace(buf)
	index := bytes.Index(buf, []byte(" "))
	if index == -1 {
		log.Error("RouteGrafanaNet: invalid message")
		return
	}

	key := buf[:index]
	hasher := fnv.New32a()
	hasher.Write(key)
	shard := int(hasher.Sum32() % uint32(route.Cfg.Concurrency))
	route.dispatch(route.in[shard], buf, route.numBuffered, route.numDropBuffFull)
}

func (route *GrafanaNet) Flush() error {
	//conf := route.config.Load().(Config)
	// no-op. Flush() is currently not called by anything.
	return nil
}

func (route *GrafanaNet) updateSchemas() {
	route.postConfig(route.addrSchemas, route.schemasStr)
	for range time.Tick(6 * time.Hour) {
		route.postConfig(route.addrSchemas, route.schemasStr)
	}
}

func (route *GrafanaNet) updateAggregation() {
	route.postConfig(route.addrAggregation, route.aggregationStr)
	for range time.Tick(6 * time.Hour) {
		route.postConfig(route.addrAggregation, route.aggregationStr)
	}
}

func (route *GrafanaNet) postConfig(path, cfg string) {

	boff := &backoff.Backoff{
		Min:    route.Cfg.ErrBackoffMin,
		Max:    30 * time.Minute,
		Factor: route.Cfg.ErrBackoffFactor,
		Jitter: true,
	}

	for {
		req, err := http.NewRequest("POST", path, strings.NewReader(cfg))
		if err != nil {
			panic(err)
		}
		req.Header.Add("Authorization", "Bearer "+route.Cfg.ApiKey)
		req.Header.Add("User-Agent", UserAgent)
		req.Header.Add("Carbon-Relay-NG-Instance", Instance)
		resp, err := route.client.Do(req)
		if err != nil {
			log.Warnf("got error for %s: %s", path, err.Error())
			time.Sleep(boff.Duration())
			continue
		}
		boff.Reset()
		if resp.StatusCode == http.StatusNotFound {
			// if grafana cloud is not updated yet for this new feature.
			// we are still done with our work. no need to log anything
		} else if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			// it got accepted, we're done.
			log.Infof("GrafanaNet %s submitted", path)
		} else {
			// if it's neither of the above, let's log it, but make it look not too scary
			log.Infof("GrafanaNet %s resulted in code %s (should be harmless)", path, resp.Status)
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
	snapshot := route.baseRoute.Snapshot()
	snapshot.Addr = route.Cfg.Addr
	return snapshot
}
