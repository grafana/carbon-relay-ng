package route

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"hash/fnv"
	"io/ioutil"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Dieterbe/go-metrics"
	"github.com/golang/snappy"
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
	schemas persister.WhisperSchemas

	bufSize      int // amount of messages we can buffer up. each message is about 100B. so 1e7 is about 1GB.
	flushMaxNum  int
	flushMaxWait time.Duration
	timeout      time.Duration
	sslVerify    bool
	blocking     bool
	dispatch     func(chan []byte, []byte, metrics.Gauge, metrics.Counter)
	concurrency  int
	orgId        int
	in           []chan []byte
	shutdown     chan struct{}
	wg           *sync.WaitGroup
	client       *http.Client

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
// ignores spool for now
func NewGrafanaNet(key, prefix, sub, regex, addr, apiKey, schemasFile string, spool, sslVerify, blocking bool, bufSize, flushMaxNum, flushMaxWait, timeout, concurrency, orgId int) (Route, error) {
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
		schemas:   schemas,

		bufSize:      bufSize,
		flushMaxNum:  flushMaxNum,
		flushMaxWait: time.Duration(flushMaxWait) * time.Millisecond,
		timeout:      time.Duration(timeout) * time.Millisecond,
		sslVerify:    sslVerify,
		blocking:     blocking,
		concurrency:  concurrency,
		orgId:        orgId,
		in:           make([]chan []byte, concurrency),
		shutdown:     make(chan struct{}),
		wg:           new(sync.WaitGroup),

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

	r.bufferSize.Update(int64(bufSize))

	if blocking {
		r.dispatch = dispatchBlocking
	} else {
		r.dispatch = dispatchNonBlocking
	}

	r.wg.Add(r.concurrency)
	for i := 0; i < r.concurrency; i++ {
		r.in[i] = make(chan []byte, bufSize/r.concurrency)
		go r.run(r.in[i])
	}
	r.config.Store(baseConfig{*m, make([]*dest.Destination, 0)})

	// start off with a transport the same as Go's DefaultTransport
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
			DualStack: true,
		}).DialContext,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	// disable http 2.0 because there seems to be a compatibility problem between nginx hosts and the golang http2 implementation
	// which would occasionally result in bogus `400 Bad Request` errors.
	transport.TLSNextProto = make(map[string]func(authority string, c *tls.Conn) http.RoundTripper)

	if !r.sslVerify {
		transport.TLSClientConfig = &tls.Config{
			InsecureSkipVerify: true,
		}
	}

	r.client = &http.Client{
		Timeout:   r.timeout,
		Transport: transport,
	}

	return r, nil
}

// run manages incoming and outgoing data for a shard
func (route *GrafanaNet) run(in chan []byte) {
	var metrics []*schema.MetricData
	buffer := new(bytes.Buffer)

	timer := time.NewTimer(route.flushMaxWait)
	for {
		select {
		case buf := <-in:
			route.numBuffered.Dec(1)
			md, err := parseMetric(buf, route.schemas, route.orgId)
			if err != nil {
				log.Error("RouteGrafanaNet: %s", err)
				continue
			}
			md.SetId()
			metrics = append(metrics, md)

			if len(metrics) == route.flushMaxNum {
				metrics = route.retryFlush(metrics, buffer)
				// reset our timer
				if !timer.Stop() {
					<-timer.C
				}
				timer.Reset(route.flushMaxWait)
			}
		case <-timer.C:
			timer.Reset(route.flushMaxWait)
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
	req, err := http.NewRequest("POST", route.addr, bytes.NewReader(body))
	if err != nil {
		panic(err)
	}
	req.Header.Add("Authorization", "Bearer "+route.apiKey)
	req.Header.Add("Content-Type", "rt-metric-binary-snappy")
	boff := &backoff.Backoff{
		Min:    100 * time.Millisecond,
		Max:    30 * time.Second,
		Factor: 1.5,
		Jitter: true,
	}
	var dur time.Duration
	for {
		dur, err = route.flush(req)
		if err == nil {
			break
		}
		route.numErrFlush.Inc(1)
		b := boff.Duration()
		log.Warning("GrafanaNet failed to submit data: %s - will try again in %s (this attempt took %s)", err.Error(), b, dur)
		time.Sleep(b)
		// re-instantiate body, since the previous .Do() attempt would have Read it all the way
		req.Body = ioutil.NopCloser(bytes.NewReader(body))
	}
	log.Info("GrafanaNet sent metrics in %s -msg size %d", dur, len(metrics))
	route.durationTickFlush.Update(dur)
	route.tickFlushSize.Update(int64(len(metrics)))
	return metrics[:0]
}

func (route *GrafanaNet) flush(req *http.Request) (time.Duration, error) {
	pre := time.Now()
	resp, err := route.client.Do(req)
	dur := time.Since(pre)
	if err != nil {
		return dur, err
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		ioutil.ReadAll(resp.Body)
		resp.Body.Close()
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
	log.Info("route %s sending to dest %s: %s", route.key, route.addr, buf)
	buf = bytes.TrimSpace(buf)
	index := bytes.Index(buf, []byte(" "))
	if index == -1 {
		log.Error("RouteGrafanaNet: invalid message")
		return
	}

	key := buf[:index]
	hasher := fnv.New32a()
	hasher.Write(key)
	shard := int(hasher.Sum32() % uint32(route.concurrency))
	route.dispatch(route.in[shard], buf, route.numBuffered, route.numDropBuffFull)
}

func (route *GrafanaNet) Flush() error {
	//conf := route.config.Load().(Config)
	// no-op. Flush() is currently not called by anything.
	return nil
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
	snapshot.Addr = route.addr
	return snapshot
}
