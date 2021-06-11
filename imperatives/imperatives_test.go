package imperatives

import (
	"io/ioutil"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/grafana/carbon-relay-ng/aggregator"
	"github.com/grafana/carbon-relay-ng/matcher"
	"github.com/grafana/carbon-relay-ng/rewriter"
	"github.com/grafana/carbon-relay-ng/route"
	"github.com/taylorchu/toki"
)

func TestScanner(t *testing.T) {
	cases := []struct {
		cmd string
		exp []toki.Token
	}{
		{
			"addBlack prefix collectd.localhost",
			[]toki.Token{addBlack, word, word},
		},
		{
			`addBlack regex ^foo\..*\.cpu+`,
			[]toki.Token{addBlack, word, word},
		},
		{
			`addAgg sum ^stats\.timers\.(app|proxy|static)[0-9]+\.requests\.(.*) stats.timers._sum_$1.requests.$2 10 20`,
			[]toki.Token{addAgg, sumFn, word, word, num, num},
		},
		{
			`addAgg avg ^stats\.timers\.(app|proxy|static)[0-9]+\.requests\.(.*) stats.timers._avg_$1.requests.$2 5 10`,
			[]toki.Token{addAgg, avgFn, word, word, num, num},
		},
		{
			"addRoute sendAllMatch carbon-default  127.0.0.1:2005 spool=true pickle=false",
			[]toki.Token{addRouteSendAllMatch, word, sep, word, optSpool, optTrue, optPickle, optFalse},
		},
		{
			"addRoute sendAllMatch carbon-tagger sub==  127.0.0.1:2006",
			[]toki.Token{addRouteSendAllMatch, word, optSub, word, sep, word},
		},
		{
			"addRoute sendFirstMatch analytics regex=(Err/s|wait_time|logger)  graphite.prod:2003 prefix=prod. spool=true pickle=true  graphite.staging:2003 prefix=staging. spool=true pickle=true",
			[]toki.Token{addRouteSendFirstMatch, word, optRegex, word, sep, word, optPrefix, word, optSpool, optTrue, optPickle, optTrue, sep, word, optPrefix, word, optSpool, optTrue, optPickle, optTrue},
		},
		{
			"addRoute sendFirstMatch myRoute1  127.0.0.1:2003 notPrefix=aaa notSub=bbb notRegex=ccc",
			[]toki.Token{addRouteSendFirstMatch, word, sep, word, optNotPrefix, word, optNotSub, word, optNotRegex, word},
		},
		//{ disabled cause tries to read the schemas.conf file
		//	"addRoute grafanaNet grafanaNet  http://localhost:8081/metrics your-grafana.net-api-key /path/to/storage-schemas.conf",
		//	[]toki.Token{addRouteGrafanaNet, word, sep, word, word},
		//},
	}
	for i, c := range cases {
		s := toki.NewScanner(tokens)
		s.SetInput(strings.Replace(c.cmd, "  ", " ## ", -1))
		for j, e := range c.exp {
			r := s.Next()
			if e != r.Token {
				t.Fatalf("case %d pos %d - expected %v, got %v", i, j, e, r.Token)
			}
		}
	}

	table := &mockTable{}
	for _, c := range cases {
		err := Apply(table, c.cmd)
		if err != nil {
			t.Fatalf("could not apply init cmd %q: %s", c.cmd, err)
		}
	}
}

type MockTable struct {
	aggregators []*aggregator.Aggregator
	rewriters   []rewriter.RW
	blacklist   []*matcher.Matcher
	routes      []route.Route
}

func (m *MockTable) AddAggregator(agg *aggregator.Aggregator) {
	m.aggregators = append(m.aggregators, agg)
}
func (m *MockTable) AddRewriter(rw rewriter.RW) {
	m.rewriters = append(m.rewriters, rw)
}
func (m *MockTable) AddBlacklist(matcher *matcher.Matcher) {
	m.blacklist = append(m.blacklist, matcher)
}
func (m *MockTable) AddRoute(route route.Route) {
	m.routes = append(m.routes, route)
}
func (m *MockTable) DelRoute(key string) error { panic("not implemented") }
func (m *MockTable) UpdateDestination(key string, index int, opts map[string]string) error {
	panic("not implemented")
}
func (m *MockTable) UpdateRoute(key string, opts map[string]string) error { panic("not implemented") }
func (m *MockTable) GetIn() chan []byte                                   { panic("not implemented") }
func (m *MockTable) GetSpoolDir() string                                  { panic("not implemented") }

