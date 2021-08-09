package table

import (
	"github.com/grafana/carbon-relay-ng/aggregator"
	"github.com/grafana/carbon-relay-ng/matcher"
	"github.com/grafana/carbon-relay-ng/rewriter"
	"github.com/grafana/carbon-relay-ng/route"
)

// Interface represents a table abstractly
type Interface interface {
	AddAggregator(agg *aggregator.Aggregator)
	AddRewriter(rw rewriter.RW)
	AddBlocklist(matcher *matcher.Matcher)
	AddRoute(route route.Route)
	DelRoute(key string) error
	UpdateDestination(key string, index int, opts map[string]string) error
	UpdateRoute(key string, opts map[string]string) error
	GetIn() chan []byte
	GetSpoolDir() string
}
