package imperatives

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/grafana/carbon-relay-ng/matcher"
	"github.com/grafana/carbon-relay-ng/pkg/test"
	"github.com/grafana/carbon-relay-ng/route"
	"github.com/grafana/carbon-relay-ng/table"
	"github.com/taylorchu/toki"
)

func TestScanner(t *testing.T) {
	cases := []struct {
		cmd string
		exp []toki.Token
	}{
		{
			"addBlock prefix collectd.localhost",
			[]toki.Token{addBlock, word, word},
		},
		{
			`addBlock regex ^foo\..*\.cpu+`,
			[]toki.Token{addBlock, word, word},
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

	table := &table.MockTable{}
	for _, c := range cases {
		err := Apply(table, c.cmd)
		if err != nil {
			t.Fatalf("could not apply init cmd %q: %s", c.cmd, err)
		}
	}
}

func TestApplyAddRouteGrafanaNet(t *testing.T) {

	schemasFile := test.TempFdOrFatal("carbon-relay-ng-TestApply-schemasFile", "[default]\npattern = .*\nretentions = 10s:1d", t)
	defer os.Remove(schemasFile.Name())

	aggregationFile := test.TempFdOrFatal("carbon-relay-ng-TestApply-aggregationFile", "[default]\npattern = .*\nretentions = 10s:1d", t)
	defer os.Remove(aggregationFile.Name())

	type testCase struct {
		cmd        string
		expCfg     route.GrafanaNetConfig
		expMatcher matcher.Matcher
		expErr     bool
	}

	var testCases []testCase

	// trivial case. mostly defaults, so let's rely on the helper that generates the (mostly default) config
	cfg, err := route.NewGrafanaNetConfig("http://foo/metrics", "apiKey", schemasFile.Name(), "")
	if err != nil {
		t.Fatal(err) // should never happen
	}
	testCases = append(testCases, testCase{
		cmd:    "addRoute grafanaNet key  http://foo/metrics apiKey " + schemasFile.Name(),
		expCfg: cfg,
		expErr: false,
	})

	// advanced case full of all possible settings.
	testCases = append(testCases, testCase{
		cmd: "addRoute grafanaNet key prefix=prefix notPrefix=notPrefix sub=sub notSub=notSub regex=regex notRegex=notRegex  http://foo.bar/metrics apiKey " + schemasFile.Name() + " aggregationFile=" + aggregationFile.Name() + " spool=true sslverify=false blocking=true concurrency=42 bufSize=123 flushMaxNum=456 flushMaxWait=5 timeout=123 orgId=10010 errBackoffMin=14 errBackoffFactor=1.8",
		expCfg: route.GrafanaNetConfig{
			Addr:            "http://foo.bar/metrics",
			ApiKey:          "apiKey",
			SchemasFile:     schemasFile.Name(),
			AggregationFile: aggregationFile.Name(),

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

	otherFile := test.TempFdOrFatal("carbon-relay-ng-TestNewGrafanaNetConfig-otherFile", "this is not an aggregation file", t)
	defer os.Remove(otherFile.Name())

	for _, aggFile := range []string{
		"some-path-that-definitely-will-not-exist-for-carbon-relay-ng",
		otherFile.Name(),
	} {
		testCases = append(testCases, testCase{
			cmd:    "addRoute grafanaNet key  http://foo/metrics apiKey " + schemasFile.Name() + " aggregationFile=" + aggFile,
			expErr: true,
		})
	}

	for _, testCase := range testCases {
		m := &table.MockTable{}
		err := Apply(m, testCase.cmd)
		if !testCase.expErr && err != nil {
			t.Fatalf("testcase with cmd %q expected no error but got error %s", testCase.cmd, err.Error())
		}
		if testCase.expErr {
			if err == nil {
				t.Fatalf("testcase with cmd %q expected error but got no error", testCase.cmd)
			}
			// don't check other conditions if we are in an error state
			continue
		}
		if len(m.Routes) != 1 {
			t.Fatalf("testcase with cmd %q resulted in %d routes, not 1", testCase.cmd, len(m.Routes))
		}
		r, ok := m.Routes[0].(*route.GrafanaNet)
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
