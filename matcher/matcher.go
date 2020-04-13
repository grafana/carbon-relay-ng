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
	prefix, notPrefix, substring, notSubstring []byte
	regex, notRegex                            *regexp.Regexp // compiled version of Regex
}

func New(prefix, notPrefix, sub, notSub, regex, notRegex string) (*Matcher, error) {
	match := new(Matcher)
	match.Prefix = prefix
	match.NotPrefix = notPrefix
	match.Sub = sub
	match.NotSub = notSub
	match.Regex = regex
	match.NotRegex = notRegex
	err := match.updateInternals()
	if err != nil {
		return nil, err
	}
	return match, nil
}

func (m *Matcher) String() string {
	return fmt.Sprintf("<Matcher. prefix:%q, sub: %q, regex: %q>", m.Prefix, m.Sub, m.Regex)
}

func (m *Matcher) updateInternals() error {
	m.prefix = []byte(m.Prefix)
	m.notPrefix = []byte(m.notPrefix)
	m.substring = []byte(m.Sub)
	m.notSubstring = []byte(m.notSubstring)
	if len(m.Regex) > 0 {
		regexObj, err := regexp.Compile(m.Regex)
		if err != nil {
			return err
		}
		m.regex = regexObj
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

func (m *Matcher) Match(s []byte) bool {
	if len(m.prefix) > 0 && !bytes.HasPrefix(s, m.prefix) {
		return false
	}
	if len(m.notPrefix) > 0 && bytes.HasPrefix(s, m.notPrefix) {
		return false
	}
	if len(m.substring) > 0 && !bytes.Contains(s, m.substring) {
		return false
	}
	if len(m.notSubstring) > 0 && bytes.Contains(s, m.notSubstring) {
		return false
	}
	if m.regex != nil && !m.regex.Match(s) {
		return false
	}
	if m.notRegex != nil && m.notRegex.Match(s) {
		return false
	}
	return true
}
