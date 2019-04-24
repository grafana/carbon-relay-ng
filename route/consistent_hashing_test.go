package route

import (
	"fmt"
	"testing"

	"github.com/graphite-ng/carbon-relay-ng/destination"

	"github.com/stretchr/testify/assert"

	"github.com/serialx/hashring"
)

func testAddr(i int) string {
	return fmt.Sprintf("127.0.0.1:%d", i)
}

func testBaseCHRoute(nodeNum int) *ConsistentHashing {
	var nodes []string
	destMap := map[string]*destination.Destination{}
	for i := 0; i < nodeNum; i++ {
		addr := testAddr(i)
		nodes = append(nodes, addr)
		destMap[addr] = &destination.Destination{Key: addr}
	}
	r := &ConsistentHashing{*newBaseRoute("test_route", "ConsistentHashing"), hashring.New(nodes)}
	r.baseRoute.destMap = destMap
	return r
}

func benchmarkCH(nodeNum, keyLen int, b *testing.B) {
	nuller := make(chan []byte, 100)
	stop := make(chan struct{})
	go func() {
		for {
			select {
			case _ = <-stop:
				return
			case _ = <-nuller:
			}
		}
	}()
	chRoute := testBaseCHRoute(nodeNum)
	keyName := make([]byte, keyLen)
	for i := range keyName {
		keyName[i] = byte('0' + i%10)
	}
	for i := 0; i < b.N; i++ {
		keyName[(i/8)%len(keyName)] = byte('0' + (i % 10))
		chRoute.GetDestinationForName(keyName)
	}
	close(stop)
}
func BenchmarkCH10D50B(b *testing.B) {
	benchmarkCH(10, 50, b)
}
func BenchmarkCH100D150B(b *testing.B) {
	benchmarkCH(100, 150, b)
}

func TestConsistentProperlyRoute(t *testing.T) {
	chRoute := testBaseCHRoute(10)

	key := "omelette_du_fromage"
	n, ok := chRoute.Ring.GetNode(key)
	assert.Equal(t, ok, true)
	refKey := n
	chRoute.Ring = chRoute.Ring.RemoveNode(refKey)
	n, ok = chRoute.Ring.GetNode(key)
	assert.Equal(t, ok, true)
	assert.NotEqual(t, n, refKey)
	chRoute.Ring = chRoute.Ring.AddNode(refKey)
	n, ok = chRoute.Ring.GetNode(key)
	assert.Equal(t, ok, true)
	assert.Equal(t, n, refKey)
}
