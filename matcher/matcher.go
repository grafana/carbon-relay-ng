package matcher

import (
	"bytes"
	"fmt"
	"regexp"
)

type Matcher struct {
	Prefix    string `json:"prefix,omitempty"`
	NotPrefix string `json:"notPrefix,omitempty"`
	Sub       string `json:"substring,omitempty"`
	NotSub    string `json:"notSubstring,omitempty"`
	Regex     string `json:"regex,omitempty"`
	NotRegex  string `json:"notRegex,omitempty"`
	// internal representation for performance optimalization
	prefix, notPrefix, sub, notSub, prefixFromRegex, prefixFromNotRegex []byte
	// compiled version of Regex
	regex, notRegex *regexp.Regexp

	Match               matcherFunc `json:"-"`
	MatchAllExceptRegex matcherFunc `json:"-"`
}

type matcherFunc func([]byte) bool

type matcherFuncs []matcherFunc

// singleMatcherFunc generates a single matcher function which encapsulates all
// functions in matcherFuncs
func (m matcherFuncs) singleMatcherFunc() matcherFunc {
	switch len(m) {
	case 0:
		// when no filter is defined we just let everything pass
		return func(_ []byte) bool { return true }
	case 1:
		// when there is only one element in matcherFuncs we can directly return
		// it and save the overhead of having to wrap another function around it
		return m[0]
	default:
		return func(s []byte) bool {
			for _, matcherFunc := range m {
				if !matcherFunc(s) {
					return false
				}
			}
			return true
		}
	}
}

func New(prefix, notPrefix, sub, notSub, regex, notRegex string) (Matcher, error) {
	match := Matcher{
		Prefix:    prefix,
		NotPrefix: notPrefix,
		Sub:       sub,
		NotSub:    notSub,
		Regex:     regex,
		NotRegex:  notRegex,
	}
	err := match.updateInternals()
	if err != nil {
		return match, err
	}
	return match, nil
}

func (m *Matcher) String() string {
	return fmt.Sprintf("<Matcher. prefix:%q, notPrefix:%q, sub: %q, notSub: %q, regex: %q, notRegex:%q>", m.Prefix, m.NotPrefix, m.Sub, m.NotSub, m.Regex, m.NotRegex)
}

// updateInternals checks which filters this matcher should use and generates a
// filter function for each of them.
// Then it combines all these filter functions into one function
// which it assigns to Matcher.Match.
// Furthermore, it combines all filter functions except the "regex" filter into
// another single filter function and assigns it to Matcher.MatchAllExceptRegex.
func (m *Matcher) updateInternals() error {
	m.prefix = []byte(m.Prefix)
	m.notPrefix = []byte(m.NotPrefix)
	m.sub = []byte(m.Sub)
	m.notSub = []byte(m.NotSub)
	if len(m.Regex) > 0 {
		regexObj, err := regexp.Compile(m.Regex)
		if err != nil {
			return err
		}
		m.regex = regexObj
		m.prefixFromRegex = regexToPrefix(m.Regex)
	}
	if len(m.NotRegex) > 0 {
		regexObj, err := regexp.Compile(m.NotRegex)
		if err != nil {
			return err
		}
		m.notRegex = regexObj
		m.prefixFromNotRegex = regexToPrefix(m.NotRegex)
	}

	var usedFuncs matcherFuncs
	var usedFuncsExceptRegex matcherFuncs
	if len(m.prefix) > 0 {
		usedFuncs = append(usedFuncs, m.matchPrefix)
		usedFuncsExceptRegex = append(usedFuncsExceptRegex, m.matchPrefix)
	}
	if len(m.notPrefix) > 0 {
		usedFuncs = append(usedFuncs, m.matchNotPrefix)
		usedFuncsExceptRegex = append(usedFuncsExceptRegex, m.matchNotPrefix)
	}
	if len(m.sub) > 0 {
		usedFuncs = append(usedFuncs, m.matchSub)
		usedFuncsExceptRegex = append(usedFuncsExceptRegex, m.matchSub)
	}
	if len(m.notSub) > 0 {
		usedFuncs = append(usedFuncs, m.matchNotSub)
		usedFuncsExceptRegex = append(usedFuncsExceptRegex, m.matchNotSub)
	}
	if m.regex != nil {
		usedFuncs = append(usedFuncs, m.matchRegex)
		if len(m.prefixFromRegex) > 0 {
			usedFuncsExceptRegex = append(usedFuncsExceptRegex, m.matchPrefixFromRegex)
		}
	}
	if m.notRegex != nil {
		if len(m.prefixFromNotRegex) > 0 {
			usedFuncs = append(usedFuncs, m.matchNotRegexWithPreMatch)
		} else {
			usedFuncs = append(usedFuncs, m.matchNotRegex)
		}
	}

	m.Match = usedFuncs.singleMatcherFunc()
	m.MatchAllExceptRegex = usedFuncsExceptRegex.singleMatcherFunc()

	return nil
}

func (m *Matcher) matchPrefix(s []byte) bool {
	return bytes.HasPrefix(s, m.prefix)
}

func (m *Matcher) matchNotPrefix(s []byte) bool {
	return !bytes.HasPrefix(s, m.notPrefix)
}

func (m *Matcher) matchSub(s []byte) bool {
	return bytes.Contains(s, m.sub)
}

func (m *Matcher) matchNotSub(s []byte) bool {
	return !bytes.Contains(s, m.notSub)
}

func (m *Matcher) matchRegex(s []byte) bool {
	return m.regex.Match(s)
}

func (m *Matcher) matchPrefixFromRegex(s []byte) bool {
	return bytes.HasPrefix(s, m.prefixFromRegex)
}

func (m *Matcher) matchNotRegex(s []byte) bool {
	return !m.notRegex.Match(s)
}

func (m *Matcher) matchNotRegexWithPreMatch(s []byte) bool {
	if !bytes.HasPrefix(s, m.prefixFromNotRegex) {
		return true
	}
	return !m.notRegex.Match(s)
}

func (m *Matcher) MatchRegexAndExpand(key, fmt []byte) (string, bool) {
	if m.notRegex != nil {
		if !m.matchNotRegexWithPreMatch(key) {
			return "", false
		}
	}

	var dst []byte
	matches := m.regex.FindSubmatchIndex(key)
	if matches == nil {
		return "", false
	}
	return string(m.regex.Expand(dst, fmt, key, matches)), true
}

// regexToPrefix inspects the regex and returns the longest static prefix part of the regex
// all inputs for which the regex match, must have this prefix
func regexToPrefix(regex string) []byte {
	substr := ""
	for i := 0; i < len(regex); i++ {
		ch := regex[i]
		if i == 0 {
			if ch == '^' {
				continue // good we need this
			} else {
				break // can't deduce any substring here
			}
		}
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_' || ch == '-' {
			substr += string(ch)
			// "\." means a dot character
		} else if ch == 92 && i+1 < len(regex) && regex[i+1] == '.' {
			substr += "."
			i += 1
		} else {
			//fmt.Println("don't know what to do with", string(ch))
			// anything more advanced should be regex syntax that is more permissive and hence not a static substring.
			break
		}
	}
	return []byte(substr)
}
