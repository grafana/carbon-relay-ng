package route

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/graphite-ng/carbon-relay-ng/encoding"
	"go.uber.org/zap"

	dest "github.com/graphite-ng/carbon-relay-ng/destination"
	"github.com/graphite-ng/carbon-relay-ng/matcher"
	"github.com/graphite-ng/carbon-relay-ng/metrics"
)

// numDropBuffFull       metrics.Counter   // metric drops due to queue full
// durationTickFlush     metrics.Timer     // only updated after successful flush
// tickFlushSize         metrics.Histogram // only updated after successful flush
// numBuffered           metrics.Gauge
// bufferSize            metrics.Gauge

type Config interface {
	Matcher() *matcher.Matcher
	Dests() []*dest.Destination
}

type baseConfig struct {
	matcher matcher.Matcher
	dests   []*dest.Destination
}

func (c baseConfig) Matcher() *matcher.Matcher {
	return &c.matcher
}

func (c baseConfig) Dests() []*dest.Destination {
	return c.dests
}

type Route interface {
	Dispatch(encoding.Datapoint)
	Match(s []byte) bool
	MatchString(s string) bool
	Snapshot() Snapshot
	Key() string
	Type() string
	Flush() error
	Shutdown() error
	GetDestination(index int) (*dest.Destination, error)
	GetDestinations() []*dest.Destination
	DelDestination(index int) error
	UpdateDestination(index int, opts map[string]string) error
	Update(opts map[string]string) error
}

type Snapshot struct {
	Matcher matcher.Matcher     `json:"matcher"`
	Dests   []*dest.Destination `json:"destination"`
	Type    string              `json:"type"`
	Key     string              `json:"key"`
	Addr    string              `json:"addr,omitempty"`
}

type baseRoute struct {
	sync.Mutex              // only needed for the multiple writers
	config     atomic.Value // for reading and writing

	key       string
	routeType string
	rm        *metrics.RouteMetrics
	destMap   map[string]*dest.Destination
	logger    *zap.Logger
}

func newBaseRoute(key, routeType string) *baseRoute {
	return &baseRoute{
		sync.Mutex{},
		atomic.Value{},
		key,
		routeType,
		metrics.NewRouteMetrics(key, routeType, nil),
		map[string]*dest.Destination{},
		zap.L().With(zap.String("routekey", key), zap.String("route_type", routeType)),
	}
}

type SendAllMatch struct {
	baseRoute
}

type SendFirstMatch struct {
	baseRoute
}

// NewSendAllMatch creates a sendAllMatch route.
// We will automatically run the route and the given destinations
func NewSendAllMatch(key, prefix, sub, regex string, destinations []*dest.Destination) (Route, error) {
	m, err := matcher.New(prefix, sub, regex)
	if err != nil {
		return nil, err
	}
	r := &SendAllMatch{*newBaseRoute(key, "SendAllMatch")}
	r.config.Store(baseConfig{*m, destinations})
	r.run()
	return r, nil
}

// NewSendFirstMatch creates a sendFirstMatch route.
// We will automatically run the route and the given destinations
func NewSendFirstMatch(key, prefix, sub, regex string, destinations []*dest.Destination) (Route, error) {
	m, err := matcher.New(prefix, sub, regex)
	if err != nil {
		return nil, err
	}
	r := &SendFirstMatch{*newBaseRoute(key, "SendFirstMatch")}
	r.config.Store(baseConfig{*m, destinations})
	r.run()
	return r, nil
}

func (route *baseRoute) run() {
	for _, d := range route.GetDestinations() {
		d.Run()
	}
}

func (route *SendAllMatch) Dispatch(d encoding.Datapoint) {
	conf := route.config.Load().(Config)

	for _, dest := range conf.Dests() {
		if dest.MatchString(d.Name) {
			// dest should handle this as quickly as it can
			route.logger.Debug("route sending to dest", zap.String("destinationKey", dest.Key), zap.Stringer("datapoint", d))
			dest.In <- d
			route.rm.OutMetrics.Inc()
		}
	}
}

func (route *SendFirstMatch) Dispatch(d encoding.Datapoint) {
	conf := route.config.Load().(Config)

	for _, dest := range conf.Dests() {
		if dest.MatchString(d.Name) {
			// dest should handle this as quickly as it can
			zap.L().Debug("route %s sending to dest %s: %v", zap.String("destinationKey", dest.Key), zap.Stringer("datapoint", d))
			dest.In <- d
			route.rm.OutMetrics.Inc()
			break
		}
	}
}

func (route *baseRoute) Key() string {
	return route.key
}

func (route *baseRoute) Type() string {
	return route.routeType
}

func (route *baseRoute) MatchString(s string) bool {
	conf := route.config.Load().(Config)
	return conf.Matcher().MatchString(s)
}

func (route *baseRoute) Match(s []byte) bool {
	conf := route.config.Load().(Config)
	return conf.Matcher().Match(s)
}

func (route *baseRoute) Flush() error {
	conf := route.config.Load().(Config)

	for _, d := range conf.Dests() {
		err := d.Flush()
		if err != nil {
			return err
		}
	}
	return nil
}