func TestApply(t *testing.T) {

	schemasFile, err := ioutil.TempFile("", "carbon-relay-ng-TestApply-schemasFile")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(schemasFile.Name())
	if _, err := schemasFile.Write([]byte("[default]\npattern = .*\nretentions = 10s:1d")); err != nil {
		t.Fatal(err)
	}
	if err := schemasFile.Close(); err != nil {
		t.Fatal(err)
	}

	type testCase struct {
		cmd        string
		expCfg     route.GrafanaNetConfig
		expMatcher matcher.Matcher
		expErr     bool
	}

	var testCases []testCase

	// trivial case. mostly defaults, so let's rely on the helper that generates the (mostly default) config
	cfg, err := route.NewGrafanaNetConfig("http://foo", "apiKey", schemasFile.Name())
	if err != nil {
		t.Fatal(err) // should never happen
	}
	testCases = append(testCases, testCase{
		cmd:    "addRoute grafanaNet key  http://foo apiKey " + schemasFile.Name(),
		expCfg: cfg,
		expErr: false,
	})

	// advanced case full of all possible settings.
	testCases = append(testCases, testCase{
		cmd: "addRoute grafanaNet key prefix=prefix notPrefix=notPrefix sub=sub notSub=notSub regex=regex notRegex=notRegex  http://foo.bar apiKey " + schemasFile.Name() + " spool=true sslverify=false blocking=true concurrency=42 bufSize=123 flushMaxNum=456 flushMaxWait=5 timeout=123 orgId=10010 errBackoffMin=14 errBackoffFactor=1.8",
		expCfg: route.GrafanaNetConfig{
			Addr:        "http://foo.bar",
			ApiKey:      "apiKey",
			SchemasFile: schemasFile.Name(),

			BufSize:      123,
			FlushMaxNum:  456,
			FlushMaxWait: 5 * time.Millisecond,
			Timeout:      123 * time.Millisecond,
			Concurrency:  42,
			OrgID:        10010,
			SSLVerify:    false,
			Blocking:     true,
			Spool:        true,

			ErrBackoffMin:    14 * time.Millisecond,
			ErrBackoffFactor: 1.8,
		},
		expMatcher: matcher.Matcher{
			Prefix:    "prefix",
			NotPrefix: "notPrefix",
			Sub:       "sub",
			NotSub:    "notSub",
			Regex:     "regex",
			NotRegex:  "notRegex",
		},
		expErr: false,
	})

	for _, testCase := range testCases {
		m := &MockTable{}
		err := Apply(m, testCase.cmd)
		if !testCase.expErr && err != nil {
			t.Fatalf("testcase with cmd %q expected no error but got error %s", testCase.cmd, err.Error())
		}
		if testCase.expErr && err == nil {
			t.Fatalf("testcase with cmd %q expected error but got no error", testCase.cmd)
		}
		if len(m.routes) != 1 {
			t.Fatalf("testcase with cmd %q resulted in %d routes, not 1", testCase.cmd, len(m.routes))
		}
		r, ok := m.routes[0].(*route.GrafanaNet)
		if !ok {
			t.Fatalf("testcase with cmd %q resulted in route of wrong type. needed GrafanaNet", testCase.cmd)
		}
		if r.Cfg != testCase.expCfg {
			t.Fatalf("testcase with cmd %q resulted in wrong config %+v", testCase.cmd, r.Cfg)
		}
		snap := r.Snapshot()
		if !snap.Matcher.Equals(testCase.expMatcher) {
			t.Fatalf("testcase with cmd %q resulted in wrong matcher %+v", testCase.cmd, snap.Matcher)
		}
	}
}
