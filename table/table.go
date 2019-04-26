package table

import (
	"bytes"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/graphite-ng/carbon-relay-ng/aggregator"
	"github.com/graphite-ng/carbon-relay-ng/badmetrics"
	"github.com/graphite-ng/carbon-relay-ng/cfg"
	"github.com/graphite-ng/carbon-relay-ng/imperatives"
	"github.com/graphite-ng/carbon-relay-ng/matcher"
	"github.com/graphite-ng/carbon-relay-ng/metrics"
	"github.com/graphite-ng/carbon-relay-ng/rewriter"
	"github.com/graphite-ng/carbon-relay-ng/route"
	"github.com/graphite-ng/carbon-relay-ng/validate"
	m20 "github.com/metrics20/go-metrics20/carbon20"
	log "github.com/sirupsen/logrus"
)

type TableConfig struct {
	Validation_level_legacy validate.LevelLegacy
	Validation_level_m20    validate.LevelM20
	Validate_order          bool
	rewriters               []rewriter.RW
	aggregators             []*aggregator.Aggregator
	blacklist               []*matcher.Matcher
	routes                  []route.Route
}

type Table struct {
	sync.Mutex              // only needed for the multiple writers
	config     atomic.Value // for reading and writing
	SpoolDir   string
	In         chan []byte `json:"-"` // channel api to trade in some performance for encapsulation, for aggregators
	bad        *badmetrics.BadMetrics
	tm         *metrics.TableMetrics
}

type TableSnapshot struct {
	Rewriters   []rewriter.RW            `json:"rewriters"`
	Aggregators []*aggregator.Aggregator `json:"aggregators"`
	Blacklist   []*matcher.Matcher       `json:"blacklist"`
	Routes      []route.Snapshot         `json:"routes"`
	SpoolDir    string
}

func New(config cfg.Config) *Table {
	t := &Table{
		sync.Mutex{},
		atomic.Value{},
		config.Spool_dir,
		make(chan []byte),
		nil,
		metrics.NewTableMetrics(),
	}

	t.config.Store(TableConfig{
		config.Validation_level_legacy,
		config.Validation_level_m20,
		config.Validate_order,
		make([]rewriter.RW, 0),
		make([]*aggregator.Aggregator, 0),
		make([]*matcher.Matcher, 0),
		make([]route.Route, 0),
	})

	go func() {
		for buf := range t.In {
			t.DispatchAggregate(buf)
		}
	}()
	return t
}

func (table *Table) GetIn() chan []byte {
	return table.In
}

func (table *Table) GetSpoolDir() string {
	return table.SpoolDir
}

// This is used by inputs to record invalid packets
// It also increments numIn so that it reflects the overall total

func (table *Table) Bad() *badmetrics.BadMetrics {
	return table.bad
}

// Dispatch is the entrypoint to send data into the table.
// it dispatches incoming metrics into matching aggregators and routes,
// after checking against the blacklist
// buf is assumed to have no whitespace at the end
func (table *Table) Dispatch(buf []byte) {

	defer metrics.ObserveSince(table.tm.RoutingDuration, time.Now())
	buf_copy := make([]byte, len(buf))
	copy(buf_copy, buf)
	log.Tracef("table received packet %s", buf_copy)

	table.tm.In.Inc()

	conf := table.config.Load().(TableConfig)

	key, val, ts, err := m20.ValidatePacket(buf_copy, conf.Validation_level_legacy.Level, conf.Validation_level_m20.Level)
	if err != nil {
		table.bad.Add(key, buf_copy, err)
		table.tm.Unrouted.WithLabelValues(metrics.TableErrorTypeInvalid).Inc()
		return
	}

	if conf.Validate_order {
		err = validate.Ordered(key, ts)
		if err != nil {
			table.bad.Add(key, buf_copy, err)
			table.tm.Unrouted.WithLabelValues(metrics.TableErrorTypeOutOfOrder).Inc()
			return
		}
	}

	fields := bytes.Fields(buf_copy)

	for _, matcher := range conf.blacklist {
		if matcher.Match(fields[0]) {
			table.tm.Unrouted.WithLabelValues(metrics.TableErrorTypeBlacklist).Inc()
			log.Tracef("table dropped %s, matched blacklist entry %s", buf_copy, matcher)
			return
		}
	}

	for _, rw := range conf.rewriters {
		fields[0] = rw.Do(fields[0])
	}

	for _, aggregator := range conf.aggregators {
		// we rely on incoming metrics already having been validated
		dropRaw := aggregator.AddMaybe(fields, val, ts)
		if dropRaw {
			log.Tracef("table dropped %s, matched dropRaw aggregator %s", buf_copy, aggregator.Regex)
			table.tm.Unrouted.WithLabelValues("drop_after_aggregation").Inc()
			return
		}
	}

	final := bytes.Join(fields, []byte(" "))

	routed := false

	for _, route := range conf.routes {
		if route.Match(fields[0]) {
			routed = true
			log.Tracef("table sending to route: %s", final)
			route.Dispatch(final)
		}
	}

	if !routed {
		table.tm.Unrouted.WithLabelValues(metrics.TableErrorTypeUnroutable).Inc()
		log.Tracef("unrouteable: %s", final)
	}
}

