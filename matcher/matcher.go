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
	prefix, notPrefix, sub, notSub, prefixFromRegex []byte
	// compiled version of Regex
	regex, notRegex *regexp.Regexp
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
	}
	return nil
}

// Match matches the given byte slice against all filter conditions
func (m *Matcher) Match(s []byte) bool {
	if len(m.prefix) > 0 && !bytes.HasPrefix(s, m.prefix) {
		return false
	}
	if len(m.notPrefix) > 0 && bytes.HasPrefix(s, m.notPrefix) {
		return false
	}
	if len(m.sub) > 0 && !bytes.Contains(s, m.sub) {
		return false
	}
	if len(m.notSub) > 0 && bytes.Contains(s, m.notSub) {
		return false
	}
	if m.regex != nil {
		if len(m.prefixFromRegex) > 0 && bytes.HasPrefix(s, m.prefixFromRegex) {
			return false
		}
		if !m.regex.Match(s) {
			return false
		}
	}
	if m.notRegex != nil && m.notRegex.Match(s) {
		return false
	}
	return true
}

// MatchAll matches the given byte slice against all filter conditions except "regex"
func (m *Matcher) MatchAllExceptRegex(s []byte) bool {
	if len(m.prefix) > 0 && !bytes.HasPrefix(s, m.prefix) {
		return false
	}
	if len(m.notPrefix) > 0 && bytes.HasPrefix(s, m.notPrefix) {
		return false
	}
	if len(m.sub) > 0 && !bytes.Contains(s, m.sub) {
		return false
	}
	if len(m.notSub) > 0 && bytes.Contains(s, m.notSub) {
		return false
	}
	if len(m.prefixFromRegex) > 0 && bytes.HasPrefix(s, m.prefixFromRegex) {
		return false
	}
	if m.notRegex != nil && m.notRegex.Match(s) {
		return false
	}
	return true
}

func (m *Matcher) MatchRegexAndExpand(key, fmt []byte) (string, bool) {
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