func (route *baseRoute) Shutdown() error {
	conf := route.config.Load().(Config)

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
func makeSnapshot(route *baseRoute) Snapshot {
	conf := route.config.Load().(Config)
	dests := make([]*dest.Destination, len(conf.Dests()))
	for i, d := range conf.Dests() {
		dests[i] = d.Snapshot()
	}
	return Snapshot{Matcher: *conf.Matcher(), Dests: dests, Type: route.routeType, Key: route.key}
}

func (route *SendAllMatch) Snapshot() Snapshot {
	return makeSnapshot(&route.baseRoute)
}

func (route *SendFirstMatch) Snapshot() Snapshot {
	return makeSnapshot(&route.baseRoute)
}

// baseCfgExtender is a function that takes a baseConfig and returns
// a configuration object that implements Config. This function may be
// the identity function, i.e., it may simply return its argument.
// This mechanism supports maintaining different configuration objects for
// different route types and creating a route-appropriate configuration object
// on a configuration change.
// The baseRoute object implements private methods (addDestination,
// delDestination, etc.) that create a new baseConfig object that reflects
// the configuration change. This baseConfig object is applicable to all
// route types. However, some route types, like ConsistentHashingRoute, have
// additional configuration and therefore have a distinct configuration object
// that embeds baseConfig (in the case of ConsistentHashingRoute, the object
// is consistentHashingConfig). This route-type-specific configuration also
// needs to be updated on a base configuration change (e.g., on a change
// affecting destinations). Accordingly, the public entry points that effect the
// configuration change (Add, DelDestination, etc.) are implemented for baseRoute
// and also for any route type, like ConsistentHashingRoute, that creates a
// configuration object. These public entry points call the private method,
// passing in a callback function of type baseCfgExtender, which takes a
// baseConfig and either returns it unchanged or creates an outer
// configuration object with the baseConfig embedded in it.
// The private method then stores the Config object returned by the callback.
type baseCfgExtender func(baseConfig) Config

func baseConfigExtender(baseConfig baseConfig) Config {
	return baseConfig
}

// Add adds a new Destination to the Route and automatically runs it for you.
// The destination must not be running already!
func (route *baseRoute) addDestination(dest *dest.Destination, extendConfig baseCfgExtender) {
	route.Lock()
	defer route.Unlock()
	conf := route.config.Load().(Config)
	dest.Run()
	newDests := append(conf.Dests(), dest)
	newConf := extendConfig(baseConfig{*conf.Matcher(), newDests})
	route.destMap[dest.Key] = dest
	route.config.Store(newConf)
}

func (route *baseRoute) Add(dest *dest.Destination) {
	route.addDestination(dest, baseConfigExtender)
}

func (route *baseRoute) delDestination(index int, extendConfig baseCfgExtender) error {
	route.Lock()
	defer route.Unlock()
	conf := route.config.Load().(Config)
	if index >= len(conf.Dests()) {
		return fmt.Errorf("Invalid index %d", index)
	}
	d := conf.Dests()[index]
	d.Shutdown()
	newDests := append(conf.Dests()[:index], conf.Dests()[index+1:]...)
	newConf := extendConfig(baseConfig{*conf.Matcher(), newDests})
	delete(route.destMap, d.Key)
	route.config.Store(newConf)
	return nil
}

func (route *baseRoute) DelDestination(index int) error {
	return route.delDestination(index, baseConfigExtender)
}

func (route *baseRoute) GetDestinations() []*dest.Destination {
	conf := route.config.Load().(Config)
	return conf.Dests()
}

func (route *baseRoute) GetDestinationByName(destName string) (*dest.Destination, error) {
	route.Lock()
	defer route.Unlock()

	d, ok := route.destMap[destName]
	if !ok {
		return nil, fmt.Errorf("Destination not found %s", destName)
	}
	return d, nil
}

func (route *baseRoute) GetDestination(index int) (*dest.Destination, error) {
	route.Lock()
	defer route.Unlock()
	conf := route.config.Load().(Config)
	if index >= len(conf.Dests()) {
		return nil, fmt.Errorf("Invalid index %d", index)
	}
	return conf.Dests()[index], nil
}

func (route *baseRoute) update(opts map[string]string, extendConfig baseCfgExtender) error {
	route.Lock()
	defer route.Unlock()
	conf := route.config.Load().(Config)
	match := conf.Matcher()
	prefix := match.Prefix
	sub := match.Sub
	regex := match.Regex
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
		match, err := matcher.New(prefix, sub, regex)
		if err != nil {
			return err
		}
		conf = extendConfig(baseConfig{*match, conf.Dests()})
	}
	route.config.Store(conf)
	return nil
}

func (route *baseRoute) Update(opts map[string]string) error {
	return route.update(opts, baseConfigExtender)
}

func (route *baseRoute) updateDestination(index int, opts map[string]string, extendConfig baseCfgExtender) error {
	route.Lock()
	defer route.Unlock()
	conf := route.config.Load().(Config)
	if index >= len(conf.Dests()) {
		return fmt.Errorf("Invalid index %d", index)
	}
	err := conf.Dests()[index].Update(opts)
	if err != nil {
		return err
	}
	conf = extendConfig(baseConfig{*conf.Matcher(), conf.Dests()})
	route.config.Store(conf)
	return nil
}

func (route *baseRoute) UpdateDestination(index int, opts map[string]string) error {
	return route.updateDestination(index, opts, baseConfigExtender)
}

func (route *baseRoute) updateMatcher(matcher matcher.Matcher, extendConfig baseCfgExtender) {
	route.Lock()
	defer route.Unlock()
	conf := route.config.Load().(Config)
	conf = extendConfig(baseConfig{matcher, conf.Dests()})
	route.config.Store(conf)
}

func (route *baseRoute) UpdateMatcher(matcher matcher.Matcher) {
	route.updateMatcher(matcher, baseConfigExtender)
}