// DispatchAggregate dispatches aggregation output by routing metrics into the matching routes.
// buf is assumed to have no whitespace at the end
func (table *Table) DispatchAggregate(buf []byte) {
	conf := table.config.Load().(TableConfig)
	routed := false
	log.Tracef("table received aggregate packet %s", buf)

	for _, route := range conf.routes {
		if route.Match(buf) {
			routed = true
			log.Tracef("table sending to route: %s", buf)
			route.Dispatch(buf)
		}
	}

	if !routed {
		table.tm.Unrouted.WithLabelValues(metrics.TableErrorTypeUnroutable).Inc()
		log.Tracef("unrouteable: %s", buf)
	}

}

func (table *Table) IncNumInvalid() {
	table.tm.In.Inc()
	table.tm.Unrouted.WithLabelValues(metrics.TableErrorTypeInvalid)
}

// to view the state of the table/route at any point in time
// we might add more functions to view specific entries if the need for that appears
func (table *Table) Snapshot() TableSnapshot {
	conf := table.config.Load().(TableConfig)

	rewriters := make([]rewriter.RW, len(conf.rewriters))
	for i, r := range conf.rewriters {
		rewriters[i] = r
	}

	blacklist := make([]*matcher.Matcher, len(conf.blacklist))
	for i, p := range conf.blacklist {
		blacklist[i] = p
	}

	routes := make([]route.Snapshot, len(conf.routes))
	for i, r := range conf.routes {
		routes[i] = r.Snapshot()
	}

	aggs := make([]*aggregator.Aggregator, len(conf.aggregators))
	for i, a := range conf.aggregators {
		aggs[i] = a.Snapshot()
	}
	return TableSnapshot{rewriters, aggs, blacklist, routes, table.SpoolDir}
}

func (table *Table) GetRoute(key string) route.Route {
	conf := table.config.Load().(TableConfig)
	for _, r := range conf.routes {
		if r.Key() == key {
			return r
		}
	}
	return nil
}

// AddRoute adds a route to the table.
// The Route must be running already
func (table *Table) AddRoute(route route.Route) {
	table.Lock()
	defer table.Unlock()
	conf := table.config.Load().(TableConfig)
	conf.routes = append(conf.routes, route)
	table.config.Store(conf)
}

func (table *Table) AddBlacklist(matcher *matcher.Matcher) {
	table.Lock()
	defer table.Unlock()
	conf := table.config.Load().(TableConfig)
	conf.blacklist = append(conf.blacklist, matcher)
	table.config.Store(conf)
}

func (table *Table) AddAggregator(agg *aggregator.Aggregator) {
	table.Lock()
	defer table.Unlock()
	conf := table.config.Load().(TableConfig)
	conf.aggregators = append(conf.aggregators, agg)
	table.config.Store(conf)
}

