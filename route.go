package main

import (
	"sync"
)

type Route struct {
	Type    interface{} // actually RouteType, can we do this better?
	Key     string
	Matcher Matcher
	Dests   []*Destination
	In      chan []byte // incoming metrics
	sync.Mutex
}

type RouteType int
type sendAllMatch RouteType
type sendFirstMatch RouteType

func NewRoute(routeType interface{}, key, prefix, sub, regex string) (*Route, error) {
	m, err := NewMatcher(prefix, sub, regex)
	if err != nil {
		return nil, err
	}
	return &Route{routeType, key, *m, make([]*Destination, 0), make(chan []byte), sync.Mutex{}}, nil
}

func (route *Route) Run() error {
	// later, as we add more route types (consistent hashing, RR, etc) we might need to do something like:
	//switch route.Type.(type) {
	//case sendAllMatch:
	//    fallthrough
	//case sendFirstMatch:

	for _, dest := range route.Dests {
		err := dest.Run()
		if err != nil {
			return err
		}
	}
	return nil
}

func (route *Route) Match(s []byte) bool {
	return route.Matcher.Match(s)
}

// to view the state of the table/route at any point in time
// we might add more functions to view specific entries if the need for that appears
func (route *Route) Snapshot() Route {
	route.Lock()
	dests := make([]*Destination, len(route.Dests))
	defer route.Unlock()
	for i, d := range route.Dests {
		snap := d.Snapshot()
		dests[i] = &snap
	}
	return Route{route.Type, route.Key, route.Matcher.Snapshot(), dests, nil, sync.Mutex{}}
}

func (route *Route) Add(dest Destination) {
	route.Lock()
	defer route.Unlock()
	route.Dests = append(route.Dests, &dest)
}

func (route *Route) UpdateMatcher(matcher Matcher) {
	route.Lock()
	route.Matcher = matcher
	defer route.Unlock()
}
