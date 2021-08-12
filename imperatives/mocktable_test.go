package imperatives

import (
	"github.com/grafana/carbon-relay-ng/aggregator"
	"github.com/grafana/carbon-relay-ng/matcher"
	"github.com/grafana/carbon-relay-ng/rewriter"
	"github.com/grafana/carbon-relay-ng/route"
)

type mockTable struct {
}

func (m *mockTable) AddAggregator(agg *aggregator.Aggregator) {}
func (m *mockTable) AddRewriter(rw rewriter.RW)               {}
func (m *mockTable) AddBlocklist(matcher *matcher.Matcher)    {}
func (m *mockTable) AddRoute(route route.Route)               {}
func (m *mockTable) DelRoute(key string) error                { return nil }
func (m *mockTable) UpdateDestination(key string, index int, opts map[string]string) error {
	return nil
}
func (m *mockTable) UpdateRoute(key string, opts map[string]string) error { return nil }
func (m *mockTable) GetIn() chan []byte                                   { return nil }
func (m *mockTable) GetSpoolDir() string                                  { return "fake-spool-dir" }
