package main

import (
	"errors"
	"fmt"
	"github.com/taylorchu/toki"
	"strings"
)

const (
	addBlack toki.TokenType = iota
	addRouteSendAllMatch
	addRouteSendFirstMatch
	opt
	str
	word
)

// we should make sure we apply changes atomatically. e.g. when changing dest between address A and pickle=false and B with pickle=true,
// we should never half apply the change if one of them fails.

var tokenDefGlobal = []toki.TokenDef{
	{Type: addBlack, Pattern: "addBlack [a-z]+"},
	{Type: addRouteSendAllMatch, Pattern: "addRoute sendAllMatch [a-z]+"},
	{Type: addRouteSendFirstMatch, Pattern: "addRoute sendFirstMatch [a-z]+"},
	{Type: opt, Pattern: "[a-z]+="},
	{Type: str, Pattern: "\".*\""},
	{Type: word, Pattern: "[a-z]+"},
}

var tokenDefDest = []toki.TokenDef{
	{Type: opt, Pattern: "[a-z]+="},
	{Type: str, Pattern: "\".*\""},
	{Type: word, Pattern: "[a-z]+"},
}

// we should read and apply all destinations at once,
// or at least make sure we apply them to the global datastruct at once,
// otherwise we can destabilize things / send wrong traffic, etc
func readDestinations(specs []string) (destinations []*Destination, err error) {
	s := toki.New(tokenDefDest)
	for _, spec := range specs {
		var prefix, sub, regex, addr, spoolDir string
		var spool, pickle bool
		spoolDir = "TODO" // also set statsd somehow
		s.SetInput(spec)
		t := s.Next()
		if t.Type != str {
			return destinations, errors.New(fmt.Sprintf("expected destination endpoint spec, not '%s'", t))
		}
		for {
			t := s.Next()
			if t.Type == opt {
				switch t.Value {
				case "prefix=":
					val := s.Next()
					prefix = val.Value
				case "sub=":
					val := s.Next()
					sub = val.Value
				case "regex=":
					val := s.Next()
					regex = val.Value
				case "addr=":
					val := s.Next()
					addr = val.Value
				case "pickle=":
					t := s.Next()
					if t.Value == "true" {
					} else if t.Value == "false" {
					} else {
						return destinations, errors.New("unrecognized pickle value '" + t.Value + "'")
					}
				case "spool=":
					t := s.Next()
					if t.Value == "true" {
						spool = true
					} else if t.Value != "false" {
						return destinations, errors.New("unrecognized spool value '" + t.Value + "'")
					}
				default:
					return destinations, errors.New("unrecognized option '" + t.Value + "'")
				}
			} else {
				return destinations, errors.New("expected endpoint option, not '" + t.Value + "'")
			}
		}
		dest, err := NewDestination(prefix, sub, regex, addr, spoolDir, spool, pickle, &statsd)
		if err != nil {
			return destinations, err
		}
		destinations = append(destinations, dest)
	}
	return destinations, nil
}

// note the two spaces between a route and endpoints
//"addBlack filter-out-all-metrics-matching-this-substring",
//"addRoute sendAllMatch carbon-default  127.0.0.1:2005 spool=false pickle=false",
//"addRoute sendFirstMatch demo sub=foo prefix=foo re=foo  127.0.0.1:12345 spool=true"
//addBlack string-without-spaces
//addRoute <type> <key> <match options>  <dests> # match options can't have spaces for now. sorry
//dests:
// <tcp addr> <options>

func applyCommand(table *Table, cmd string) error {
	inputs := strings.Split(cmd, "  ")
	s := toki.New(tokenDefGlobal)
	s.SetInput(inputs[0])
	t := s.Next()
	if t.Type == addBlack {
		split := strings.Split(t.Value, " ")
		patt := split[1]
		if t = s.Next(); t.Type != toki.TokenEOF {
			return errors.New("extraneous input '" + t.Value + "'")
		}
		fmt.Println(patt)
		m, err := NewMatcher("", patt, "")
		if err != nil {
			return err
		}
		table.AddBlacklist(m)
	} else if t.Type == addRouteSendAllMatch {
		split := strings.Split(t.Value, " ")
		key := split[2]
		// read opts
		t = s.Next()
		var prefix, sub, regex string
		for {
			t := s.Next()
			if t.Type == toki.TokenEOF {
				break
			}
			if t.Type == toki.TokenError {
				return errors.New(t.Value)
			}
			if t.Type == opt {
				switch t.Value {
				case "prefix=":
					val := s.Next()
					prefix = val.Value
				case "sub=":
					val := s.Next()
					sub = val.Value
				case "regex=":
					val := s.Next()
					regex = val.Value
				default:
					return errors.New("unrecognized option '" + t.Value + "'")
				}
			}
		}
		if len(inputs) < 2 {
			return errors.New("must get at least 1 destination for route " + key)
		}
		destinations, err := readDestinations(inputs[1:])
		if err != nil {
			return err
		}
		route, err := NewRoute(sendAllMatch(1), key, prefix, sub, regex)
		if err != nil {
			return err
		}
		route.Dests = destinations
		table.AddRoute(route)
	} else if t.Type == addRouteSendFirstMatch {
	} else {
		return errors.New("unrecognized command '" + t.Value + "'")
	}
	if t = s.Next(); t.Type != toki.TokenEOF {
		return errors.New("extraneous input '" + t.Value + "'")
	}
	return nil
}
