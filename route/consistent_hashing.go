package route

import (
	"fmt"

	"go.uber.org/zap"

	"github.com/graphite-ng/carbon-relay-ng/encoding"

	dest "github.com/graphite-ng/carbon-relay-ng/destination"
	"github.com/graphite-ng/carbon-relay-ng/matcher"
	"github.com/serialx/hashring"
)

type ConsistentHashing struct {
	baseRoute
	Ring    *hashring.HashRing
	Mutator *RoutingMutator
}

func NewConsistentHashing(key, prefix, sub, regex string, destinations []*dest.Destination, routingMutator *RoutingMutator) (*ConsistentHashing, error) {
	m, err := matcher.New(prefix, sub, regex)
	if err != nil {
		return nil, err
	}
	ring := hashring.New(nil)
	r := &ConsistentHashing{
		*newBaseRoute(key, "ConsistentHashing"),
		ring,
		routingMutator,
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
