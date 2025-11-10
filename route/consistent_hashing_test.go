package route

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/grafana/carbon-relay-ng/destination"
)

func TestConsistentHashingComputeRingPosition(t *testing.T) {
	assert.Equal(t, uint16(54437), computeRingPosition([]byte("a.b.c.d")))
	assert.Equal(t, uint16(54301), computeRingPosition([]byte("")))
}

func TestIssue335(t *testing.T) {
	initialDestinations := []*destination.Destination{
		{Addr: "10.20.34.114:12003"},
		{Addr: "10.20.39.104:12003"},
		{Addr: "10.20.40.161:12003"},
		{Addr: "10.20.35.158:12003"},
		{Addr: "10.20.37.70:12003"},
		{Addr: "10.20.40.126:12003"},
		{Addr: "10.20.33.78:12003"},
		{Addr: "10.20.39.19:12003"},
		{Addr: "10.20.42.66:12003"},
		{Addr: "10.20.34.131:12003"},
		{Addr: "10.20.38.55:12003"},
		{Addr: "10.20.41.75:12003"},
		{Addr: "10.20.32.8:12003"},
		{Addr: "10.20.37.165:12003"},
	}
	hasherWithoutFix := NewConsistentHasherReplicaCount(initialDestinations, 2, false)
	hasherWithFix := NewConsistentHasherReplicaCount(initialDestinations, 2, true)
	fmt.Println("len without fix and with fix - should be different", len(hasherWithoutFix.Ring), len(hasherWithFix.Ring))
	for _, v := range hasherWithFix.Ring {
		if v.Position == 59418 {
			fmt.Println("found our missing value!") // this should trigger
		}
		fmt.Printf("%+v\n", v)
	}
}

func TestConsistentHashingDestinations(t *testing.T) {
	initialDestinations := []*destination.Destination{
		{Addr: "10.0.0.1"},
		{Addr: "127.0.0.1:2003", Instance: "a"},
		{Addr: "127.0.0.1:2004", Instance: "b"}}
	hasher := NewConsistentHasherReplicaCount(initialDestinations, 2, false)
	expectedHashRing := hashRing{
		hashRingEntry{Position: uint16(7885), Hostname: "127.0.0.1", Instance: "a", DestinationIndex: 1},
		hashRingEntry{Position: uint16(10461), Hostname: "127.0.0.1", Instance: "b", DestinationIndex: 2},
		hashRingEntry{Position: uint16(24043), Hostname: "127.0.0.1", Instance: "a", DestinationIndex: 1},
		hashRingEntry{Position: uint16(35540), Hostname: "10.0.0.1", DestinationIndex: 0},
		hashRingEntry{Position: uint16(46982), Hostname: "10.0.0.1", DestinationIndex: 0},
		hashRingEntry{Position: uint16(54295), Hostname: "127.0.0.1", Instance: "b", DestinationIndex: 2}}
	assert.Equal(t, expectedHashRing, hasher.Ring)
	hasher.AddDestination(&destination.Destination{Addr: "127.0.0.1", Instance: "c"})
	hasher.AddDestination(&destination.Destination{Addr: "10.0.0.2"})
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
