package main

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/taylorchu/toki"
	"strconv"
	"strings"
	"time"
	//"github.com/davecgh/go-spew/spew"
)

const (
	addBlack toki.Token = iota
	addRouteSendAllMatch
	addRouteSendFirstMatch
	opt
	str
	word
)

// we should make sure we apply changes atomatically. e.g. when changing dest between address A and pickle=false and B with pickle=true,
// we should never half apply the change if one of them fails.

var tokenDefGlobal = []toki.Def{
	{Token: addBlack, Pattern: "addBlack [a-z-_]+"},
	{Token: addRouteSendAllMatch, Pattern: "addRoute sendAllMatch [a-z-_]+"},
	{Token: addRouteSendFirstMatch, Pattern: "addRoute sendFirstMatch [a-z-_]+"},
	{Token: opt, Pattern: "[a-z]+="},
	{Token: str, Pattern: "\".*\""},
	{Token: word, Pattern: "[^ ]+"},
}

var tokenDefDest = []toki.Def{
	{Token: opt, Pattern: "[a-z]+="},
	{Token: str, Pattern: "\".*\""},
	{Token: word, Pattern: "[^ ]+"},
}

// we should read and apply all destinations at once,
// or at least make sure we apply them to the global datastruct at once,
// otherwise we can destabilize things / send wrong traffic, etc
func readDestinations(specs []string, table *Table) (destinations []*Destination, err error) {
	s := toki.NewScanner(tokenDefDest)
	for _, spec := range specs {
		//fmt.Println("spec" + spec)
		var prefix, sub, regex, addr, spoolDir string
		var spool, pickle bool
		flush := 1000
		reconn := 10000
		spoolDir = table.spoolDir
		s.SetInput(spec)
		t := s.Next()
		//fmt.Println("thisisit")
		if len(t.Value) == 0 {
			return destinations, errors.New(fmt.Sprintf("addr not set for endpoint"))
		}
		addr = string(t.Value)
		//fmt.Println(t.Token, word)
		if t.Token != word {
			//fmt.Println("wtf", t.Token, word)
			return destinations, errors.New(fmt.Sprintf("expected destination endpoint spec, not '%s'", t.Value))
		}
		for {
			t := s.Next()
			if t.Token == opt {
				val := string(t.Value)
				//fmt.Println("yes got my opt with val", val)
				switch val {
				case "prefix=":
					val := s.Next()
					prefix = string(val.Value)
				case "sub=":
					val := s.Next()
					sub = string(val.Value)
				case "regex=":
					val := s.Next()
					regex = string(val.Value)
				case "flush=":
					val := s.Next()
					i, err := strconv.Atoi(string(val.Value))
					if err != nil {
						return destinations, err
					}
					flush = i
				case "reconn=":
					val := s.Next()
					i, err := strconv.Atoi(string(val.Value))
					if err != nil {
						return destinations, err
					}
					reconn = i
				case "pickle=":
					t := s.Next()
					val := string(t.Value)
					if val == "true" {
						pickle = true
					} else if val == "false" {
					} else {
						return destinations, errors.New("unrecognized pickle value '" + val + "'")
					}
				case "spool=":
					t := s.Next()
					val := string(t.Value)
					if val == "true" {
						spool = true
					} else if val != "false" {
						return destinations, errors.New("unrecognized spool value '" + val + "'")
					}
				default:
					return destinations, errors.New("unrecognized option '" + val + "'")
				}
			} else {
				break
				//return destinations, errors.New(fmt.Sprintf("expected endpoint option, not token type %v with value '%s'", t.Token, t.Value))
			}
		}

		periodFlush := time.Duration(flush) * time.Millisecond
		periodReConn := time.Duration(reconn) * time.Millisecond
		dest, err := NewDestination(prefix, sub, regex, addr, spoolDir, spool, pickle, periodFlush, periodReConn)
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
	s := toki.NewScanner(tokenDefGlobal)
	s.SetInput(inputs[0])
	t := s.Next()
	if t.Token == addBlack {
		split := bytes.Split(t.Value, []byte(" "))
		if t = s.Next(); t.Token != toki.EOF {
			return errors.New(fmt.Sprintf("extraneous input '%s'", string(t.Value)))
		}
		patt := string(split[1])
		//fmt.Println(patt)
		m, err := NewMatcher("", patt, "")
		if err != nil {
			return err
		}
		table.AddBlacklist(m)
	} else if t.Token == addRouteSendAllMatch {
		split := strings.Split(string(t.Value), " ")
		key := split[2]
		if len(inputs) < 2 {
			return errors.New("must get at least 1 destination for route " + key)
		}

		prefix, sub, regex, err := readRouteOpts(s)
		if err != nil {
			return err
		}
		destinations, err := readDestinations(inputs[1:], table)
		if err != nil {
			return err
		}
		route, err := NewRoute(sendAllMatch(1), key, prefix, sub, regex)
		if err != nil {
			return err
		}
		route.Dests = destinations
		table.AddRoute(route)
	} else if t.Token == addRouteSendFirstMatch {
		split := strings.Split(string(t.Value), " ")
		key := split[2]
		if len(inputs) < 2 {
			return errors.New("must get at least 1 destination for route " + key)
		}

		prefix, sub, regex, err := readRouteOpts(s)
		if err != nil {
			return err
		}
		destinations, err := readDestinations(inputs[1:], table)
		if err != nil {
			return err
		}
		route, err := NewRoute(sendFirstMatch(2), key, prefix, sub, regex)
		if err != nil {
			return err
		}
		route.Dests = destinations
		table.AddRoute(route)
	} else {
		return errors.New("unrecognized command '" + string(t.Value) + "'")
	}
	if t = s.Next(); t.Token != toki.EOF {
		//fmt.Println("h")
		return errors.New("extraneous input '" + string(t.Value) + "'")
	}
	return nil
}

func readRouteOpts(s *toki.Scanner) (prefix, sub, regex string, err error) {
	for {
		t := s.Next()
		//spew.Dump(t.Token)
		//spew.Dump(t)
		if t.Token == toki.EOF {
			break
		}
		if t.Token == toki.Error {
			return "", "", "", errors.New("read the error token instead of one i recognize")
		}
		if t.Token == opt {
			//fmt.Println("yet")
			switch string(t.Value) {
			case "prefix=":
				val := s.Next()
				prefix = string(val.Value)
			case "sub=":
				val := s.Next()
				sub = string(val.Value)
			case "regex=":
				val := s.Next()
				regex = string(val.Value)
			default:
				return "", "", "", errors.New("unrecognized option '" + string(t.Value) + "'")
			}
		}
	}
	return
}