func (table *Table) AddRewriter(rw rewriter.RW) {
	table.Lock()
	defer table.Unlock()
	conf := table.config.Load().(TableConfig)
	conf.rewriters = append(conf.rewriters, rw)
	table.config.Store(conf)
}

func (table *Table) Flush() error {
	conf := table.config.Load().(TableConfig)
	for _, route := range conf.routes {
		err := route.Flush()
		if err != nil {
			return err
		}
	}
	return nil
}

func (table *Table) Shutdown() error {
	table.Lock()
	defer table.Unlock()
	conf := table.config.Load().(TableConfig)
	for _, route := range conf.routes {
		err := route.Shutdown()
		if err != nil {
			return err
		}
	}
	conf.routes = make([]route.Route, 0)
	table.config.Store(conf)
	return nil
}

func (table *Table) DelAggregator(id int) error {
	table.Lock()
	defer table.Unlock()

	conf := table.config.Load().(TableConfig)

	if id >= len(conf.aggregators) {
		return fmt.Errorf("Invalid index %d", id)
	}

	agg := conf.aggregators[id]
	fmt.Println("len", len(conf.aggregators))
	conf.aggregators = append(conf.aggregators[:id], conf.aggregators[id+1:]...)
	fmt.Println("len", len(conf.aggregators))
	agg.Shutdown()
	table.config.Store(conf)
	return nil
}

func (table *Table) DelBlacklist(index int) error {
	table.Lock()
	defer table.Unlock()
	conf := table.config.Load().(TableConfig)
	if index >= len(conf.blacklist) {
		return fmt.Errorf("Invalid index %d", index)
	}
	conf.blacklist = append(conf.blacklist[:index], conf.blacklist[index+1:]...)
	table.config.Store(conf)
	return nil
}

func (table *Table) DelDestination(key string, index int) error {
	route := table.GetRoute(key)
	if route == nil {
		return fmt.Errorf("Invalid route for %v", key)
	}
	return route.DelDestination(index)
}

func (table *Table) DelRewriter(id int) error {
	table.Lock()
	defer table.Unlock()

	conf := table.config.Load().(TableConfig)

	if id >= len(conf.rewriters) {
		return fmt.Errorf("Invalid index %d", id)
	}

	conf.rewriters = append(conf.rewriters[:id], conf.rewriters[id+1:]...)
	table.config.Store(conf)
	return nil
}

// idempotent semantics, not existing is fine
func (table *Table) DelRoute(key string) error {
	table.Lock()
	defer table.Unlock()
	conf := table.config.Load().(TableConfig)
	toDelete := -1
	var i int
	var route route.Route
	for i, route = range conf.routes {
		if route.Key() == key {
			toDelete = i
			break
		}
	}
	if toDelete == -1 {
		return nil
	}

	conf.routes = append(conf.routes[:toDelete], conf.routes[toDelete+1:]...)
	table.config.Store(conf)

	err := route.Shutdown()
	if err != nil {
		// dest removed from routing table but still trying to connect
		// it won't get new stuff on its input though
		return err
	}
	return nil
}

func (table *Table) UpdateDestination(key string, index int, opts map[string]string) error {
	route := table.GetRoute(key)
	if route == nil {
		return fmt.Errorf("Invalid route for %v", key)
	}
	return route.UpdateDestination(index, opts)
}

func (table *Table) UpdateRoute(key string, opts map[string]string) error {
	route := table.GetRoute(key)
	if route == nil {
		return fmt.Errorf("Invalid route for %v", key)
	}
	return route.Update(opts)
}

