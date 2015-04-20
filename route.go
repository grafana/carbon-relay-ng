package main

import (
	"fmt"
	"sync"
	"sync/atomic"
)

type RouteConfig struct {
	Matcher Matcher
	Dests   []*Destination
}

type Route interface {
	Dispatch(buf []byte)
	Match(s []byte) bool
	Snapshot() RouteSnapshot
	Key() string
	Flush() error
	Shutdown() error
	DelDestination(index int) error
	UpdateDestination(index int, opts map[string]string) error
	Update(opts map[string]string) error
}

type RouteSnapshot struct {
	Matcher Matcher        `json:"matcher"`
	Dests   []*Destination `json:"destination"`
	Type    string         `json:"type"`
	Key     string         `json:"key"`
}

type baseRoute struct {
	sync.Mutex              // only needed for the multiple writers
	config     atomic.Value // for reading and writing

	key string
}

type RouteSendAllMatch struct {
	baseRoute
}

type RouteSendFirstMatch struct {
	baseRoute
}

// NewRouteSendAllMatch creates a sendAllMatch route.
// We will automatically run the route and the given destinations
func NewRouteSendAllMatch(key, prefix, sub, regex string, destinations []*Destination) (Route, error) {
	m, err := NewMatcher(prefix, sub, regex)
	if err != nil {
		return nil, err
	}
	r := &RouteSendAllMatch{baseRoute{sync.Mutex{}, atomic.Value{}, key}}
	r.config.Store(RouteConfig{*m, destinations})
	r.run()
	return r, nil
}

// NewRouteSendFirstMatch creates a sendFirstMatch route.
// We will automatically run the route and the given destinations
func NewRouteSendFirstMatch(key, prefix, sub, regex string, destinations []*Destination) (Route, error) {
	m, err := NewMatcher(prefix, sub, regex)
	if err != nil {
		return nil, err
	}
	r := &RouteSendFirstMatch{baseRoute{sync.Mutex{}, atomic.Value{}, key}}
	r.config.Store(RouteConfig{*m, destinations})
	r.run()
	return r, nil
}

func (route *baseRoute) run() {
	conf := route.config.Load().(RouteConfig)
	for _, dest := range conf.Dests {
		dest.Run()
	}
}

func (route *RouteSendAllMatch) Dispatch(buf []byte) {
	conf := route.config.Load().(RouteConfig)

	for _, dest := range conf.Dests {
		if dest.Match(buf) {
			// dest should handle this as quickly as it can
			log.Info("route %s sending to dest %s: %s", route.key, dest.Addr, buf)
			dest.in <- buf
		}
	}
}

func (route *RouteSendFirstMatch) Dispatch(buf []byte) {
	conf := route.config.Load().(RouteConfig)

	for _, dest := range conf.Dests {
		if dest.Match(buf) {
			// dest should handle this as quickly as it can
			log.Info("route %s sending to dest %s: %s", route.key, dest.Addr, buf)
			dest.in <- buf
			break
		}
	}
}

func (route *baseRoute) Key() string {
	return route.key
}

func (route *baseRoute) Match(s []byte) bool {
	conf := route.config.Load().(RouteConfig)
	return conf.Matcher.Match(s)
}

func (route *baseRoute) Flush() error {
	conf := route.config.Load().(RouteConfig)

	for _, d := range conf.Dests {
		err := d.Flush()
		if err != nil {
			return err
		}
	}
	return nil
}

func (route *baseRoute) Shutdown() error {
	conf := route.config.Load().(RouteConfig)

	destErrs := make([]error, 0)

	for _, d := range conf.Dests {
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
	return fmt.Errorf("one or more destinations failed to shutdown:" + errStr)
}

// to view the state of the table/route at any point in time
func (route *RouteSendAllMatch) Snapshot() RouteSnapshot {
	conf := route.config.Load().(RouteConfig)
	dests := make([]*Destination, len(conf.Dests))
	for i, d := range conf.Dests {
		dests[i] = d.Snapshot()
	}
	return RouteSnapshot{conf.Matcher, dests, "sendAllMatch", route.key}
}

func (route *RouteSendFirstMatch) Snapshot() RouteSnapshot {
	conf := route.config.Load().(RouteConfig)
	dests := make([]*Destination, len(conf.Dests))
	for i, d := range conf.Dests {
		dests[i] = d.Snapshot()
	}
	return RouteSnapshot{conf.Matcher, dests, "sendFirstMatch", route.key}
}

// Add adds a new Destination to the Route and automatically runs it for you.
// The destination must not be running already!
func (route *baseRoute) Add(dest *Destination) {
	route.Lock()
	defer route.Unlock()
	conf := route.config.Load().(RouteConfig)
	dest.Run()
	conf.Dests = append(conf.Dests, dest)
	route.config.Store(conf)
}

func (route *baseRoute) DelDestination(index int) error {
	route.Lock()
	defer route.Unlock()
	conf := route.config.Load().(RouteConfig)
	if index >= len(conf.Dests) {
		return fmt.Errorf("Invalid index %d", index)
	}
	conf.Dests[index].Shutdown()
	conf.Dests = append(conf.Dests[:index], conf.Dests[index+1:]...)
	route.config.Store(conf)
	return nil
}

func (route *baseRoute) Update(opts map[string]string) error {
	route.Lock()
	defer route.Unlock()
	conf := route.config.Load().(RouteConfig)
	matcher := conf.Matcher
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
			return fmt.Errorf("no such option '%s'", name)
		}
	}
	if updateMatcher {
		matcher, err := NewMatcher(prefix, sub, regex)
		if err != nil {
			return err
		}
		conf.Matcher = *matcher
	}
	route.config.Store(conf)
	return nil
}
func (route *baseRoute) UpdateDestination(index int, opts map[string]string) error {
	conf := route.config.Load().(RouteConfig)
	if index >= len(conf.Dests) {
		return fmt.Errorf("Invalid index %d", index)
	}
	return conf.Dests[index].Update(opts)
}

func (route *baseRoute) UpdateMatcher(matcher Matcher) {
	route.Lock()
	defer route.Unlock()
	conf := route.config.Load().(RouteConfig)
	conf.Matcher = matcher
	route.config.Store(conf)
}
