package route

import (
	"fmt"
	"regexp"
	"sort"
	"sync"

	"go.uber.org/zap"

	"github.com/graphite-ng/carbon-relay-ng/encoding"

	"github.com/coocood/freecache"

	dest "github.com/graphite-ng/carbon-relay-ng/destination"
	"github.com/graphite-ng/carbon-relay-ng/matcher"
	"github.com/serialx/hashring"
)

type Mutator struct {
	Matcher *regexp.Regexp
	Output  []byte
}

func (m Mutator) MutateMaybe(buf []byte) (out []byte) {
	// Not sure if optimized, but hey..
	matches := m.Matcher.FindSubmatchIndex(buf)
	if matches != nil {
		// Sadly expand is still converting our template to string
		out = m.Matcher.Expand(out, m.Output, buf, matches)
	}
	return out
}

type RoutingMutator struct {
	sync.RWMutex
	Table []*Mutator
	cache *freecache.Cache
}

func NewRoutingMutator(table map[string]string, cacheSize int) (*RoutingMutator, error) {
	mutators := []*Mutator{}
	if table != nil {
		for m, out := range table {
			re, err := regexp.Compile(m)
			if err != nil {
				return nil, fmt.Errorf("can't compile matcher `%s` to a valid regex: %s", m, err)
			}
			mutators = append(mutators, &Mutator{re, []byte(out)})
		}
	}
	sort.SliceStable(mutators, func(i, j int) bool {
		return mutators[i].Matcher.String() < mutators[j].Matcher.String()
	})

	var cache *freecache.Cache
	if cacheSize > 0 {
		cache = freecache.NewCache(cacheSize)
	}
	return &RoutingMutator{
		sync.RWMutex{}, mutators, cache,
	}, nil
}

func (rm *RoutingMutator) HandleString(key string) (string, bool) {
	routingKey, ok := rm.HandleBuf([]byte(key))
	if routingKey == nil || !ok {
		return "", false
	}
	return string(routingKey), ok
}

func (rm *RoutingMutator) HandleBuf(bufKey []byte) ([]byte, bool) {
	if rm.cache != nil {
		cachedKey, err := rm.cache.Get(bufKey)
		if err == nil {
			// Cache Hit !
			if cachedKey == nil {
				return nil, false
			}
			return cachedKey, true
		}
	}
	new := rm.mutateMaybe(bufKey)
	if rm.cache != nil {
		rm.cache.Set(bufKey, new, 0)
	}
	if new == nil {
		return nil, false
	}
	return new, true
}

func (rm *RoutingMutator) mutateMaybe(key []byte) []byte {
	var out []byte
	for i := 0; i < len(rm.Table) && out == nil; i++ {
		if out = rm.Table[i].MutateMaybe(key); out != nil {
			break
		}
	}
	return out
}

type ConsistentHashing struct {
	baseRoute
	Ring    *hashring.HashRing
	Mutator *RoutingMutator
}

func NewConsistentHashing(key, prefix, sub, regex string, destinations []*dest.Destination, routingMutations map[string]string, cacheSize int) (*ConsistentHashing, error) {
	m, err := matcher.New(prefix, sub, regex)
	if err != nil {
		return nil, err
	}
	ring := hashring.New(nil)
	routeMutator, err := NewRoutingMutator(routingMutations, cacheSize)
	if err != nil {
		return nil, fmt.Errorf("can't create the routing mutator: %s", err)
	}
	r := &ConsistentHashing{
		*newBaseRoute(key, "ConsistentHashing"),
		ring,
		routeMutator,
	}
	r.config.Store(baseConfig{*m, destinations})
	for _, dest := range destinations {
		r.Add(dest)
	}
	return r, nil
}

func (cs *ConsistentHashing) Add(d *dest.Destination) {
	cs.Ring = cs.Ring.AddNode(d.Key)
	cs.baseRoute.Add(d)
}

func (cs *ConsistentHashing) DelDestination(index int) error {
	d, err := cs.GetDestination(index)
	if err != nil {
		return err
	}
	cs.baseRoute.DelDestination(index)
	cs.Lock()
	defer cs.Unlock()
	cs.Ring = cs.Ring.RemoveNode(d.Key)
	return nil
}

func (cs *ConsistentHashing) GetDestinationForNameString(name string) (*dest.Destination, error) {
	var ok bool
	var dName string
	newName, mutated := cs.Mutator.HandleString(name)
	if mutated {
		name = newName
	}
	dName, ok = cs.Ring.GetNode(name)
	if !ok {
		return nil, fmt.Errorf("can't generate a consistent key for %s. ring is empty", name)
	}
	d, err := cs.GetDestinationByName(dName)
	if err != nil {
		return nil, fmt.Errorf("can't find a destination %s with metric: %s", dName, name)
	}
	return d, nil
}

func (cs *ConsistentHashing) GetDestinationForName(name []byte) (*dest.Destination, error) {
	var ok bool
	var dName string
	newName, mutated := cs.Mutator.HandleBuf(name)
	if mutated {
		name = newName
	}
	dName, ok = cs.Ring.GetNode(string(name))
	if !ok {
		return nil, fmt.Errorf("can't generate a consistent key for %s. ring is empty", name)
	}
	d, err := cs.GetDestinationByName(dName)
	if err != nil {
		return nil, fmt.Errorf("can't find a destination %s with metric: %s", dName, name)
	}
	return d, nil
}

func (cs *ConsistentHashing) Dispatch(dp encoding.Datapoint) {
	dest, err := cs.GetDestinationForNameString(dp.Name)
	if err != nil {
		cs.logger.Error("can't process metric", zap.String("metricName", dp.Name), zap.Error(err))
		return
	}
	// dest should handle this as quickly as it can
	cs.logger.Debug("route sending to dest",
		zap.String("destinationKey", dest.Key),
		zap.String("metricName", dp.Name))
	dest.In <- dp
	cs.baseRoute.rm.OutMetrics.Inc()
}

func (route *ConsistentHashing) Snapshot() Snapshot {
	return makeSnapshot(&route.baseRoute)
}
