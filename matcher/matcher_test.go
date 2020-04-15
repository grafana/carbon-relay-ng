package matcher

import (
	"testing"
)

type tcMatcher struct {
	name               string
	prefix             string
	notPrefix          string
	substr             string
	notSubstr          string
	regex              string
	notRegex           string
	expectInitError    bool
	valuesIn           []string
	matches            []bool
	matchesExceptRegex []bool
}

func (tc tcMatcher) Run(t *testing.T) {
	matcher, err := New(tc.prefix, tc.notPrefix, tc.substr, tc.notSubstr, tc.regex, tc.notRegex)
	if err != nil && !tc.expectInitError {
		t.Fatalf("%s: Unexepcted initialization error: %s", tc.name, err)
	}
	if err == nil && tc.expectInitError {
		t.Fatalf("%s: Expected initialization error but didn't get one", tc.name)
	}

	for idx, value := range tc.valuesIn {
		gotMatch := matcher.Match([]byte(value))
		expectMatch := tc.matches[idx]
		if gotMatch != expectMatch {
			t.Fatalf("%s: Expected match to be %t but got %t: %s", tc.name, expectMatch, gotMatch, value)
		}
		gotMatchExceptRegex := matcher.PreMatch([]byte(value))
		expectMatchExceptRegex := tc.matchesExceptRegex[idx]
		if gotMatchExceptRegex != expectMatchExceptRegex {
			t.Fatalf("%s: Expected matchExceptRegex to be %t but got %t: %s", tc.name, expectMatchExceptRegex, gotMatchExceptRegex, value)
		}
	}
}

func TestInvalidRegex(t *testing.T) {
	tcMatcher{
		name:            "test passing invalid regex",
		regex:           "(abc",
		expectInitError: true,
	}.Run(t)
}

func TestInvalidNotRegex(t *testing.T) {
	tcMatcher{
		name:            "test passing invalid expression to notRegex",
		notRegex:        "aaa)",
		expectInitError: true,
	}.Run(t)
}

func TestMatchingPrefix(t *testing.T) {
	tcMatcher{
		name:   "test prefix",
		prefix: "aaa",
		valuesIn: []string{
			"abc",
			"aaaaa",
			"baab",
			"a",
		},
		matches: []bool{
			false,
			true,
			false,
			false,
		},
		matchesExceptRegex: []bool{
			false,
			true,
			false,
			false,
		},
	}.Run(t)
}

func TestMatchingNotPrefix(t *testing.T) {
	tcMatcher{
		name:      "test notPrefix",
		notPrefix: "aaa",
		valuesIn: []string{
			"abc",
			"aaaaa",
			"baab",
			"a",
		},
		matches: []bool{
			true,
			false,
			true,
			true,
		},
		matchesExceptRegex: []bool{
			true,
			false,
			true,
			true,
		},
	}.Run(t)
}

func TestMatchingSubstr(t *testing.T) {
	tcMatcher{
		name:   "test substr",
		substr: "aaa",
		valuesIn: []string{
			"abc",
			"aaaaa",
			"baaa",
			"aaab",
			"bbbaaabbb",
			"aa",
		},
		matches: []bool{
			false,
			true,
			true,
			true,
			true,
			false,
		},
		matchesExceptRegex: []bool{
			false,
			true,
			true,
			true,
			true,
			false,
		},
	}.Run(t)
}

func TestMatchingNotSubstr(t *testing.T) {
	tcMatcher{
		name:      "test not substr",
		notSubstr: "aaa",
		valuesIn: []string{
			"abc",
			"aaaaa",
			"baaa",
			"aaab",
			"bbbaaabbb",
			"aa",
		},
		matches: []bool{
			true,
			false,
			false,
			false,
			false,
			true,
		},
		matchesExceptRegex: []bool{
			true,
			false,
			false,
			false,
			false,
			true,
		},
	}.Run(t)
}

func TestMatchingRegexAnchored(t *testing.T) {
	tcMatcher{
		name:  "test regex anchored",
		regex: "^[a-c]{3}",
		valuesIn: []string{
			"abcdef",
			"aad",
			"daaab",
		},
		matches: []bool{
			true,
			false,
			false,
		},
		matchesExceptRegex: []bool{
			true,
			true,
			true,
		},
	}.Run(t)
}

func TestMatchingRegexWithFixPrefix(t *testing.T) {
	tcMatcher{
		name:  "test regex anchored",
		regex: "^abc.+",
		valuesIn: []string{
			"daaab",
			"abc",
			"abcdef",
		},
		matches: []bool{
			false,
			false,
			true,
		},
		matchesExceptRegex: []bool{
			false,
			true,
			true,
		},
	}.Run(t)
}

func TestMatchingRegexNotAnchored(t *testing.T) {
	tcMatcher{
		name:  "test regex not anchored",
		regex: "[a-c]{3}",
		valuesIn: []string{
			"abcdef",
			"aadbbb",
			"daaab",
			"dadada",
		},
		matches: []bool{
			true,
			true,
			true,
			false,
		},
		matchesExceptRegex: []bool{
			true,
			true,
			true,
			true,
		},
	}.Run(t)
}

func TestMatchingNotRegexAnchored(t *testing.T) {
	tcMatcher{
		name:     "test not regex anchored",
		notRegex: "^[a-c]{3}",
		valuesIn: []string{
			"abcdef",
			"aad",
			"daaab",
		},
		matches: []bool{
			false,
			true,
			true,
		},
		matchesExceptRegex: []bool{
			true,
			true,
			true,
		},
	}.Run(t)
}

