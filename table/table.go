package table

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/graphite-ng/carbon-relay-ng/encoding"
	"go.uber.org/zap"

	"github.com/BurntSushi/toml"
	"github.com/graphite-ng/carbon-relay-ng/aggregator"
	"github.com/graphite-ng/carbon-relay-ng/badmetrics"
	"github.com/graphite-ng/carbon-relay-ng/cfg"
	"github.com/graphite-ng/carbon-relay-ng/imperatives"
	"github.com/graphite-ng/carbon-relay-ng/matcher"
	"github.com/graphite-ng/carbon-relay-ng/metrics"
	"github.com/graphite-ng/carbon-relay-ng/rewriter"
	"github.com/graphite-ng/carbon-relay-ng/route"
)

type TableConfig struct {
	rewriters   []rewriter.RW
	aggregators []*aggregator.Aggregator
	blacklist   []*matcher.Matcher
	routes      []route.Route
}

type Table struct {
	sync.Mutex              // only needed for the multiple writers
	config     atomic.Value // for reading and writing
	SpoolDir   string
	In         chan encoding.Datapoint `json:"-"` // channel api to trade in some performance for encapsulation, for aggregators
	bad        *badmetrics.BadMetrics
	tm         *metrics.TableMetrics
	logger     *zap.Logger
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
		make(chan encoding.Datapoint),
		nil,
		metrics.NewTableMetrics(),
		zap.L(),
	}

	t.config.Store(TableConfig{
		make([]rewriter.RW, 0),
		make([]*aggregator.Aggregator, 0),
		make([]*matcher.Matcher, 0),
		make([]route.Route, 0),
	})

	go func() {
		for dp := range t.In {
			t.DispatchAggregate(dp)
		}
	}()
	return t
}

