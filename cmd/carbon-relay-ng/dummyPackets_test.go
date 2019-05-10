package main

// this generates, and provides acces to a bunch of generated metrics/packets
// it does this using a single backing array to save memory allocations

import (
	"fmt"

	"github.com/graphite-ng/carbon-relay-ng/encoding"
)

// so that whatever the timestamp is, if we add it to this, it will always use same amount
// of chars. in other words, don't use that many DP's that this statement would no longer be true!
const tsBase = 1000000000

type dummyPackets struct {
	key    string
	points []encoding.Datapoint
}

func NewDummyPackets(key string, amount int) *dummyPackets {
	tpl := "%s.dummyPacket"
	val := 123.0
	ts := tsBase + 1
	points := make([]encoding.Datapoint, 0, amount)
	for i := 0; i < len(points); i++ {
		points = append(points, encoding.Datapoint{
			Name:      fmt.Sprintf(tpl, key),
			Value:     val,
			Timestamp: uint64(ts + i),
		})
	}

	return &dummyPackets{key, points}
}

func (dp *dummyPackets) Len() int {
	return len(dp.points)
}

func (dp *dummyPackets) Get(i int) encoding.Datapoint {
	if i >= len(dp.points) {
		panic("can't ask for higher index then what we have in dummyPackets")
	}
	return dp.points[i]
}

func (dp *dummyPackets) All() chan encoding.Datapoint {
	ret := make(chan encoding.Datapoint, 10000) // pretty arbitrary, but seems to help perf
	go func() {
		for i := 0; i < len(dp.points); i++ {
			ret <- dp.points[i]
		}
		close(ret)
	}()
	return ret
}

func mergeAll(in ...chan encoding.Datapoint) chan encoding.Datapoint {
	ret := make(chan encoding.Datapoint, 10000) // pretty arbitrary, but seems to help perf
	go func() {
		for _, inChan := range in {
			for val := range inChan {
				ret <- val
			}
		}
		close(ret)
	}()
	return ret
}
