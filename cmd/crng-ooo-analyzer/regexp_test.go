package main

import (
	"regexp/syntax"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestGroupsEmpty(t *testing.T) {
	testGroups("", nil, t)
}

func TestGroupsNone(t *testing.T) {
	testGroups("foo.bar.baz", nil, t)
}

func TestGroupsBasic(t *testing.T) {
	want := []group{
		{
			Num:  1,
			Patt: `[^\.]+`,
		},
		{
			Num:  2,
			Patt: "test",
		},
		{
			Num:  3,
			Name: "last",
			Patt: "[A-Za-z]+",
		},
	}
	testGroups(`^foo-([^\.]+)\.(test)\.otherstuff(?P<last>[a-zA-Z]+)`, want, t)
}

func testGroups(input string, want []group, t *testing.T) {
	re, err := syntax.Parse(input, syntax.Perl)
	if err != nil {
		t.Error(err)
	}
	got := groups(re)
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("groups() mismatch (-want +got):\n%s", diff)
	}
}
