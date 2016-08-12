package main

import (
	"bytes"
	"fmt"
	"github.com/Dieterbe/go-metrics"
	"github.com/graphite-ng/carbon-relay-ng/aggregator"
	"github.com/graphite-ng/carbon-relay-ng/rewriter"
	"sync"
	"sync/atomic"
)

type TableConfig struct {
	rewriters   []rewriter.RW
	aggregators []*aggregator.Aggregator
	blacklist   []*Matcher
	routes      []Route
}

type Table struct {
	sync.Mutex                 // only needed for the multiple writers
	config        atomic.Value // for reading and writing
	spoolDir      string
	numBlacklist  metrics.Counter
	numUnroutable metrics.Counter
	In            chan []byte `json:"-"` // channel api to trade in some performance for encapsulation, for aggregators
}

type TableSnapshot struct {
	Rewriters   []rewriter.RW            `json:"rewriters"`
	Aggregators []*aggregator.Aggregator `json:"aggregators"`
	Blacklist   []*Matcher               `json:"blacklist"`
	Routes      []RouteSnapshot          `json:"routes"`
	spoolDir    string
}

func NewTable(spoolDir string) *Table {
	t := &Table{
		sync.Mutex{},
		atomic.Value{},
		spoolDir,
		Counter("unit=Metric.direction=blacklist"),
		Counter("unit=Metric.direction=unroutable"),
		make(chan []byte),
	}

	t.config.Store(TableConfig{
		make([]rewriter.RW, 0),
		make([]*aggregator.Aggregator, 0),
		make([]*Matcher, 0),
		make([]Route, 0),
	})

	go func() {
		for buf := range t.In {
			t.DispatchAggregate(buf)
		}
	}()
	return t
}

// Dispatch dispatches incoming metrics into matching aggregators and routes,
// after checking against the blacklist
// buf is assumed to have no whitespace at the end
func (table *Table) Dispatch(buf []byte) {
	conf := table.config.Load().(TableConfig)

	for _, matcher := range conf.blacklist {
		if matcher.Match(buf) {
			table.numBlacklist.Inc(1)
			return
		}
	}

	if len(conf.aggregators) > 0 {
		fields := bytes.Fields(buf)
		for _, aggregator := range conf.aggregators {
			// we rely on incoming metrics already having been validated
			if aggregator.PreMatch(fields[0]) {
				aggregator.In <- fields
			}
		}
	}

	for _, rw := range conf.rewriters {
		buf = rw.Do(buf)
	}

	routed := false

	for _, route := range conf.routes {
		if route.Match(buf) {
			routed = true
			log.Info("table sending to route: %s", buf)
			route.Dispatch(buf)
		}
	}

	if !routed {
		table.numUnroutable.Inc(1)
		log.Notice("unrouteable: %s\n", buf)
	}

}

// DispatchAggregate dispatches aggregation output by routing metrics into the matching routes.
// buf is assumed to have no whitespace at the end
func (table *Table) DispatchAggregate(buf []byte) {
	conf := table.config.Load().(TableConfig)
	routed := false

	for _, route := range conf.routes {
		if route.Match(buf) {
			routed = true
			log.Info("table sending to route: %s", buf)
			route.Dispatch(buf)
		}
	}

	if !routed {
		table.numUnroutable.Inc(1)
		log.Notice("unrouteable: %s\n", buf)
	}

}

// to view the state of the table/route at any point in time
// we might add more functions to view specific entries if the need for that appears
func (table *Table) Snapshot() TableSnapshot {
	conf := table.config.Load().(TableConfig)

	rewriters := make([]rewriter.RW, len(conf.rewriters))
	for i, r := range conf.rewriters {
		rewriters[i] = r
	}

	blacklist := make([]*Matcher, len(conf.blacklist))
	for i, p := range conf.blacklist {
		blacklist[i] = p
	}

	routes := make([]RouteSnapshot, len(conf.routes))
	for i, r := range conf.routes {
		routes[i] = r.Snapshot()
	}

	aggs := make([]*aggregator.Aggregator, len(conf.aggregators))
	for i, a := range conf.aggregators {
		aggs[i] = a.Snapshot()
	}
	return TableSnapshot{rewriters, aggs, blacklist, routes, table.spoolDir}
}

