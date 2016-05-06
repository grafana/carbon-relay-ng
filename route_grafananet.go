package main

import (
	"bytes"
	"fmt"
	"github.com/jpillora/backoff"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lomik/go-carbon/persister"
	"github.com/raintank/raintank-metric/msg"
	"github.com/raintank/raintank-metric/schema"
)

// amount of messages we can buffer up before providing backpressure
// since a message is typically around 100B this is 1GB
const bufSize = 1e7

const flushMaxNum = 200 // number of metrics
var flushMaxWait = 100 * time.Millisecond

type RouteGrafanaNet struct {
	baseRoute
	addr    string
	apiKey  string
	buf     chan []byte
	schemas persister.WhisperSchemas
}

// NewRouteGrafanaNet creates a special route that writes to a grafana.net datastore
// We will automatically run the route and the destination
// ignores spool for now
func NewRouteGrafanaNet(key, prefix, sub, regex, addr, apiKey, schemasFile string, spool bool) (Route, error) {
	m, err := NewMatcher(prefix, sub, regex)
	if err != nil {
		return nil, err
	}
	schemas, err := persister.ReadWhisperSchemas(schemasFile)
	if err != nil {
		return nil, err
	}
	var defaultFound bool
	for _, schema := range schemas {
		if schema.Pattern.String() == ".*" {
			defaultFound = true
		}
		if len(schema.Retentions) == 0 {
			return nil, fmt.Errorf("retention setting cannot be empty")
		}
	}
	if !defaultFound {
		// good graphite health (not sure what graphite does if there's no .*
		// but we definitely need to always be able to determine which interval to use
		return nil, fmt.Errorf("storage-conf does not have a default '.*' pattern")
	}
	r := &RouteGrafanaNet{baseRoute{sync.Mutex{}, atomic.Value{}, key}, addr, apiKey, make(chan []byte, bufSize), schemas}
	r.config.Store(baseRouteConfig{*m, make([]*Destination, 0)})
	go r.run()
	return r, nil
}

func (route *RouteGrafanaNet) run() {
	metrics := make([]*schema.MetricData, 0, flushMaxNum)
	ticker := time.NewTicker(flushMaxWait)
	client := &http.Client{}

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
		for {
			mda := schema.MetricDataArray(metrics)
			data, err := msg.CreateMsg(mda, 0, msg.FormatMetricDataArrayMsgp)
			if err != nil {
				panic(err)
			}

			req, err := http.NewRequest("POST", route.addr, bytes.NewBuffer(data))
			if err != nil {
				panic(err)
			}
			req.Header.Add("Authorization", "Bearer "+route.apiKey)
			req.Header.Add("Content-Type", "rt-metric-binary")
			resp, err := client.Do(req)
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				b.Reset()
				log.Info("GrafanaNet sent %d metrics", len(metrics))
				metrics = metrics[:0]
				break
			}
			log.Warning("GrafanaNet failed to submit data.. will try again.")
			time.Sleep(b.Duration())
		}
	}
	for {
		select {
		case buf := <-route.buf:
			md := parseMetric(buf, route.schemas)
			if md == nil {
				continue
			}
			md.SetId()
			metrics = append(metrics, md)
			if len(metrics) == flushMaxNum {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}

func parseMetric(buf []byte, schemas persister.WhisperSchemas) *schema.MetricData {
	msg := strings.TrimSpace(string(buf))

	elements := strings.Fields(msg)
	if len(elements) != 3 {
		log.Error("RouteGrafanaNet: %q error: need 3 fields", str)
		return nil
	}
	name := elements[0]
	val, err := strconv.ParseFloat(elements[1], 64)
	if err != nil {
		log.Error("RouteGrafanaNet: %q error: %s", str, err.Error())
		return nil
	}
	timestamp, err := strconv.ParseUint(elements[2], 10, 32)
	if err != nil {
		log.Error("RouteGrafanaNet: %q error: %s", str, err.Error())
		return nil
	}

	s, ok := schemas.Match(name)
	if !ok {
		panic(fmt.Errorf("couldn't find a schema for %q - this is impossible since we asserted there was a default with patt .*", name))
	}

	md := schema.MetricData{
		Name:       name,
		Interval:   s.Retentions[0].SecondsPerPoint(),
		Value:      val,
		Unit:       "",
		Time:       int64(timestamp),
		TargetType: "gauge",
		Tags:       []string{},
	}
	return &md
}

func (route *RouteGrafanaNet) Dispatch(buf []byte) {
	//conf := route.config.Load().(RouteConfig)
	// should return as quickly as possible
	log.Info("route %s sending to dest %s: %s", route.key, route.addr, buf)
	route.buf <- buf
}

func (route *RouteGrafanaNet) Flush() error {
	//conf := route.config.Load().(RouteConfig)
	// no-op. Flush() is currently not called by anything.
	return nil
}

func (route *RouteGrafanaNet) Shutdown() error {
	//conf := route.config.Load().(RouteConfig)
	return nil
}

func (route *RouteGrafanaNet) Snapshot() RouteSnapshot {
	return makeSnapshot(&route.baseRoute, "GrafanaNet")
}
