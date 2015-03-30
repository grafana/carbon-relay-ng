package main

import (
	"errors"
	"fmt"
	"sync"
)

type Route struct {
	Type    interface{}    `json:"type"` // actually RouteType, can we do this better?
	Key     string         `json:"key"`
	Matcher Matcher        `json:"matcher"`
	Dests   []*Destination `json:"destination"`
	in      chan []byte    // incoming metrics
	running bool
	sync.Mutex
}

type RouteType int
type sendAllMatch RouteType
type sendFirstMatch RouteType

// NewRoute creates a route.
// We will automatically run the route and the given destinations
func NewRoute(routeType interface{}, key, prefix, sub, regex string, destinations []*Destination) (*Route, error) {
	m, err := NewMatcher(prefix, sub, regex)
	if err != nil {
		return nil, err
	}
	r := &Route{routeType, key, *m, destinations, make(chan []byte), false, sync.Mutex{}}
	r.run()
	return r, nil
}

func (route *Route) run() {
	route.Lock()
	defer route.Unlock()
	if route.running {
		panic(fmt.Sprintf("route.Run() called on already running route '%s'", route.Key))
	}

	for _, dest := range route.Dests {
		dest.Run()
	}
	// this probably can be cleaner if we make Route an interface.
	switch route.Type.(type) {
	case sendAllMatch:
		go route.RelaySendAllMatch()
	case sendFirstMatch:
		go route.RelaySendFirstMatch()
	}
	route.running = true
}

func (route *Route) RelaySendAllMatch() {
	for buf := range route.in {
		log.Info("route %s receiving %s", route.Key, buf)
		route.Lock()
		for _, dest := range route.Dests {
			if dest.Match(buf) {
				// dest should handle this as quickly as it can
				log.Info("route %s sending to dest %s: %s", route.Key, dest.Addr, buf)
				dest.in <- buf
			}
		}
		route.Unlock()
	}
}

func (route *Route) RelaySendFirstMatch() {
	for buf := range route.in {
		route.Lock()
		for _, dest := range route.Dests {
			if dest.Match(buf) {
				// dest should handle this as quickly as it can
				dest.in <- buf
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

func (route *Route) Flush() error {
	route.Lock()
	defer route.Unlock()

	for _, d := range route.Dests {
		err := d.Flush()
		if err != nil {
			return err
		}
	}
	return nil
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
func (route *Route) Snapshot() *Route {
	route.Lock()
	dests := make([]*Destination, len(route.Dests))
	defer route.Unlock()
	for i, d := range route.Dests {
		dests[i] = d.Snapshot()
	}
	return &Route{route.Type, route.Key, route.Matcher, dests, nil, route.running, sync.Mutex{}}
}

// Add adds a new Destination to the Route and automatically runs it for you.
// The destination must not be running already!
func (route *Route) Add(dest *Destination) {
	route.Lock()
	defer route.Unlock()
	dest.Run()
	route.Dests = append(route.Dests, dest)
}

func (route *Route) DelDestination(index int) error {
	route.Lock()
	defer route.Unlock()
	if index >= len(route.Dests) {
		return errors.New(fmt.Sprintf("Invalid index %d", index))
	}
	route.Dests[index].Shutdown()
	route.Dests = append(route.Dests[:index], route.Dests[index+1:]...)
	return nil
}

func (route *Route) Update(opts map[string]string) error {
	route.Lock()
	defer route.Unlock()
	matcher := route.Matcher
	prefix := matcher.Prefix
	sub := matcher.Sub
	regex := matcher.Regex
	updateMatcher := false

	for name, val := range opts {
		switch name {
		case "prefix":
			prefix = val
			updateMatcher = true
		case "sub":
			sub = val
			updateMatcher = true
		case "regex":
			regex = val
			updateMatcher = true
		default:
			return errors.New("no such option: " + name)
		}
	}
	if updateMatcher {
		matcher, err := NewMatcher(prefix, sub, regex)
		if err != nil {
			return err
		}
		route.Matcher = *matcher
	}
	return nil
}
func (route *Route) UpdateDestination(index int, opts map[string]string) error {
	route.Lock()
	defer route.Unlock()
	if index >= len(route.Dests) {
		return errors.New(fmt.Sprintf("Invalid index %d", index))
	}
	return route.Dests[index].Update(opts)
}

func (route *Route) UpdateMatcher(matcher Matcher) {
	route.Lock()
	defer route.Unlock()
	route.Matcher = matcher
}
