package main

import (
	"errors"
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
	r := &Route{routeType, key, *m, make([]*Destination, 0), make(chan []byte), sync.Mutex{}}
	r.Run()
	return r, nil
}

func (route *Route) Run() error {
	route.Lock()
	defer route.Unlock()

	for _, dest := range route.Dests {
		err := dest.Run()
		if err != nil {
			return err
		}
	}
	// this probably can be cleaner if we make Route an interface.
	switch route.Type.(type) {
	case sendAllMatch:
		go route.RelaySendAllMatch()
	case sendFirstMatch:
		go route.RelaySendFirstMatch()
	}
	return nil
}

func (route *Route) RelaySendAllMatch() {
	for metric := range route.In {
		route.Lock()
		for _, dest := range route.Dests {
			if dest.Match(metric) {
				// dest should handle this as quickly as it can
				dest.In <- metric
			}
		}
		route.Unlock()
	}
}

func (route *Route) RelaySendFirstMatch() {
	for metric := range route.In {
		route.Lock()
		for _, dest := range route.Dests {
			if dest.Match(metric) {
				// dest should handle this as quickly as it can
				dest.In <- metric
				break
			}
		}
		route.Unlock()
	}
}

func (route *Route) Match(s []byte) bool {
	route.Lock()
	defer route.Unlock()
	return route.Matcher.Match(s)
}

func (route *Route) Shutdown() error {
	route.Lock()
	defer route.Unlock()

	destErrs := make([]error, 0)

	for _, d := range route.Dests {
		err := d.Shutdown()
		if err != nil {
			destErrs = append(destErrs, err)
		}
	}

	if len(destErrs) == 0 {
		return nil
	}
	errStr := ""
	for _, e := range destErrs {
		errStr += "   " + e.Error()
	}
	return errors.New("one or more destinations failed to shutdown:" + errStr)
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
	defer route.Unlock()
	route.Matcher = matcher
}
