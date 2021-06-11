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
	schemasFile, err := ioutil.TempFile("", "carbon-relay-ng-TestNewGrafanaNetConfig-valid")
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

	otherFile, err := ioutil.TempFile("", "carbon-relay-ng-TestNewGrafanaNetConfig-otherFile")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(otherFile.Name())
	if _, err := otherFile.Write([]byte("this is not a schemas file")); err != nil {
		t.Fatal(err)
	}
	if err := otherFile.Close(); err != nil {
		t.Fatal(err)
	}

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
		{"https://a", false},
		{"http://foo.bar", false},
		{"https://foo/bar", false},
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
