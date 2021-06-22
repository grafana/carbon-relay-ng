package route

import (
	"io/ioutil"
	"os"
	"testing"
)

func TestNewGrafanaNetConfig(t *testing.T) {
	// set up some test files to use
	// note: the goal of this test is not to strictly test the correctness of the schemas reading
	// we have separate tests for that

	schemasFile := test.TempFdOrFatal("carbon-relay-ng-TestNewGrafanaNetConfig-schemasFile-valid", "[default]\npattern = .*\nretentions = 10s:1d", t)
	defer os.Remove(schemasFile.Name())

	otherFile := test.TempFdOrFatal("", "carbon-relay-ng-TestNewGrafanaNetConfig-otherFile", "this is not a schemas file", t)
	defer os.Remove(otherFile.Name())

	type input struct {
		addr        string
		apiKey      string
		schemasFile string
	}
	type testCase struct {
		in     input
		expErr bool
	}

	type option struct {
		str    string
		expErr bool
	}

	// we now test all combo's of a bunch of options for each input param
	// if any of the options expect an error, the combination expects an error

	addrOptions := []option{
		{"", true},
		{"/foo/bar", true},
		{"http://", true},
		{"https://", true},
		{"https://a", true},
		{"http://foo.bar", true},
		{"https://foo/bar", true},
		{"https://a/metrics", false},
		{"http://foo.bar/metrics", false},
		{"https://foo/bar/metrics", false},
	}

	keyOptions := []option{
		{"", true},
		{"someKey", false},
	}
	schemasFileOptions := []option{
		{"", true},
		{"some-path-that-definitely-will-not-exist-for-carbon-relay-ng", true},
		{otherFile.Name(), true},
		{schemasFile.Name(), false},
	}

	var testCases []testCase
	for _, addr := range addrOptions {
		for _, key := range keyOptions {
			for _, schemasFile := range schemasFileOptions {
				testCases = append(testCases, testCase{
					in: input{
						addr:        addr.str,
						apiKey:      key.str,
						schemasFile: schemasFile.str,
					},
					expErr: addr.expErr || key.expErr || schemasFile.expErr,
				})
			}
		}
	}
	for _, testCase := range testCases {
		_, err := NewGrafanaNetConfig(testCase.in.addr, testCase.in.apiKey, testCase.in.schemasFile)
		if !testCase.expErr && err != nil {
			t.Errorf("test with input %+v expected no error but got %s", testCase.in, err.Error())
		}
		if testCase.expErr && err == nil {
			t.Errorf("test with input %+v expected error but got none", testCase.in)
		}
	}
}

func TestGetGrafanaNetAddr(t *testing.T) {
	type testCase struct {
		in         string
		expMetrics string
		expSchemas string
	}
	testCases := []testCase{
		{
			"http://foo/metrics",
			"http://foo/metrics",
			"http://foo/graphite/config/storageSchema",
		},
		{
			"https://localhost/metrics/",
			"https://localhost/metrics",
			"https://localhost/graphite/config/storageSchema",
		},
		{
			"https://foo-us-central1.grafana.com/graphite/metrics",
			"https://foo-us-central1.grafana.com/graphite/metrics",
			"https://foo-us-central1.grafana.com/graphite/config/storageSchema",
		},
		{
			"https://foo-us-central1.grafana.com/graphite/metrics/",
			"https://foo-us-central1.grafana.com/graphite/metrics",
			"https://foo-us-central1.grafana.com/graphite/config/storageSchema",
		},
	}
	for _, c := range testCases {
		gotMetrics, gotSchemas := getGrafanaNetAddr(c.in)
		if gotMetrics != c.expMetrics || gotSchemas != c.expSchemas {
			t.Errorf("testcase %s mismatch:\nexp metrics addr: %s\ngot metrics addr: %s \nexp schemas addr: %s\ngot schemas addr: %s", c.in, c.expMetrics, gotMetrics, c.expSchemas, gotSchemas)
		}
	}
}
