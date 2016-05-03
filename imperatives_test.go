package main

import (
	"github.com/taylorchu/toki"
	"strings"
	"testing"
)

func TestScanner(t *testing.T) {
	cases := []struct {
		cmd string
		exp []toki.Token
	}{
		{
			"addBlack prefix collectd.localhost",
			[]toki.Token{addBlack, word, word},
		},
		{
			`addBlack regex ^foo\..*\.cpu+`,
			[]toki.Token{addBlack, word, word},
		},
		{
			`addAgg sum ^stats\.timers\.(app|proxy|static)[0-9]+\.requests\.(.*) stats.timers._sum_$1.requests.$2 10 20`,
			[]toki.Token{addAgg, sumFn, word, word, num, num},
		},
		{
			`addAgg avg ^stats\.timers\.(app|proxy|static)[0-9]+\.requests\.(.*) stats.timers._avg_$1.requests.$2 5 10`,
			[]toki.Token{addAgg, avgFn, word, word, num, num},
		},
		{
			"addRoute sendAllMatch carbon-default  127.0.0.1:2005 spool=true pickle=false",
			[]toki.Token{addRouteSendAllMatch, word, sep, word, optSpool, optTrue, optPickle, optFalse},
		},
		{
			"addRoute sendAllMatch carbon-tagger sub==  127.0.0.1:2006",
			[]toki.Token{addRouteSendAllMatch, word, optSub, word, sep, word},
		},
		{
			"addRoute sendFirstMatch analytics regex=(Err/s|wait_time|logger)  graphite.prod:2003 prefix=prod. spool=true pickle=true  graphite.staging:2003 prefix=staging. spool=true pickle=true",
			[]toki.Token{addRouteSendFirstMatch, word, optRegex, word, sep, word, optPrefix, word, optSpool, optTrue, optPickle, optTrue, sep, word, optPrefix, word, optSpool, optTrue, optPickle, optTrue},
		},
	}
	for i, c := range cases {
		s := toki.NewScanner(tokens)
		s.SetInput(strings.Replace(c.cmd, "  ", " ## ", -1))
		for j, e := range c.exp {
			r := s.Next()
			if e != r.Token {
				t.Fatalf("case %d pos %d - expected %v, got %v", i, j, e, r.Token)
			}
		}
	}

	table = NewTable("")
	for _, c := range cases {
		err := applyCommand(table, c.cmd)
		if err != nil {
			t.Fatalf("could not apply init cmd %q: %s", c.cmd, err)
		}
	}
	tablePrinted := table.Print()
	t.Log("===========================")
	t.Log("========== TABLE ==========")
	t.Log("===========================")
	for _, line := range strings.Split(tablePrinted, "\n") {
		t.Logf(line)
	}

}