func (table *Table) Print() (str string) {
	// we want to print things concisely (but no smaller than the defaults below)
	// so we have to figure out the max lengths of everything first
	// the default values can be arbitrary (bot not smaller than the column titles),
	// 'R' stands for Route, 'D' for dest, 'B' blacklist, 'A" for aggregation, 'RW' for rewriter
	maxBPrefix := 6
	maxBSub := 6
	maxBRegex := 5
	maxAFunc := 4
	maxARegex := 5
	maxAPrefix := 6
	maxASub := 6
	maxAOutFmt := 6
	maxAInterval := 8
	maxAwait := 4
	maxRType := 4
	maxRKey := 3
	maxRPrefix := 6
	maxRSub := 6
	maxRRegex := 5
	maxDPrefix := 6
	maxDSub := 6
	maxDRegex := 5
	maxDAddr := 16
	maxDSpoolDir := 16

	maxRWOld := 3
	maxRWNew := 3
	maxRWNot := 3
	maxRWMax := 3

	t := table.Snapshot()
	for _, rw := range t.Rewriters {
		maxRWOld = max(maxRWOld, len(rw.Old))
		maxRWNew = max(maxRWNew, len(rw.New))
		maxRWNot = max(maxRWNot, len(rw.Not))
		maxRWMax = max(maxRWMax, len(fmt.Sprintf("%d", rw.Max)))
	}
	for _, black := range t.Blacklist {
		maxBPrefix = max(maxBPrefix, len(black.Prefix))
		maxBSub = max(maxBSub, len(black.Sub))
		maxBRegex = max(maxBRegex, len(black.Regex))
	}
	for _, agg := range t.Aggregators {
		maxAFunc = max(maxAFunc, len(agg.Fun))
		maxARegex = max(maxARegex, len(agg.Regex))
		maxAPrefix = max(maxAPrefix, len(agg.Prefix))
		maxASub = max(maxASub, len(agg.Sub))
		maxAOutFmt = max(maxAOutFmt, len(agg.OutFmt))
		maxAInterval = max(maxAInterval, len(fmt.Sprintf("%d", agg.Interval)))
		maxAwait = max(maxAwait, len(fmt.Sprintf("%d", agg.Wait)))
	}
	for _, route := range t.Routes {
		maxRType = max(maxRType, len(route.Type))
		maxRKey = max(maxRKey, len(route.Key))
		maxRPrefix = max(maxRPrefix, len(route.Matcher.Prefix))
		maxRSub = max(maxRSub, len(route.Matcher.Sub))
		maxRRegex = max(maxRRegex, len(route.Matcher.Regex))
		for _, dest := range route.Dests {
			maxDPrefix = max(maxDPrefix, len(dest.Matcher.Prefix))
			maxDSub = max(maxDSub, len(dest.Matcher.Sub))
			maxDRegex = max(maxDRegex, len(dest.Matcher.Regex))
			maxDAddr = max(maxDAddr, len(dest.Addr))
			maxDSpoolDir = max(maxDSpoolDir, len(dest.SpoolDir))
		}
	}
	heaFmtRW := fmt.Sprintf("%%-%ds  %%-%ds  %%-%ds  %%-%ds\n", maxRWOld, maxRWNew, maxRWNot, maxRWMax)
	rowFmtRW := fmt.Sprintf("%%-%ds  %%-%ds  %%-%ds  %%-%dd\n", maxRWOld, maxRWNew, maxRWNot, maxRWMax)
	heaFmtB := fmt.Sprintf("%%-%ds  %%-%ds  %%-%ds\n", maxBPrefix, maxBSub, maxBRegex)
	rowFmtB := fmt.Sprintf("%%-%ds  %%-%ds  %%-%ds\n", maxBPrefix, maxBSub, maxBRegex)
	heaFmtA := fmt.Sprintf("%%-%ds  %%-%ds  %%-%ds  %%-%ds  %%-%ds  %%-5s  %%-%ds  %%-%ds %%-7s\n", maxAFunc, maxARegex, maxAPrefix, maxASub, maxAOutFmt, maxAInterval, maxAwait)
	rowFmtA := fmt.Sprintf("%%-%ds  %%-%ds  %%-%ds  %%-%ds  %%-%ds  %%-5t  %%-%dd  %%-%dd %%-7t\n", maxAFunc, maxARegex, maxAPrefix, maxASub, maxAOutFmt, maxAInterval, maxAwait)
	heaFmtR := fmt.Sprintf("  %%-%ds  %%-%ds  %%-%ds  %%-%ds  %%-%ds\n", maxRType, maxRKey, maxRPrefix, maxRSub, maxRRegex)
	rowFmtR := fmt.Sprintf("> %%-%ds  %%-%ds  %%-%ds  %%-%ds  %%-%ds\n", maxRType, maxRKey, maxRPrefix, maxRSub, maxRRegex)
	heaFmtD := fmt.Sprintf("%%-%ds  %%-%ds  %%-%ds  %%-%ds  %%-%ds  %%-5s  %%-6s  %%-6s\n", maxDPrefix, maxDSub, maxDRegex, maxDAddr, maxDSpoolDir)
	rowFmtD := fmt.Sprintf("%%-%ds  %%-%ds  %%-%ds  %%-%ds  %%-%ds  %%-5t  %%-6t  %%-6t\n", maxDPrefix, maxDSub, maxDRegex, maxDAddr, maxDSpoolDir)

	underscore := func(amount int) string {
		return strings.Repeat("=", amount) + "\n"
	}

	str += "\n## Rewriters:\n"
	cols := fmt.Sprintf(heaFmtRW, "old", "new", "not", "max")
	str += cols + underscore(len(cols)-1)
	for _, rw := range t.Rewriters {
		str += fmt.Sprintf(rowFmtRW, rw.Old, rw.New, rw.Not, rw.Max)
	}

	str += "\n## Blacklist:\n"
	cols = fmt.Sprintf(heaFmtB, "prefix", "substr", "regex")
	str += cols + underscore(len(cols)-1)
	for _, black := range t.Blacklist {
		str += fmt.Sprintf(rowFmtB, black.Prefix, black.Sub, black.Regex)
	}

	str += "\n## Aggregations:\n"
	cols = fmt.Sprintf(heaFmtA, "func", "regex", "prefix", "substr", "outFmt", "cache", "interval", "wait", "dropRaw")
	str += cols + underscore(len(cols)-1)
	for _, agg := range t.Aggregators {
		str += fmt.Sprintf(rowFmtA, agg.Fun, agg.Regex, agg.Prefix, agg.Sub, agg.OutFmt, agg.Cache, agg.Interval, agg.Wait, agg.DropRaw)
	}

	str += "\n## Routes:\n"
	cols = fmt.Sprintf(heaFmtR, "type", "key", "prefix", "substr", "regex")
	rcols := fmt.Sprintf(heaFmtD, "prefix", "substr", "regex", "addr", "spoolDir", "spool", "pickle", "online")
	indent := "  "
	str += cols + underscore(max(len(cols), len(rcols)+len(indent))-1)
	divider := indent + strings.Repeat("-", max(len(cols)-len(indent), len(rcols))-1) + "\n"

	for _, route := range t.Routes {
		m := route.Matcher
		str += fmt.Sprintf(rowFmtR, route.Type, route.Key, m.Prefix, m.Sub, m.Regex)
		if route.Type == "GrafanaNet" {
			str += indent + route.Addr + "\n"
			continue
		}

		if len(route.Dests) < 1 {
			continue
		}
		str += indent + rcols + divider
		for _, dest := range route.Dests {
			m := dest.Matcher
			str += indent + fmt.Sprintf(rowFmtD, m.Prefix, m.Sub, m.Regex, dest.Addr, dest.SpoolDir, dest.Spool, dest.Pickle, dest.Online)
		}
		str += "\n"
	}
	return
}

