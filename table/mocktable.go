package table

import (
	"github.com/grafana/carbon-relay-ng/aggregator"
	"github.com/grafana/carbon-relay-ng/matcher"
	"github.com/grafana/carbon-relay-ng/rewriter"
	"github.com/grafana/carbon-relay-ng/route"
)

// MockTable is used for tests
type MockTable struct {
	Aggregators []*aggregator.Aggregator
	Rewriters   []rewriter.RW
	Blocklist   []*matcher.Matcher
	Routes      []route.Route
}

func (m *MockTable) AddAggregator(agg *aggregator.Aggregator) {
	m.Aggregators = append(m.Aggregators, agg)
}
func (m *MockTable) AddRewriter(rw rewriter.RW) {
	m.Rewriters = append(m.Rewriters, rw)
}
func (m *MockTable) AddBlocklist(matcher *matcher.Matcher) {
	m.Blocklist = append(m.Blocklist, matcher)
}
func (m *MockTable) AddRoute(route route.Route) {
	m.Routes = append(m.Routes, route)
}
func (m *MockTable) DelRoute(key string) error { panic("not implemented") }
func (m *MockTable) UpdateDestination(key string, index int, opts map[string]string) error {
	panic("not implemented")
}
func (m *MockTable) UpdateRoute(key string, opts map[string]string) error { panic("not implemented") }
func (m *MockTable) GetIn() chan []byte                                   { return nil }
func (m *MockTable) GetSpoolDir() string                                  { return "/fake/non/existant/spooldir/that/shouldnt/be/used" }
