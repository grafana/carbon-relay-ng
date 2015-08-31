package main

import (
	"bytes"
	"fmt"
	"sync"
	"sync/atomic"
)

type RouteConfig interface {
	Matcher() *Matcher
	Dests() []*Destination
}

type baseRouteConfig struct {
	matcher Matcher
	dests   []*Destination
}

func (c baseRouteConfig) Matcher() *Matcher {
	return &c.matcher
}

func (c baseRouteConfig) Dests() []*Destination {
	return c.dests
}

type consistentHashingRouteConfig struct {
	baseRouteConfig
	Hasher *ConsistentHasher
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

type RouteConsistentHashing struct {
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
	r.config.Store(baseRouteConfig{*m, destinations})
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
	r.config.Store(baseRouteConfig{*m, destinations})
	r.run()
	return r, nil
}

func NewRouteConsistentHashing(key, prefix, sub, regex string, destinations []*Destination) (Route, error) {
	m, err := NewMatcher(prefix, sub, regex)
	if err != nil {
		return nil, err
	}
	r := &RouteConsistentHashing{baseRoute{sync.Mutex{}, atomic.Value{}, key}}
	hasher := NewConsistentHasher(destinations)
	r.config.Store(consistentHashingRouteConfig{baseRouteConfig{*m, destinations},
		&hasher})
	r.run()
	return r, nil
}

func (route *baseRoute) run() {
	conf := route.config.Load().(RouteConfig)
	for _, dest := range conf.Dests() {
		dest.Run()
	}
}

func (route *RouteSendAllMatch) Dispatch(buf []byte) {
	conf := route.config.Load().(RouteConfig)

	for _, dest := range conf.Dests() {
		if dest.Match(buf) {
			// dest should handle this as quickly as it can
			log.Info("route %s sending to dest %s: %s", route.key, dest.Addr, buf)
			dest.in <- buf
		}
	}
}

func (route *RouteSendFirstMatch) Dispatch(buf []byte) {
	conf := route.config.Load().(RouteConfig)

	for _, dest := range conf.Dests() {
		if dest.Match(buf) {
			// dest should handle this as quickly as it can
			log.Info("route %s sending to dest %s: %s", route.key, dest.Addr, buf)
			dest.in <- buf
			break
		}
	}
}

func (route *RouteConsistentHashing) Dispatch(buf []byte) {
	conf := route.config.Load().(consistentHashingRouteConfig)
	if pos := bytes.IndexByte(buf, ' '); pos > 0 {
		name := buf[0:pos]
		dest := conf.Dests()[conf.Hasher.GetDestinationIndex(name)]
		// dest should handle this as quickly as it can
		log.Info("route %s sending to dest %s: %s", route.key, dest.Addr, name)
		dest.in <- buf
	} else {
		log.Error("could not parse %s\n", buf)
	}
}

func (route *baseRoute) Key() string {
	return route.key
}

func (route *baseRoute) Match(s []byte) bool {
	conf := route.config.Load().(RouteConfig)
	return conf.Matcher().Match(s)
}

func (route *baseRoute) Flush() error {
	conf := route.config.Load().(RouteConfig)

	for _, d := range conf.Dests() {
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

	for _, d := range conf.Dests() {
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
func makeSnapshot(route *baseRoute, routeType string) RouteSnapshot {
	conf := route.config.Load().(RouteConfig)
	dests := make([]*Destination, len(conf.Dests()))
	for i, d := range conf.Dests() {
		dests[i] = d.Snapshot()
	}
	return RouteSnapshot{*conf.Matcher(), dests, routeType, route.key}

}

func (route *RouteSendAllMatch) Snapshot() RouteSnapshot {
	return makeSnapshot(&route.baseRoute, "sendAllMatch")
}

func (route *RouteSendFirstMatch) Snapshot() RouteSnapshot {
	return makeSnapshot(&route.baseRoute, "sendFirstMatch")
}

func (route *RouteConsistentHashing) Snapshot() RouteSnapshot {
	return makeSnapshot(&route.baseRoute, "consistentHashing")
}

// baseConfigExtender is a function that takes a baseRouteConfig and returns
// a configuration object that implements RouteConfig. This function may be
// the identity function, i.e., it may simply return its argument.
// This mechanism supports maintaining different configuration objects for
// different route types and creating a route-appropriate configuration object
// on a configuration change.
// The baseRoute object implements private methods (addDestination,
// delDestination, etc.) that create a new baseRouteConfig object that reflects
// the configuration change. This baseRouteConfig object is applicable to all
// route types. However, some route types, like ConsistentHashingRoute, have
// additional configuration and therefore have a distinct configuration object
// that embeds baseRouteConfig (in the case of ConsistentHashingRoute, the object
// is consistentHashingRouteConfig). This route-type-specific configuration also
// needs to be updated on a base configuration change (e.g., on a change
// affecting destinations). Accordingly, the public entry points that effect the
// configuration change (Add, DelDestination, etc.) are implemented for baseRoute
// and also for any route type, like ConsistentHashingRoute, that creates a
// configuration object. These public entry points call the private method,
// passing in a callback function of type baseConfigExtender, which takes a
// baseRouteConfig and either returns it unchanged or creates an outer
// configuration object with the baseRouteConfig embedded in it.
// The private method then stores the RouteConfig object returned by the callback.
type baseConfigExtender func(baseRouteConfig) RouteConfig

func baseRouteConfigExtender(baseConfig baseRouteConfig) RouteConfig {
	return baseConfig
}

func consistentHashingRouteConfigExtender(baseConfig baseRouteConfig) RouteConfig {
	hasher := NewConsistentHasher(baseConfig.Dests())
	return consistentHashingRouteConfig{baseConfig, &hasher}
}

// Add adds a new Destination to the Route and automatically runs it for you.
// The destination must not be running already!
func (route *baseRoute) addDestination(dest *Destination, extendConfig baseConfigExtender) {
	route.Lock()
	defer route.Unlock()
	conf := route.config.Load().(RouteConfig)
	dest.Run()
	newDests := append(conf.Dests(), dest)
	newConf := extendConfig(baseRouteConfig{*conf.Matcher(), newDests})
	route.config.Store(newConf)
}

func (route *baseRoute) Add(dest *Destination) {
	route.addDestination(dest, baseRouteConfigExtender)
}

func (route *RouteConsistentHashing) Add(dest *Destination) {
	route.addDestination(dest, consistentHashingRouteConfigExtender)
}

func (route *baseRoute) delDestination(index int, extendConfig baseConfigExtender) error {
	route.Lock()
	defer route.Unlock()
	conf := route.config.Load().(RouteConfig)
	if index >= len(conf.Dests()) {
		return fmt.Errorf("Invalid index %d", index)
	}
	conf.Dests()[index].Shutdown()
	newDests := append(conf.Dests()[:index], conf.Dests()[index+1:]...)
	newConf := extendConfig(baseRouteConfig{*conf.Matcher(), newDests})
	route.config.Store(newConf)
	return nil
}

func (route *baseRoute) DelDestination(index int) error {
	return route.delDestination(index, baseRouteConfigExtender)
}

func (route *RouteConsistentHashing) DelDestination(index int) error {
	return route.delDestination(index, consistentHashingRouteConfigExtender)
}

func (route *baseRoute) update(opts map[string]string, extendConfig baseConfigExtender) error {
	route.Lock()
	defer route.Unlock()
	conf := route.config.Load().(RouteConfig)
	matcher := conf.Matcher()
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
		conf = extendConfig(baseRouteConfig{*matcher, conf.Dests()})
	}
	route.config.Store(conf)
	return nil
}

func (route *baseRoute) Update(opts map[string]string) error {
	return route.update(opts, baseRouteConfigExtender)
}

func (route *RouteConsistentHashing) Update(opts map[string]string) error {
	return route.update(opts, consistentHashingRouteConfigExtender)
}

func (route *baseRoute) updateDestination(index int, opts map[string]string, extendConfig baseConfigExtender) error {
	route.Lock()
	defer route.Unlock()
	conf := route.config.Load().(RouteConfig)
	if index >= len(conf.Dests()) {
		return fmt.Errorf("Invalid index %d", index)
	}
	err := conf.Dests()[index].Update(opts)
	if err != nil {
		return err
	}
	conf = extendConfig(baseRouteConfig{*conf.Matcher(), conf.Dests()})
	route.config.Store(conf)
	return nil
}

func (route *baseRoute) UpdateDestination(index int, opts map[string]string) error {
	return route.updateDestination(index, opts, baseRouteConfigExtender)
}

func (route *RouteConsistentHashing) UpdateDestination(index int, opts map[string]string) error {
	return route.updateDestination(index, opts, consistentHashingRouteConfigExtender)
}

func (route *baseRoute) updateMatcher(matcher Matcher, extendConfig baseConfigExtender) {
	route.Lock()
	defer route.Unlock()
	conf := route.config.Load().(RouteConfig)
	conf = extendConfig(baseRouteConfig{matcher, conf.Dests()})
	route.config.Store(conf)
}

func (route *baseRoute) UpdateMatcher(matcher Matcher) {
	route.updateMatcher(matcher, baseRouteConfigExtender)
}

func (route *RouteConsistentHashing) UpdateMatcher(matcher Matcher) {
	route.updateMatcher(matcher, consistentHashingRouteConfigExtender)
}