func InitFromConfig(config cfg.Config, meta toml.MetaData) (*Table, error) {
	table := New(config)

	err := table.InitBadMetrics(config)
	if err != nil {
		return table, err
	}

	err = table.InitCmd(config)
	if err != nil {
		return table, err
	}

	err = table.InitBlacklist(config)
	if err != nil {
		return table, err
	}

	err = table.InitAggregation(config)
	if err != nil {
		return table, err
	}

	err = table.InitRewrite(config)
	if err != nil {
		return table, err
	}

	err = table.InitRoutes(config, meta)
	if err != nil {
		return table, err
	}

	return table, nil
}

func (table *Table) InitBadMetrics(config cfg.Config) error {
	maxAge, err := time.ParseDuration(config.Bad_metrics_max_age)
	if err != nil {
		log.Errorf("could not parse badMetrics max age: %s", err.Error())
		return fmt.Errorf("could not initialize bad metrics")
	}
	table.bad = badmetrics.New(maxAge)

	return nil
}

func (table *Table) InitCmd(config cfg.Config) error {
	for i, cmd := range config.Init.Cmds {
		log.Infof("applying: %s", cmd)
		err := imperatives.Apply(table, cmd)
		if err != nil {
			log.Errorf("could not apply init cmd #%d: %s", i+1, err.Error())
			return fmt.Errorf("could not apply init cmd #%d", i+1)
		}
	}

	return nil
}

