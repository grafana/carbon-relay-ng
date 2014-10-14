package main

import (
	"sync"
)

type Route struct {
	sync.Mutex
	Type  RouteType
	Dests []*Destination
	Key   string
}

type RouteType int
type sendAllMatch RouteType
type sendFirstMatch RouteType

func (route *Route) Run() error {
	if route.Type == sendAllMatch || route.Type == sendFirstMatch {
		for _, dest := range route.Dests {
			err := dest.Run()
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// to view the state of the table/route at any point in time
// we might add more functions to view specific entries if the need for that appears
func (route *Route) Snapshot() Route {
	route.Lock()
	ret := make([]Destination, len(route.dests))
	defer route.Unlock()
	for i, d := range route.dests {
		ret[i] = d.Snapshot()
	}
	return route{route.Type, dests}
}

func (route *Route) Add(dest Destination) {
	route.Lock()
	defer route.Unlock()
	route.dests = append(route.dests, dest)
}
