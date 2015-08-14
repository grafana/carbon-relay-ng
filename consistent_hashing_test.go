package main

import (
	"testing"

	"github.com/graphite-ng/carbon-relay-ng/_third_party/github.com/bmizerany/assert"
)

func TestConsistentHashingComputeRingPosition(t *testing.T) {
	assert.Equal(t, uint16(54437), computeRingPosition([]byte("a.b.c.d")))
	assert.Equal(t, uint16(54301), computeRingPosition([]byte("")))
}

func TestConsistentHashingDestinations(t *testing.T) {
	initialDestinations := []*Destination{
		&Destination{Addr: "10.0.0.1"},
		&Destination{Addr: "127.0.0.1:2003", Instance: "a"},
		&Destination{Addr: "127.0.0.1:2004", Instance: "b"}}
	hasher := NewConsistentHasherReplicaCount(initialDestinations, 2)
	expectedHashRing := hashRing{
		hashRingEntry{Position: uint16(7885), Hostname: "127.0.0.1", Instance: "a", DestinationIndex: 1},
		hashRingEntry{Position: uint16(10461), Hostname: "127.0.0.1", Instance: "b", DestinationIndex: 2},
		hashRingEntry{Position: uint16(24043), Hostname: "127.0.0.1", Instance: "a", DestinationIndex: 1},
		hashRingEntry{Position: uint16(35540), Hostname: "10.0.0.1", DestinationIndex: 0},
		hashRingEntry{Position: uint16(46982), Hostname: "10.0.0.1", DestinationIndex: 0},
		hashRingEntry{Position: uint16(54295), Hostname: "127.0.0.1", Instance: "b", DestinationIndex: 2}}
	assert.Equal(t, expectedHashRing, hasher.Ring)
	hasher.AddDestination(&Destination{Addr: "127.0.0.1", Instance: "c"})
	hasher.AddDestination(&Destination{Addr: "10.0.0.2"})
	expectedHashRing = hashRing{
		hashRingEntry{Position: uint16(6639), Hostname: "10.0.0.2", DestinationIndex: 4},
		hashRingEntry{Position: uint16(7885), Hostname: "127.0.0.1", Instance: "a", DestinationIndex: 1},
		hashRingEntry{Position: uint16(10461), Hostname: "127.0.0.1", Instance: "b", DestinationIndex: 2},
		hashRingEntry{Position: uint16(24043), Hostname: "127.0.0.1", Instance: "a", DestinationIndex: 1},
		hashRingEntry{Position: uint16(24467), Hostname: "127.0.0.1", Instance: "c", DestinationIndex: 3},
		hashRingEntry{Position: uint16(35540), Hostname: "10.0.0.1", DestinationIndex: 0},
		hashRingEntry{Position: uint16(46982), Hostname: "10.0.0.1", DestinationIndex: 0},
		hashRingEntry{Position: uint16(52177), Hostname: "10.0.0.2", DestinationIndex: 4},
		hashRingEntry{Position: uint16(53472), Hostname: "127.0.0.1", Instance: "c", DestinationIndex: 3},
		hashRingEntry{Position: uint16(54295), Hostname: "127.0.0.1", Instance: "b", DestinationIndex: 2}}
	assert.Equal(t, expectedHashRing, hasher.Ring)
	assert.Equal(t, 4, hasher.GetDestinationIndex([]byte("a.b.c.d")))
	assert.Equal(t, 1, hasher.GetDestinationIndex([]byte("a.b.c..d")))
	assert.Equal(t, 3, hasher.GetDestinationIndex([]byte("collectd.bar.memory.free")))
}
