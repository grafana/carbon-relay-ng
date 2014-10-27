package main

import (
	"expvar"
	"fmt"
	"sync"
)

type Table struct {
	sync.Mutex
	Blacklist     []*Matcher
	routes        []*Route
	spoolDir      string
	numBlacklist  *expvar.Int
	numUnroutable *expvar.Int
}

func NewTable(spoolDir string) *Table {
	routes := make([]*Route, 0)
	blacklist := make([]*Matcher, 0)
	t := &Table{
		sync.Mutex{},
		blacklist,
		routes,
		spoolDir,
		Int("target_type=count.unit=Metric.direction=blacklist"),
		Int("target_type=count.unit=Metric.direction=unroutable"),
	}
	t.Run()
	return t
}

// not thread safe, run this once only
func (table *Table) Run() error {

	table.Lock()
	defer table.Unlock()

	for _, route := range table.routes {
		err := route.Run()
		if err != nil {
			return err
		}
	}
	return nil
}

// buf is assumed to have no whitespace at the end
func (table *Table) Dispatch(buf []byte) {
	table.Lock()
	defer table.Unlock()

	for _, matcher := range table.Blacklist {
		if matcher.Match(buf) {
			table.numBlacklist.Add(1)
			return
		}
	}

	routed := false

	for _, route := range table.routes {
		if route.Match(buf) {
			routed = true
			//fmt.Println("routing to " + dest.Key)
			// routes should take this in as fast as they can
			fmt.Println("table sending to route", string(buf))
			route.In <- buf
		}
	}

	if !routed {
		table.numUnroutable.Add(1)
		log.Notice("unrouteable: %s\n", buf)
	}

}

// to view the state of the table/route at any point in time
// we might add more functions to view specific entries if the need for that appears
func (table *Table) Snapshot() Table {

	table.Lock()
	defer table.Unlock()

	blacklist := make([]*Matcher, len(table.Blacklist))
	for i, p := range table.Blacklist {
		snap := p.Snapshot()
		blacklist[i] = &snap
	}

	routes := make([]*Route, len(table.routes))
	for i, r := range table.routes {
		snap := r.Snapshot()
		routes[i] = &snap
	}
	return Table{sync.Mutex{}, blacklist, routes, table.spoolDir, nil, nil}
}

func (table *Table) GetRoute(key string) *Route {
	table.Lock()
	defer table.Unlock()
	for _, r := range table.routes {
		if r.Key == key {
			return r
		}
	}
	return nil
}

func (table *Table) AddRoute(route *Route) {
	table.Lock()
	defer table.Unlock()
	table.routes = append(table.routes, route)
}

func (table *Table) AddBlacklist(matcher *Matcher) {
	table.Lock()
	defer table.Unlock()
	table.Blacklist = append(table.Blacklist, matcher)
}

func (table *Table) Flush() error {
	table.Lock()
	defer table.Unlock()
	for _, route := range table.routes {
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
	for _, route := range table.routes {
		err := route.Shutdown()
		if err != nil {
			return err
		}
	}
	table.routes = make([]*Route, 0)
	return nil
}

// idempotent semantics, not existing is fine
func (table *Table) DelRoute(key string) error {
	table.Lock()
	defer table.Unlock()
	toDelete := -1
	var i int
	var route *Route
	for i, route = range table.routes {
		if route.Key == key {
			toDelete = i
			break
		}
	}
	if toDelete == -1 {
		return nil
	}

	table.routes = append(table.routes[:toDelete], table.routes[toDelete+1:]...)

	err := route.Shutdown()
	if err != nil {
		// dest removed from routing table but still trying to connect
		// it won't get new stuff on its input though
		return err
	}
	return nil
}

func (table *Table) Print() (str string) {
	// TODO also print route type, print blacklist
	// we want to print things concisely (but no smaller than the defaults below)
	// so we have to figure out the max lengths of everything first
	// the default values can be arbitrary (bot not smaller than the column titles),
	// i figured multiples of 4 should look good
	// 'R' stands for Route, 'D' for dest, 'B' blacklist
	maxBPrefix := 4
	maxBSub := 4
	maxBRegex := 4
	maxRKey := 8
	maxRPrefix := 4
	maxRSub := 4
	maxRRegex := 4
	maxDPrefix := 4
	maxDSub := 4
	maxDRegex := 4
	maxDAddr := 16
	maxDSpoolDir := 16

	t := table.Snapshot()
	for _, black := range t.Blacklist {
		maxBPrefix = max(maxBRegex, len(black.Prefix))
		maxBSub = max(maxBSub, len(black.Sub))
		maxBRegex = max(maxBRegex, len(black.Regex))
	}
	for _, route := range t.routes {
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
	heaFmtB := fmt.Sprintf("Blacklist: %%%ds %%%ds %%%ds\n", maxBPrefix+1, maxBSub+1, maxBRegex+1)
	rowFmtB := fmt.Sprintf("           %%%ds %%%ds %%%ds\n", maxBPrefix+1, maxBSub+1, maxBRegex+1)
	heaFmtR := fmt.Sprintf("Routes: %%%ds %%%ds %%%ds %%%ds\n", maxRKey+1, maxRPrefix+1, maxRSub+1, maxRRegex+1)
	rowFmtR := fmt.Sprintf(">       %%%ds %%%ds %%%ds %%%ds\n", maxRKey+1, maxRPrefix+1, maxRSub+1, maxRRegex+1)
	heaFmtD := fmt.Sprintf("              %%%ds %%%ds %%%ds %%%ds %%%ds %%6s %%6s %%6s\n", maxDPrefix+1, maxDSub+1, maxDRegex+1, maxDAddr+1, maxDSpoolDir+1)
	rowFmtD := fmt.Sprintf("                %%%ds %%%ds %%%ds %%%ds %%%ds %%6t %%6t %%6t\n", maxDPrefix+1, maxDSub+1, maxDRegex+1, maxDAddr+1, maxDSpoolDir+1)

	str += fmt.Sprintf(heaFmtB, "prefix", "substr", "regex")
	for _, black := range t.Blacklist {
		str += fmt.Sprintf(rowFmtB, black.Prefix, black.Sub, black.Regex)
	}
	str += "\n"

	str += fmt.Sprintf(heaFmtR, "key", "prefix", "substr", "regex")
	str += "==========="
	for i := 1; i < maxRKey+maxRPrefix+maxRSub+maxRRegex+7; i++ {
		str += "="
	}
	str += "\n"

	for _, route := range t.routes {
		m := route.Matcher
		str += fmt.Sprintf(rowFmtR, route.Key, m.Prefix, m.Sub, m.Regex)
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