func TestMatchingNotRegexWithFixPrefix(t *testing.T) {
	tcMatcher{
		name:     "test not regex with fix prefix",
		notRegex: "^abc.+",
		valuesIn: []string{
			"daab",
			"abc",
			"abcabc",
		},
		matches: []bool{
			true,
			true,
			false,
		},
		matchesExceptRegex: []bool{
			true,
			true,
			true,
		},
	}.Run(t)
}

func TestMatchingNotRegexNotAnchored(t *testing.T) {
	tcMatcher{
		name:     "test not regex not anchored",
		notRegex: "[a-c]{3}",
		valuesIn: []string{
			"abcdef",
			"aadbbb",
			"daaab",
			"dadada",
		},
		matches: []bool{
			false,
			false,
			false,
			true,
		},
		matchesExceptRegex: []bool{
			true,
			true,
			true,
			true,
		},
	}.Run(t)
}

func TestMatchingWith2Conditions(t *testing.T) {
	tcMatcher{
		name:   "test regex and prefix combined",
		regex:  "[a-c]{3}",
		prefix: "f",
		valuesIn: []string{
			"abcf",
			"fabc",
			"fffabc",
			"fffab",
			"abcffff",
		},
		matches: []bool{
			false,
			true,
			true,
			false,
			false,
		},
		matchesExceptRegex: []bool{
			false,
			true,
			true,
			true,
			false,
		},
	}.Run(t)
}

func TestMatchingWith3Conditions(t *testing.T) {
	tcMatcher{
		name:      "test regex, prefix and notPrefix combined",
		regex:     "[a-c]{3}",
		prefix:    "f",
		notPrefix: "fa",
		valuesIn: []string{
			"fabc",
			"fbca",
			"fcba",
			"fcb",
			"cbaf",
		},
		matches: []bool{
			false,
			true,
			true,
			false,
			false,
		},
		matchesExceptRegex: []bool{
			false,
			true,
			true,
			true,
			false,
		},
	}.Run(t)

	tcMatcher{
		name:      "test sub, notSub and notRegex combined",
		substr:    "a",
		notSubstr: "aa",
		notRegex:  "ab",
		valuesIn: []string{
			"aaa",
			"aba",
			"aab",
			"aca",
			"bab",
			"bbb",
			"cba",
		},
		matches: []bool{
			false,
			false,
			false,
			true,
			false,
			false,
			true,
		},
		matchesExceptRegex: []bool{
			false,
			true,
			false,
			true,
			true,
			false,
			true,
		},
	}.Run(t)
}

func TestMatchingWith4Conditions(t *testing.T) {
	tcMatcher{
		name:      "test regex, prefix, notSub and notPrefix combined",
		regex:     "[a-c]{3}",
		prefix:    "f",
		notSubstr: "bca",
		notPrefix: "fa",
		valuesIn: []string{
			"fabc",
			"fbca",
			"fcba",
			"fcb",
			"cbaf",
		},
		matches: []bool{
			false,
			false,
			true,
			false,
			false,
		},
		matchesExceptRegex: []bool{
			false,
			false,
			true,
			true,
			false,
		},
	}.Run(t)
}

func BenchmarkMatchPrefix(b *testing.B) {
	matcher, _ := New("abcde_fghij.klmnopqrst", "", "", "", "", "")
	metric70 := []byte("abcde_fghij.klmnopqrst.uv_wxyz.1234567890abcdefg 12345.6789 1234567890") // key = 48, val = 10, ts = 10 -> 70
	for i := 0; i < b.N; i++ {
		matcher.Match(metric70)
	}
}

func BenchmarkMatchSub(b *testing.B) {
	matcher, _ := New("", "", "1234567890abc", "", "", "")
	metric70 := []byte("abcde_fghij.klmnopqrst.uv_wxyz.1234567890abcdefg 12345.6789 1234567890") // key = 48, val = 10, ts = 10 -> 70
	for i := 0; i < b.N; i++ {
		matcher.Match(metric70)
	}
}

func BenchmarkMatchRegex(b *testing.B) {
	matcher, _ := New("", "", "", "", "abcde_(fghij|foo).[^\\.]+.\\.*.\\.*", "")
	metric70 := []byte("abcde_fghij.klmnopqrst.uv_wxyz.1234567890abcdefg 12345.6789 1234567890") // key = 48, val = 10, ts = 10 -> 70
	for i := 0; i < b.N; i++ {
		matcher.Match(metric70)
	}
}

func BenchmarkMatch3Conditions(b *testing.B) {
	matcher, _ := New("abc", "cba", "", "notpresentstring", "", "")
	metric70 := []byte("abcde_fghij.klmnopqrst.uv_wxyz.1234567890abcdefg 12345.6789 1234567890") // key = 48, val = 10, ts = 10 -> 70
	for i := 0; i < b.N; i++ {
		matcher.Match(metric70)
	}
}

func BenchmarkMatch4Conditions(b *testing.B) {
	matcher, _ := New("abc", "cba", "klmno", "notpresentstring", "", "")
	metric70 := []byte("abcde_fghij.klmnopqrst.uv_wxyz.1234567890abcdefg 12345.6789 1234567890") // key = 48, val = 10, ts = 10 -> 70
	for i := 0; i < b.N; i++ {
		matcher.Match(metric70)
	}
}
