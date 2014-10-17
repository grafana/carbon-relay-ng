package main

import (
	statsD "github.com/Dieterbe/statsd-go"
	"sync"
)

type Table struct {
	sync.Mutex
	routes   []*Route
	spoolDir string
	statsd   *statsD.Client
}

func NewTable(spoolDir string, statsd *statsD.Client) *Table {
	routes := make([]*Route, 0, 0)
	return &Table{sync.Mutex{}, routes, spoolDir, statsd}
}

// not thread safe, run this once only
func (table *Table) Run() error {
	for _, route := range table.routes {
		err := route.Run()
		if err != nil {
			return err
		}
	}
	return nil
}

func (table *Table) Dispatch(buf []byte) (destd bool) {
	//fmt.Println("entering dispatch")
	table.Lock()
	table.Unlock()
	for _, route := range table.routes {
		if route.Match(buf) {
			destd = true
			//fmt.Println("routing to " + dest.Key)
			route.In <- buf
		}
	}
	//fmt.Println("Dispatched")
	return destd
}

// to view the state of the table/route at any point in time
// we might add more functions to view specific entries if the need for that appears
func (table *Table) Snapshot() Table {
	table.Lock()
	routes := make([]*Route, len(table.routes))
	defer table.Unlock()
	for i, r := range table.routes {
		snap := r.Snapshot()
		routes[i] = &snap
	}
	return Table{sync.Mutex{}, routes, table.spoolDir, nil}
}

func (table *Table) Add(route *Route) {
	table.Lock()
	defer table.Unlock()
	table.routes = append(table.routes, route)
}

func (table *Table) Del(index int) error {
	// TODO implement this better, maybe by route key to avoid race conditions
	table.Lock()
	defer table.Unlock()
	//route, found := table.routes[index]
	//if !found {
	//		return errors.New("unknown route '" + key + "'")
	//	}
	//delete(dests.Map, key)
	//err := route.Shutdown()
	//if err != nil {
	// dest removed from routing table but still trying to connect
	// it won't get new stuff on its input though
	//	return err
	//}
	//return nil
	return nil
}