func (table *Table) InitBlacklist(config cfg.Config) error {
	for i, entry := range config.BlackList {
		parts := strings.SplitN(entry, " ", 2)
		if len(parts) < 2 {
			return fmt.Errorf("invalid blacklist cmd #%d", i+1)
		}

		prefix := ""
		sub := ""
		regex := ""

		switch parts[0] {
		case "prefix":
			prefix = parts[1]
		case "sub":
			sub = parts[1]
		case "regex":
			regex = parts[1]
		default:
			return fmt.Errorf("invalid blacklist method for cmd #%d: %s", i+1, parts[1])
		}

		m, err := matcher.New(prefix, sub, regex)
		if err != nil {
			log.Error(err.Error())
			return fmt.Errorf("could not apply blacklist cmd #%d", i+1)
		}

		table.AddBlacklist(m)
	}

	return nil
}

func (table *Table) InitAggregation(config cfg.Config) error {
	for i, aggConfig := range config.Aggregation {
		agg, err := aggregator.New(aggConfig.Function, aggConfig.Regex, aggConfig.Prefix, aggConfig.Substr, aggConfig.Format, aggConfig.Cache, uint(aggConfig.Interval), uint(aggConfig.Wait), aggConfig.DropRaw, table.In)
		if err != nil {
			log.Error(err.Error())
			return fmt.Errorf("could not add aggregation #%d", i+1)
		}

		table.AddAggregator(agg)
	}

	return nil
}

func (table *Table) InitRewrite(config cfg.Config) error {
	for i, rewriterConfig := range config.Rewriter {
		rw, err := rewriter.New(rewriterConfig.Old, rewriterConfig.New, rewriterConfig.Not, rewriterConfig.Max)
		if err != nil {
			log.Error(err.Error())
			return fmt.Errorf("could not add rewriter #%d", i+1)
		}

		table.AddRewriter(rw)
	}

	return nil
}

