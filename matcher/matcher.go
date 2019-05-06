package matcher

import (
	"bytes"
	"fmt"
	"regexp"
)

type Matcher struct {
	Prefix string `json:"prefix,omitempty"`
	Sub    string `json:"substring,omitempty"`
	Regex  string `json:"regex,omitempty"`
	// internal representation for performance optimalization
	prefix, substring []byte
	regex             *regexp.Regexp // compiled version of Regex
}

func New(prefix, sub, regex string) (*Matcher, error) {
	match := new(Matcher)
	match.Prefix = prefix
	match.Sub = sub
	match.Regex = regex
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
	m.substring = []byte(m.Sub)
	if len(m.Regex) > 0 {
		regexObj, err := regexp.Compile(m.Regex)
		if err != nil {
			return err
		}
		m.regex = regexObj
	}
	return nil
}

func (m *Matcher) MatchString(s string) bool {
	return m.Match([]byte(s))
}

func (m *Matcher) Match(s []byte) bool {
	if len(m.prefix) > 0 && !bytes.HasPrefix(s, m.prefix) {
		return false
	}
	if len(m.substring) > 0 && !bytes.Contains(s, m.substring) {
		return false
	}
	if m.regex != nil && !m.regex.Match(s) {
		return false
	}
	return true
}
