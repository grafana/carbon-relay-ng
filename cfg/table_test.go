package cfg

import (
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/grafana/carbon-relay-ng/matcher"
	"github.com/grafana/carbon-relay-ng/route"
	"github.com/grafana/carbon-relay-ng/table"
)

func TestTomlToGrafanaNetRoute(t *testing.T) {
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
		title      string
		cfg        string
		expCfg     route.GrafanaNetConfig
		expMatcher matcher.Matcher
		expErr     bool
	}

	var testCases []testCase

	cfg, err := route.NewGrafanaNetConfig("http://foo", "apiKey", schemasFile.Name())
	if err != nil {
		t.Fatal(err) // should never happen
	}
	testCases = append(testCases, testCase{
		title: "trivial case. mostly defaults",
		cfg: `
[[route]]
key = 'routeKey'
type = 'grafanaNet'
addr = 'http://foo'
apikey = 'apiKey'
schemasFile = '` + schemasFile.Name() + `'
`,
		expCfg: cfg,
		expErr: false,
	})

	testCases = append(testCases, testCase{
		title: "advanced case full of all possible settings",
		cfg: `
[[route]]
key              = 'routeKey'
type             = 'grafanaNet'
addr             = 'http://foo.bar'
apikey           = 'apiKey'
schemasFile      = '` + schemasFile.Name() + `'
prefix           = 'prefix'
notPrefix        = 'notPrefix'
sub              = 'sub'
notSub           = 'notSub'
regex            = 'regex'
notRegex         = 'notRegex'
sslverify        = false
spool            = true
bufSize          = 123
blocking         = true
flushMaxNum      = 456
flushMaxWait     = 5
timeout          = 123
concurrency      = 42
errBackoffMin    = 14
errBackoffFactor = 1.8
orgId            = 10010
`,
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
		config := NewConfig()
		meta, err := toml.Decode(testCase.cfg, &config)
		if err != nil {
			t.Fatal(err)
		}

		m := &table.MockTable{}

		err = InitRoutes(m, config, meta)
		if err != nil {
			t.Fatal(err)
		}
		if !testCase.expErr && err != nil {
			t.Fatalf("testcase %q expected no error but got error %s", testCase.title, err.Error())
		}
		if testCase.expErr && err == nil {
			t.Fatalf("testcase %q expected error but got no error", testCase.title)
		}
		if len(m.Routes) != 1 {
			t.Fatalf("testcase %q resulted in %d routes, not 1", testCase.title, len(m.Routes))
		}
		r, ok := m.Routes[0].(*route.GrafanaNet)
		if !ok {
			t.Fatalf("testcase %q resulted in route of wrong type. needed GrafanaNet", testCase.title)
		}
		if r.Cfg != testCase.expCfg {
			t.Fatalf("testcase %q config expected:\n%+v\nconfig got:\n%+v\n", testCase.title, testCase.expCfg, r.Cfg)
		}
		snap := r.Snapshot()
		if !snap.Matcher.Equals(testCase.expMatcher) {
			t.Fatalf("testcase %q resulted in wrong matcher %+v", testCase.title, snap.Matcher)
		}
	}
}