func (table *Table) InitRoutes(config cfg.Config, meta toml.MetaData) error {
	for _, routeConfig := range config.Route {
		switch routeConfig.Type {
		case "sendAllMatch":
			destinations, err := imperatives.ParseDestinations(routeConfig.Destinations, table, true, routeConfig.Key)
			if err != nil {
				log.Error(err.Error())
				return fmt.Errorf("could not parse destinations for route '%s'", routeConfig.Key)
			}
			if len(destinations) == 0 {
				return fmt.Errorf("must get at least 1 destination for route '%s'", routeConfig.Key)
			}

			route, err := route.NewSendAllMatch(routeConfig.Key, routeConfig.Prefix, routeConfig.Substr, routeConfig.Regex, destinations)
			if err != nil {
				log.Error(err.Error())
				return fmt.Errorf("error adding route '%s'", routeConfig.Key)
			}
			table.AddRoute(route)
		case "sendFirstMatch":
			destinations, err := imperatives.ParseDestinations(routeConfig.Destinations, table, true, routeConfig.Key)
			if err != nil {
				log.Error(err.Error())
				return fmt.Errorf("could not parse destinations for route '%s'", routeConfig.Key)
			}
			if len(destinations) == 0 {
				return fmt.Errorf("must get at least 1 destination for route '%s'", routeConfig.Key)
			}

			route, err := route.NewSendFirstMatch(routeConfig.Key, routeConfig.Prefix, routeConfig.Substr, routeConfig.Regex, destinations)
			if err != nil {
				log.Error(err.Error())
				return fmt.Errorf("error adding route '%s'", routeConfig.Key)
			}
			table.AddRoute(route)
		case "consistentHashing":
			destinations, err := imperatives.ParseDestinations(routeConfig.Destinations, table, false, routeConfig.Key)
			if err != nil {
				log.Error(err.Error())
				return fmt.Errorf("could not parse destinations for route '%s'", routeConfig.Key)
			}
			if len(destinations) < 2 {
				return fmt.Errorf("must get at least 2 destination for route '%s'", routeConfig.Key)
			}

			route, err := route.NewConsistentHashing(routeConfig.Key, routeConfig.Prefix, routeConfig.Substr, routeConfig.Regex, destinations, routeConfig.RoutingMutations, routeConfig.CacheSize)
			if err != nil {
				log.Error(err.Error())
				return fmt.Errorf("error adding route '%s'", routeConfig.Key)
			}
			table.AddRoute(route)
		case "grafanaNet":
			var spool bool
			sslVerify := true
			var bufSize = int(1e7) // since a message is typically around 100B this is 1GB
			var flushMaxNum = 5000 // number of metrics
			var flushMaxWait = 500 // in ms
			var timeout = 10000    // in ms
			var concurrency = 100  // number of concurrent connections to tsdb-gw
			var orgId = 1

			// by merely looking at routeConfig.SslVerify we can't differentiate between the user not specifying the property
			// (which should default to false) vs specifying it as false explicitly (which should set it to false), as in both
			// cases routeConfig.SslVerify would simply be set to false.
			// So, we must look at the metadata returned by the config parser.

			// We don't use grafananet anyway.
			if routeConfig.SslVerify != nil {
				sslVerify = *(routeConfig.SslVerify)
			}

			// Note: toml library allows arbitrary casing of properties,
			// and the map keys are these properties as specified by user
			// so we can't look up directly
			if routeConfig.Spool {
				spool = routeConfig.Spool
			}
			if routeConfig.BufSize != 0 {
				bufSize = routeConfig.BufSize
			}
			if routeConfig.FlushMaxNum != 0 {
				flushMaxNum = routeConfig.FlushMaxNum
			}
			if routeConfig.FlushMaxWait != 0 {
				flushMaxWait = routeConfig.FlushMaxWait
			}
			if routeConfig.Timeout != 0 {
				timeout = routeConfig.Timeout
			}
			if routeConfig.Concurrency != 0 {
				concurrency = routeConfig.Concurrency
			}
			if routeConfig.OrgId != 0 {
				orgId = routeConfig.OrgId
			}

			route, err := route.NewGrafanaNet(routeConfig.Key, routeConfig.Prefix, routeConfig.Substr, routeConfig.Regex, routeConfig.Addr, routeConfig.ApiKey, routeConfig.SchemasFile, spool, sslVerify, routeConfig.Blocking, bufSize, flushMaxNum, flushMaxWait, timeout, concurrency, orgId)
			if err != nil {
				log.Error(err.Error())
				return fmt.Errorf("error adding route '%s'", routeConfig.Key)
			}
			table.AddRoute(route)
		case "kafkaMdm":
			var bufSize = int(1e7)  // since a message is typically around 100B this is 1GB
			var flushMaxNum = 10000 // number of metrics
			var flushMaxWait = 500  // in ms
			var timeout = 2000      // in ms
			var orgId = 1

			if routeConfig.PartitionBy != "byOrg" && routeConfig.PartitionBy != "bySeries" {
				return fmt.Errorf("invalid partitionBy for route '%s'", routeConfig.Key)
			}

			if routeConfig.BufSize != 0 {
				bufSize = routeConfig.BufSize
			}
			if routeConfig.FlushMaxNum != 0 {
				flushMaxNum = routeConfig.FlushMaxNum
			}
			if routeConfig.FlushMaxWait != 0 {
				flushMaxWait = routeConfig.FlushMaxWait
			}
			if routeConfig.Timeout != 0 {
				timeout = routeConfig.Timeout
			}
			if routeConfig.OrgId != 0 {
				orgId = routeConfig.OrgId
			}

			route, err := route.NewKafkaMdm(routeConfig.Key, routeConfig.Prefix, routeConfig.Substr, routeConfig.Regex, routeConfig.Topic, routeConfig.Codec, routeConfig.SchemasFile, routeConfig.PartitionBy, routeConfig.Brokers, bufSize, orgId, flushMaxNum, flushMaxWait, timeout, routeConfig.Blocking)
			if err != nil {
				log.Error(err.Error())
				return fmt.Errorf("error adding route '%s'", routeConfig.Key)
			}
			table.AddRoute(route)
		case "pubsub":
			var codec = "gzip"
			var format = "plain"                    // aka graphite 'linemode'
			var bufSize = int(1e7)                  // since a message is typically around 100B this is 1GB
			var flushMaxSize = int(1e7) - int(4096) // 5e6 = 5MB. max size of message. Note google limits to 10M, but we want to limit to less to account for overhead
			var flushMaxWait = 1000                 // in ms

			if routeConfig.Codec != "" {
				codec = routeConfig.Codec
			}
			if routeConfig.Format != "" {
				format = routeConfig.Format
			}
			if routeConfig.BufSize != 0 {
				bufSize = routeConfig.BufSize
			}
			if routeConfig.FlushMaxSize != 0 {
				flushMaxSize = routeConfig.FlushMaxSize
			}
			if routeConfig.FlushMaxWait != 0 {
				flushMaxWait = routeConfig.FlushMaxWait
			}

			route, err := route.NewPubSub(routeConfig.Key, routeConfig.Prefix, routeConfig.Substr, routeConfig.Regex, routeConfig.Project, routeConfig.Topic, format, codec, bufSize, flushMaxSize, flushMaxWait, routeConfig.Blocking)
			if err != nil {
				log.Error(err.Error())
				return fmt.Errorf("error adding route '%s'", routeConfig.Key)
			}
			table.AddRoute(route)
		case "cloudWatch":
			var bufSize = int(1e7)            // since a message is typically around 100B this is 1GB
			var flushMaxSize = int(20)        // Amazon limits to 20 MetricDatum/PutMetricData request https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/cloudwatch_limits.html
			var flushMaxWait = 10000          // in ms
			var storageResolution = int64(60) // Default CloudWatch resolution is 60s
			var awsProfile = ""
			var awsRegion = ""
			var awsNamespace = ""
			var awsDimensions [][]string

			if routeConfig.BufSize != 0 {
				bufSize = routeConfig.BufSize
			}
			if routeConfig.FlushMaxSize != 0 {
				flushMaxSize = routeConfig.FlushMaxSize
			}
			if routeConfig.FlushMaxWait != 0 {
				flushMaxWait = routeConfig.FlushMaxWait
			}
			if routeConfig.Profile != "" {
				awsProfile = routeConfig.Profile
			}
			if routeConfig.Region != "" {
				awsRegion = routeConfig.Region
			}
			if routeConfig.Namespace != "" {
				awsNamespace = routeConfig.Namespace
			}
			if len(routeConfig.Dimensions) > 0 {
				awsDimensions = routeConfig.Dimensions
			}
			if routeConfig.StorageResolution != 0 {
				storageResolution = routeConfig.StorageResolution
			}

			route, err := route.NewCloudWatch(routeConfig.Key, routeConfig.Prefix, routeConfig.Substr, routeConfig.Regex, awsProfile, awsRegion, awsNamespace, awsDimensions, bufSize, flushMaxSize, flushMaxWait, storageResolution, routeConfig.Blocking)
			if err != nil {
				log.Error(err.Error())
				return fmt.Errorf("error adding route '%s'", routeConfig.Key)
			}
			table.AddRoute(route)
		default:
			return fmt.Errorf("unrecognized route type '%s'", routeConfig.Type)
		}
	}

	return nil
}
