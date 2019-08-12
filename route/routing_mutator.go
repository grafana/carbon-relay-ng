package route

import (
	"fmt"
	"regexp"
	"sort"
	"sync"

	"github.com/VictoriaMetrics/fastcache"
)

type Mutator struct {
	Matcher *regexp.Regexp
	Output  []byte
}

func (m Mutator) MutateMaybeBuf(dst, buf []byte) (out []byte) {
	// Not sure if optimized, but hey..
	matches := m.Matcher.FindSubmatchIndex(buf)
	if matches != nil {
		// Sadly expand is still converting our template to string
		return m.Matcher.Expand(dst, m.Output, buf, matches)
	}
	return dst
}

type RoutingMutator struct {
	sync.RWMutex
	Table []*Mutator
	cache *fastcache.Cache
	pool  *sync.Pool
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

	// var cache *freecache.Cache
	var cache *fastcache.Cache
	if cacheSize > 0 {
		cache = fastcache.New(cacheSize)
	}
	return &RoutingMutator{
		sync.RWMutex{}, mutators, cache, &sync.Pool{New: func() interface{} {
			return make([]byte, 0, 100)
		}},
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
	ret := rm.pool.Get().([]byte)
	defer func() {
		if len(ret) < 300 {
			ret = ret[:0]
			rm.pool.Put(ret)
		}
	}()
	if rm.cache != nil {
		ret = rm.cache.Get(ret, bufKey)
		if len(ret) > 0 {
			return ret, true
		}
	}
	ret = rm.mutateMaybe(ret, bufKey)
	if rm.cache != nil {
		rm.cache.Set(bufKey, ret)
	}
	if len(ret) == 0 {
		return nil, false
	}
	return ret, true
}
func (rm *RoutingMutator) mutateMaybe(dst []byte, key []byte) []byte {
	if dst == nil {
		dst = []byte{}
	}
	for i := 0; i < len(rm.Table) && len(dst) == 0; i++ {
		dst = rm.Table[i].MutateMaybeBuf(dst, key)
	}
	return dst
}
