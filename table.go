package main

import (
	statsD "github.com/Dieterbe/statsd-go"
	"sync"
)

type Table struct {
	sync.Mutex
	routes   []*Route
	spooldir string
	statsd   *statsD.Client
}

func NewTable(spoolDir string, statsd *statsD.Client) *Table {
	routes := make([]*Route, 0, 0)
	return &Table{routes, spoolDir, statsd}
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

func (table *Table) Dispatch(buf []byte, first_only bool) (destd bool) {
	//fmt.Println("entering dispatch")
	dests.Lock()
	defer dests.Unlock()
	for _, dest := range dests.Map {
		if dest.Reg.Match(buf) {
			destd = true
			//fmt.Println("routing to " + dest.Key)
			dest.ch <- buf
			if first_only {
				break
			}
		}
	}
	//fmt.Println("Dispatched")
	return destd
}

// to view the state of the table/route at any point in time
// we might add more functions to view specific entries if the need for that appears
func (table *Table) Snapshot() Table {
	table.Lock()
	ret := make([]Route, len(table.routes))
	defer table.Unlock()
	for i, r := range table.routes {
		ret[i] = r.Snapshot()
	}
	return table{routes, spoolDir, statsd}
}

func (table *Table) Add(route Route) {
	table.Lock()
	defer table.Unlock()
	table.routes = append(table.routes, route)
}

func (table *Table) Update(key string, addr, patt *string) error {
	dests.Lock()
	defer dests.Unlock()
	dest, found := dests.Map[key]
	if !found {
		return errors.New("unknown dest '" + key + "'")
	}
	if patt != nil {
		err := dest.updatePattern(*patt)
		if err != nil {
			return err
		}
	}
	if addr != nil {
		return dest.updateConn(*addr)
	}
	return nil
}

func (table *Table) Del(key string) error {
	dests.Lock()
	defer dests.Unlock()
	dest, found := dests.Map[key]
	if !found {
		return errors.New("unknown dest '" + key + "'")
	}
	delete(dests.Map, key)
	err := dest.Shutdown()
	if err != nil {
		// dest removed from routing table but still trying to connect
		// it won't get new stuff on its input though
		return err
	}
	return nil
}
