package main

import (
	statsD "github.com/Dieterbe/statsd-go"
	"log"
	"sync"
)

type Table struct {
	sync.Mutex
	Blacklist []*Matcher
	routes    []*Route
	spoolDir  string
	statsd    *statsD.Client
}

func NewTable(spoolDir string, statsd *statsD.Client) *Table {
	routes := make([]*Route, 0)
	blacklist := make([]*Matcher, 0)
	t := &Table{sync.Mutex{}, blacklist, routes, spoolDir, statsd}
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

func (table *Table) Dispatch(buf []byte) {
	table.Lock()
	table.Unlock()

	for _, matcher := range table.Blacklist {
		if matcher.Match(buf) {
			statsd.Increment("target_type=count.unit=Metric.direction=blacklist")
			return
		}
	}

	routed := false

	for _, route := range table.routes {
		if route.Match(buf) {
			routed = true
			//fmt.Println("routing to " + dest.Key)
			// routes should take this in as fast as they can
			route.In <- buf
		}
	}

	// this is a bit sloppy (no synchronisation), but basically we don't want to block here
	// Dispatch should return asap
	if !routed {
		go func() {
			statsd.Increment("target_type=count.unit=Metric.direction=unroutable")
			log.Printf("unrouteable: %s\n", buf)
		}()
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
	return Table{sync.Mutex{}, blacklist, routes, table.spoolDir, nil}
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