func (table *Table) GetRoute(key string) Route {
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
func (table *Table) AddRoute(route Route) {
	table.Lock()
	defer table.Unlock()
	conf := table.config.Load().(TableConfig)
	conf.routes = append(conf.routes, route)
	table.config.Store(conf)
}

func (table *Table) AddBlacklist(matcher *Matcher) {
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
	conf.routes = make([]Route, 0)
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
	var route Route
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
	// TODO also print route type, print blacklist
	// we want to print things concisely (but no smaller than the defaults below)
	// so we have to figure out the max lengths of everything first
	// the default values can be arbitrary (bot not smaller than the column titles),
	// i figured multiples of 4 should look good
	// 'R' stands for Route, 'D' for dest, 'B' blacklist, 'A" for aggregation, 'RW' for rewriter
	maxBPrefix := 4
	maxBSub := 4
	maxBRegex := 4
	maxAFunc := 4
	maxARegex := 8
	maxAOutFmt := 8
	maxAInterval := 4
	maxAwait := 4
	maxRType := 8
	maxRKey := 8
	maxRPrefix := 4
	maxRSub := 4
	maxRRegex := 4
	maxDPrefix := 4
	maxDSub := 4
	maxDRegex := 4
	maxDAddr := 16
	maxDSpoolDir := 16

	maxRWOld := 4
	maxRWNew := 4
	maxRWMax := 4

	t := table.Snapshot()
	for _, rw := range t.Rewriters {
		maxRWOld = max(maxRWOld, len(rw.Old))
		maxRWNew = max(maxRWNew, len(rw.New))
		maxRWMax = max(maxRWMax, len(fmt.Sprintf("%d", rw.Max)))
	}
	for _, black := range t.Blacklist {
		maxBPrefix = max(maxBRegex, len(black.Prefix))
		maxBSub = max(maxBSub, len(black.Sub))
		maxBRegex = max(maxBRegex, len(black.Regex))
	}
	for _, agg := range t.Aggregators {
		maxAFunc = max(maxAFunc, len(agg.Fun))
		maxARegex = max(maxARegex, len(agg.Regex))
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
			maxDSpoolDir = max(maxDSpoolDir, len(dest.spoolDir))
		}
	}
	heaFmtRW := fmt.Sprintf("%%%ds %%%ds %%%ds\n", maxRWOld+1, maxRWNew+1, maxRWMax+1)
	rowFmtRW := fmt.Sprintf("%%%ds %%%ds %%%dd\n", maxRWOld+1, maxRWNew+1, maxRWMax+1)
	heaFmtB := fmt.Sprintf("%%%ds %%%ds %%%ds\n", maxBPrefix+1, maxBSub+1, maxBRegex+1)
	rowFmtB := fmt.Sprintf("%%%ds %%%ds %%%ds\n", maxBPrefix+1, maxBSub+1, maxBRegex+1)
	heaFmtA := fmt.Sprintf("%%%ds %%%ds %%%ds %%%ds %%%ds\n", maxAFunc+1, maxARegex+1, maxAOutFmt+1, maxAInterval+1, maxAwait+1)
	rowFmtA := fmt.Sprintf("%%%ds %%%ds %%%ds %%%dd %%%dd\n", maxAFunc+1, maxARegex+1, maxAOutFmt+1, maxAInterval+1, maxAwait+1)
	heaFmtR := fmt.Sprintf("  %%%ds %%%ds %%%ds %%%ds %%%ds\n", maxRType+1, maxRKey+1, maxRPrefix+1, maxRSub+1, maxRRegex+1)
	rowFmtR := fmt.Sprintf("> %%%ds %%%ds %%%ds %%%ds %%%ds\n", maxRType+1, maxRKey+1, maxRPrefix+1, maxRSub+1, maxRRegex+1)
	heaFmtD := fmt.Sprintf("        %%%ds %%%ds %%%ds %%%ds %%%ds %%6s %%6s %%6s\n", maxDPrefix+1, maxDSub+1, maxDRegex+1, maxDAddr+1, maxDSpoolDir+1)
	rowFmtD := fmt.Sprintf("                %%%ds %%%ds %%%ds %%%ds %%%ds %%6t %%6t %%6t\n", maxDPrefix+1, maxDSub+1, maxDRegex+1, maxDAddr+1, maxDSpoolDir+1)

	underscore := func(amount int) string {
		str := ""
		for i := 1; i < amount; i++ {
			str += "="
		}
		str += "\n"
		return str
	}

	str += "\n## Rewriters:\n"
	cols := fmt.Sprintf(heaFmtRW, "old", "new", "max")
	str += cols + underscore(len(cols))
	for _, rw := range t.Rewriters {
		str += fmt.Sprintf(rowFmtRW, rw.Old, rw.New, rw.Max)
	}

	str += "\n## Blacklist:\n"
	cols = fmt.Sprintf(heaFmtB, "prefix", "substr", "regex")
	str += cols + underscore(len(cols))
	for _, black := range t.Blacklist {
		str += fmt.Sprintf(rowFmtB, black.Prefix, black.Sub, black.Regex)
	}

	str += "\n## Aggregations:\n"
	cols = fmt.Sprintf(heaFmtA, "func", "regex", "outFmt", "interval", "wait")
	str += cols + underscore(len(cols))
	for _, agg := range t.Aggregators {
		str += fmt.Sprintf(rowFmtA, agg.Fun, agg.Regex, agg.OutFmt, agg.Interval, agg.Wait)
	}

	str += "\n## Routes:\n"
	cols = fmt.Sprintf(heaFmtR, "type", "key", "prefix", "substr", "regex")
	str += cols + underscore(len(cols))

	for _, route := range t.Routes {
		m := route.Matcher
		str += fmt.Sprintf(rowFmtR, route.Type, route.Key, m.Prefix, m.Sub, m.Regex)
		str += fmt.Sprintf(heaFmtD, "prefix", "substr", "regex", "addr", "spoolDir", "spool", "pickle", "online")
		str += "              "
		for i := 1; i < maxDPrefix+maxDSub+maxDRegex+maxDAddr+maxDSpoolDir+5+3*6+10; i++ {
			str += "-"
		}
		str += "\n"
		for _, dest := range route.Dests {
			m := dest.Matcher
			str += fmt.Sprintf(rowFmtD, m.Prefix, m.Sub, m.Regex, dest.Addr, dest.spoolDir, dest.Spool, dest.Pickle, dest.Online)
		}
		str += "\n"
	}
	return
}
