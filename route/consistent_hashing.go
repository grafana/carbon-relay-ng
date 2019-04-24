package route

import (
	"bytes"
	"fmt"

	dest "github.com/graphite-ng/carbon-relay-ng/destination"
	"github.com/graphite-ng/carbon-relay-ng/matcher"
	"github.com/serialx/hashring"
	log "github.com/sirupsen/logrus"
)

type ConsistentHashing struct {
	baseRoute
	Ring *hashring.HashRing
}

func NewConsistentHashing(key, prefix, sub, regex string, destinations []*dest.Destination) (*ConsistentHashing, error) {
	m, err := matcher.New(prefix, sub, regex)
	if err != nil {
		return nil, err
	}
	ring := hashring.New(nil)
	r := &ConsistentHashing{*newBaseRoute(key, "ConsistentHashing"), ring}
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

func (cs *ConsistentHashing) GetDestinationForName(name []byte) (*dest.Destination, error) {
	var ok bool
	var dName string
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

func (cs *ConsistentHashing) Dispatch(buf []byte) {
	if pos := bytes.IndexByte(buf, ' '); pos > 0 {
		name := buf[0:pos]
		dest, err := cs.GetDestinationForName(name)
		if err != nil {
			log.Errorf("can't process metric `%s`: %s", name, err)
			return
		}
		// dest should handle this as quickly as it can
		log.Tracef("route %s sending to dest %s: %s", cs.key, dest.Key, name)
		dest.In <- buf
	} else {
		log.Errorf("could not parse %s", buf)
	}
}

func (route *ConsistentHashing) Snapshot() Snapshot {
	return makeSnapshot(&route.baseRoute)
}
