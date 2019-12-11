package route

import (
	"testing"

	"github.com/graphite-ng/carbon-relay-ng/destination"
	"github.com/graphite-ng/carbon-relay-ng/encoding"
)

// just sending into route, no matching or sending to dest
func BenchmarkRouteDispatchMetric(b *testing.B) {
	route, err := NewSendAllMatch("", "", "", "", make([]*destination.Destination, 0))
	if err != nil {
		b.Fatal(err)
	}

	metric70 := []byte("abcde_fghij.klmnopqrst.uv_wxyz.1234567890abcdefg 12345.6789 1234567890") // key = 48, val = 10, ts = 10 -> 70
	dp, _ := encoding.NewPlain(false).Load(metric70, make(encoding.Tags))
	for i := 0; i < b.N; i++ {
		route.Dispatch(dp)
	}
}