func (table *Table) GetIn() chan encoding.Datapoint {
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
func (table *Table) Dispatch(dp encoding.Datapoint) {
	tableLogger := table.logger.With(zap.Stringer("datapoint", dp))

	defer metrics.ObserveSince(table.tm.RoutingDuration, time.Now())
	tableLogger.Debug("table received packet")
	table.tm.In.Inc()

	conf := table.config.Load().(TableConfig)

	for _, matcher := range conf.blacklist {
		if matcher.MatchString(dp.Name) {
			table.tm.Unrouted.WithLabelValues(metrics.TableErrorTypeBlacklist).Inc()
			tableLogger.Debug("table dropped, matched blacklist entry", zap.Stringer("matcher", matcher))
			return
		}
	}

	for _, rw := range conf.rewriters {
		dp.Name = rw.DoString(dp.Name)
	}

	for _, aggregator := range conf.aggregators {
		// we rely on incoming metrics already having been validated
		dropRaw := aggregator.AddMaybe(dp)
		if dropRaw {
			tableLogger.Debug("table dropped, matched dropRaw aggregator",
				zap.String("metricName", dp.Name),
				zap.String("aggregatorRegex", aggregator.Regex))
			table.tm.Unrouted.WithLabelValues("drop_after_aggregation").Inc()
			return
		}
	}

	routed := false

	for _, route := range conf.routes {
		if route.MatchString(dp.Name) {
			routed = true
			tableLogger.Debug("table sending to route")
			route.Dispatch(dp)
		}
	}

	if !routed {
		table.tm.Unrouted.WithLabelValues(metrics.TableErrorTypeUnroutable).Inc()
		tableLogger.Debug("unrouteable")
	}
}

// DispatchAggregate dispatches aggregation output by routing metrics into the matching routes.
// buf is assumed to have no whitespace at the end
func (table *Table) DispatchAggregate(dp encoding.Datapoint) {
	conf := table.config.Load().(TableConfig)
	routed := false

	tableLogger := table.logger.With(zap.Stringer("datapoint", dp))

	tableLogger.Debug("table received aggregate packet")

	for _, route := range conf.routes {
		if route.MatchString(dp.Name) {
			routed = true
			tableLogger.Debug("table sending to route")
			route.Dispatch(dp)
		}
	}

	if !routed {
		table.tm.Unrouted.WithLabelValues(metrics.TableErrorTypeUnroutable).Inc()
		tableLogger.Debug("unrouteable")
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
		table.logger.Error("could not parse badMetrics max age", zap.Error(err))
		return fmt.Errorf("could not initialize bad metrics")
	}
	table.bad = badmetrics.New(maxAge)

	return nil
}

func (table *Table) InitCmd(config cfg.Config) error {
	for i, cmd := range config.Init.Cmds {
		table.logger.Info("applying command", zap.String("command", cmd))
		err := imperatives.Apply(table, cmd)
		if err != nil {
			table.logger.Error("could not apply init cmd", zap.Int("initCmd", i+1), zap.Error(err))
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
			table.logger.Error("could not apply blacklist cmd", zap.Error(err))
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
			table.logger.Error("could not add aggregation", zap.Error(err))
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
			table.logger.Error("could not add rewriter", zap.Error(err))
			return fmt.Errorf("could not add rewriter #%d", i+1)
		}

		table.AddRewriter(rw)
	}

	return nil
}

func (table *Table) InitRoutes(config cfg.Config, meta toml.MetaData) error {
	for _, routeConfig := range config.Route {
		routeConfigLogger := table.logger.With(zap.String("routeKey", routeConfig.Key))
		switch routeConfig.Type {
		case "sendAllMatch":
			destinations, err := imperatives.ParseDestinations(routeConfig.Destinations, table, true, routeConfig.Key)
			if err != nil {
				routeConfigLogger.Error("could not parse destinations for route", zap.Error(err))
				return fmt.Errorf("could not parse destinations for route '%s'", routeConfig.Key)
			}
			if len(destinations) == 0 {
				routeConfigLogger.Error("must get at least 1 destination for route")
				return fmt.Errorf("must get at least 1 destination for route '%s'", routeConfig.Key)
			}

			route, err := route.NewSendAllMatch(routeConfig.Key, routeConfig.Prefix, routeConfig.Substr, routeConfig.Regex, destinations)
			if err != nil {
				routeConfigLogger.Error("error adding route", zap.Error(err))
				return fmt.Errorf("error adding route '%s'", routeConfig.Key)
			}
			table.AddRoute(route)
		case "sendFirstMatch":
			destinations, err := imperatives.ParseDestinations(routeConfig.Destinations, table, true, routeConfig.Key)
			if err != nil {
				routeConfigLogger.Error("could not parse destinations for route", zap.Error(err))
				return fmt.Errorf("could not parse destinations for route '%s'", routeConfig.Key)
			}
			if len(destinations) == 0 {
				return fmt.Errorf("must get at least 1 destination for route '%s'", routeConfig.Key)
			}

			route, err := route.NewSendFirstMatch(routeConfig.Key, routeConfig.Prefix, routeConfig.Substr, routeConfig.Regex, destinations)
			if err != nil {
				routeConfigLogger.Error("error adding route", zap.Error(err))
				return fmt.Errorf("error adding route '%s'", routeConfig.Key)
			}
			table.AddRoute(route)
		case "consistentHashing":
			destinations, err := imperatives.ParseDestinations(routeConfig.Destinations, table, false, routeConfig.Key)
			if err != nil {
				routeConfigLogger.Error("could not parse destinations for route", zap.Error(err))
				return fmt.Errorf("could not parse destinations for route '%s'", routeConfig.Key)
			}
			if len(destinations) < 2 {
				return fmt.Errorf("must get at least 2 destination for route '%s'", routeConfig.Key)
			}

			route, err := route.NewConsistentHashing(routeConfig.Key, routeConfig.Prefix, routeConfig.Substr, routeConfig.Regex, destinations, routeConfig.RoutingMutations, routeConfig.CacheSize)
			if err != nil {
				routeConfigLogger.Error("error adding route", zap.Error(err))
				return fmt.Errorf("error adding route '%s'", routeConfig.Key)
			}
			table.AddRoute(route)
		default:
			return fmt.Errorf("unrecognized route type '%s'", routeConfig.Type)
		}
	}

	return nil
}
