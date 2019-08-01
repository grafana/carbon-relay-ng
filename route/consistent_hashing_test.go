package route

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/valyala/fastrand"

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
	rm, _ := NewRoutingMutator(nil, 0)
	r := &ConsistentHashing{*newBaseRoute("test_route", "ConsistentHashing"), hashring.New(nodes), rm}
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

func TestRoutingMutatorFailOnIncorrectRegexp(t *testing.T) {
	_, err := NewRoutingMutator(map[string]string{"(123": "fail !"}, 0)
	assert.Error(t, err)
}

func TestConsistentHonorMutation(t *testing.T) {
	chRoute := testBaseCHRoute(10)

	routing, err := NewRoutingMutator(map[string]string{
		"test":             "toto",
		".+-(camembert)":   "$1",
		"omelette-du-(.+)": "roblochon",
	}, 0)
	if err != nil {
		t.Fatalf("error creating the routing mutator: %s", err)
	}
	chRoute.Mutator = routing

	destTest, err := chRoute.GetDestinationForName([]byte("test"))
	assert.Nil(t, err)
	destToto, err := chRoute.GetDestinationForName([]byte("toto"))
	assert.Nil(t, err)
	assert.Equal(t, destTest, destToto)

	destOmeletteCamembert, err := chRoute.GetDestinationForName([]byte("omelette-du-camembert"))
	assert.Nil(t, err)

	destTartine, err := chRoute.GetDestinationForName([]byte("tartine-au-camembert"))
	assert.Nil(t, err)
	destOmeletteEpoisse, err := chRoute.GetDestinationForName([]byte("omelette-du-epoisse"))
	assert.Nil(t, err)
	destOmeletteLangre, err := chRoute.GetDestinationForName([]byte("omelette-du-langre"))
	assert.Nil(t, err)

	assert.Equal(t, destTartine, destOmeletteCamembert)
	assert.Equal(t, destOmeletteEpoisse, destOmeletteLangre)
}

func TestConsistentOverallDistribution(t *testing.T) {
	type DistributionTest struct {
		Entries      int
		Nodes        int
		MaxDiffRatio float64
	}
	distibTests := []DistributionTest{
		{1000000, 100, 2},
		{1000000, 1000, 2},
		{1000000, 10, 1.5},
	}

	for _, test := range distibTests {
		t.Run(fmt.Sprintf("%dEntries%dNodes%fMaxRatio", test.Entries, test.Nodes, test.MaxDiffRatio),
			func(t *testing.T) {
				// t.Parallel()
				n := test.Entries

				chRoute := testBaseCHRoute(test.Nodes)

				dstMap := map[string]int{}

				for i := 0; i < n; i++ {
					d, err := chRoute.GetDestinationForNameString(strconv.Itoa(i))
					assert.Nil(t, err)
					dN, ok := dstMap[d.Key]
					if !ok {
						dN = 0
					}
					dstMap[d.Key] = dN + 1
				}

				min := n
				max := 0
				for _, hits := range dstMap {
					if min >= hits {
						min = hits
					}
					if max <= hits {
						max = hits
					}
				}
				t.Log("Max:", max, "Min:", min)
				t.Log("Ratio:", float64(max)/float64(min))
				assert.True(t, float64(max) < test.MaxDiffRatio*float64(min))
			},
		)
	}
}

func TestConsistentMinimalDistribution(t *testing.T) {
	chRoute := testBaseCHRoute(1000)

	destTest, err := chRoute.GetDestinationForName([]byte("test"))
	assert.Nil(t, err)
	destToto, err := chRoute.GetDestinationForName([]byte("toto"))
	assert.Nil(t, err)
	assert.NotEqual(t, destTest, destToto)
}

func benchRoutingMutation(b *testing.B, cached bool, randKey bool) {
	var cacheSize = 0
	if cached {
		cacheSize = 1024 * 1024 * 100
	}

	rm, err := NewRoutingMutator(map[string]string{
		"test\\.(.*)": "$1",
	}, cacheSize)
	if err != nil {
		b.Fatalf("can't init routing mutator: %s", err)
	}
	key := []byte("test.")
	for i := 0; i < 90; i++ {
		key = append(key, 'a')
	}
	mutKey := key[5:]
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if randKey {
			mutKey[int(fastrand.Uint32())%len(mutKey)] = byte('0' + i%20)
		}
		rm.HandleBuf(key)
	}
}

func BenchmarkRoutingMutation(b *testing.B) {
	type Case struct {
		Rand  bool
		Cache bool
	}
	cases := []Case{
		{false, false},
		{true, false},
		{false, true},
		{true, true},
	}
	for _, c := range cases {
		b.Run(fmt.Sprintf("Cache%sRand%s", strings.Title(fmt.Sprint(c.Cache)), strings.Title(fmt.Sprint(c.Rand))),
			func(b *testing.B) {
				benchRoutingMutation(b, c.Cache, c.Rand)
			})
	}
}
