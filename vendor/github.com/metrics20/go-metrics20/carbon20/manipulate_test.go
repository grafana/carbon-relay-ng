package carbon20

import (
	"github.com/bmizerany/assert"
	"strings"
	"testing"
)

var out string

type Case struct {
	in   string
	p1   string
	p2   string
	p2ne string
	out  string
}

func TestDeriveCount(t *testing.T) {
	cases := []Case{
		// metrics 2.0 cases with equals
		{"foo.bar.unit=yes.baz", "prefix.", "", "ignored", "foo.bar.unit=yesps.baz"},
		{"foo.bar.unit=yes", "prefix.", "our=prefix.", "ignored", "our=prefix.foo.bar.unit=yesps"},
		{"unit=yes.foo.bar", "prefix.", "", "ignored", "unit=yesps.foo.bar"},
		{"mtype=count.foo.unit=ok.bar", "prefix.", "", "ignored", "mtype=rate.foo.unit=okps.bar"},

		// metrics 2.0 cases without equals
		{"foo.bar.unit_is_yes.baz", "prefix.", "ignored", "", "foo.bar.unit_is_yesps.baz"},
		{"foo.bar.unit_is_yes", "prefix.", "ignored", "our_is_prefix.", "our_is_prefix.foo.bar.unit_is_yesps"},
		{"unit_is_yes.foo.bar", "prefix.", "ignored", "", "unit_is_yesps.foo.bar"},
		{"mtype_is_count.foo.unit_is_ok.bar", "prefix.", "ignored", "", "mtype_is_rate.foo.unit_is_okps.bar"},
	}
	for _, c := range cases {
		assert.Equal(t, DeriveCount(c.in, c.p1, c.p2, c.p2ne, false), c.out)
	}
}

// only 1 kind of stat is enough, cause they all behave the same
func TestStat(t *testing.T) {
	cases := []Case{
		// metrics 2.0 cases with equals
		{"foo.bar.unit=yes.baz", "prefix.", "", "ignored", "foo.bar.unit=yes.baz.stat=max_90"},
		{"foo.bar.unit=yes", "prefix.", "our=prefix.", "ignored", "our=prefix.foo.bar.unit=yes.stat=max_90"},
		{"unit=yes.foo.bar", "prefix.", "", "ignored", "unit=yes.foo.bar.stat=max_90"},
		{"mtype=count.foo.unit=ok.bar", "prefix.", "", "ignored", "mtype=count.foo.unit=ok.bar.stat=max_90"},
		// metrics 2.0 cases without equals
		{"foo.bar.unit_is_yes.baz", "prefix.", "ignored", "", "foo.bar.unit_is_yes.baz.stat_is_max_90"},
		{"foo.bar.unit_is_yes", "prefix.", "ignored", "our_is_prefix.", "our_is_prefix.foo.bar.unit_is_yes.stat_is_max_90"},
		{"unit_is_yes.foo.bar", "prefix.", "ignored", "", "unit_is_yes.foo.bar.stat_is_max_90"},
		{"mtype_is_count.foo.unit_is_ok.bar", "prefix.", "ignored", "", "mtype_is_count.foo.unit_is_ok.bar.stat_is_max_90"},
	}
	for _, c := range cases {
		assert.Equal(t, Max(c.in, c.p1, c.p2, c.p2ne, "90", ""), c.out)
	}
	// same but without percentile
	for i, c := range cases {
		cases[i].out = strings.Replace(c.out, "max_90", "max", 1)
	}
	for _, c := range cases {
		assert.Equal(t, Max(c.in, c.p1, c.p2, c.p2ne, "", ""), c.out)
	}
}
func TestRateCountPckt(t *testing.T) {
	cases := []Case{
		// metrics 2.0 cases with equals
		{"foo.bar.unit=yes.baz", "prefix.", "", "ignored", "foo.bar.unit=Pckt.baz.orig_unit=yes.pckt_type=sent.direction=in"},
		{"foo.bar.unit=yes", "prefix.", "our=prefix.", "ignored", "our=prefix.foo.bar.unit=Pckt.orig_unit=yes.pckt_type=sent.direction=in"},
		{"unit=yes.foo.bar", "prefix.", "", "ignored", "unit=Pckt.foo.bar.orig_unit=yes.pckt_type=sent.direction=in"},
		{"mtype=count.foo.unit=ok.bar", "prefix.", "", "ignored", "mtype=count.foo.unit=Pckt.bar.orig_unit=ok.pckt_type=sent.direction=in"},
		// metrics 2.0 cases without equals
		{"foo.bar.unit_is_yes.baz", "prefix.", "ignored", "", "foo.bar.unit_is_Pckt.baz.orig_unit_is_yes.pckt_type_is_sent.direction_is_in"},
		{"foo.bar.unit_is_yes", "prefix.", "ignored", "our_is_prefix.", "our_is_prefix.foo.bar.unit_is_Pckt.orig_unit_is_yes.pckt_type_is_sent.direction_is_in"},
		{"unit_is_yes.foo.bar", "prefix.", "ignored", "", "unit_is_Pckt.foo.bar.orig_unit_is_yes.pckt_type_is_sent.direction_is_in"},
		{"mtype_is_count.foo.unit_is_ok.bar", "prefix.", "ignored", "", "mtype_is_count.foo.unit_is_Pckt.bar.orig_unit_is_ok.pckt_type_is_sent.direction_is_in"},
	}
	for _, c := range cases {
		assert.Equal(t, CountPckt(c.in, c.p1, c.p2, c.p2ne), c.out)
		c.out = strings.Replace(strings.Replace(c.out, "unit=Pckt", "unit=Pcktps", -1), "mtype=count", "mtype=rate", -1)
		c.out = strings.Replace(strings.Replace(c.out, "unit_is_Pckt", "unit_is_Pcktps", -1), "mtype_is_count", "mtype_is_rate", -1)
		assert.Equal(t, RatePckt(c.in, c.p1, c.p2, c.p2ne), c.out)
	}
}

func BenchmarkDeriveCountsM20Bare(b *testing.B) {
	for i := 0; i < b.N; i++ {
		out = DeriveCount("foo=bar", "prefix-m1.", "prefix-m2.", "prefix-m2ne.", false)
	}
}

func BenchmarkDeriveCountsM20Proper(b *testing.B) {
	for i := 0; i < b.N; i++ {
		out = DeriveCount("foo=bar.unit=yes.mtype=count", "prefix-m1.", "prefix-m2.", "prefix-m2ne.", false)
	}
}

func BenchmarkDeriveCountsM20NoEqualsBare(b *testing.B) {
	for i := 0; i < b.N; i++ {
		out = DeriveCount("foo_is_bar", "prefix-m1.", "prefix-m2.", "prefix-m2ne.", false)
	}
}

func BenchmarkDeriveCountsM20NoEqualsProper(b *testing.B) {
	for i := 0; i < b.N; i++ {
		out = DeriveCount("foo_is_bar.unit_is_yes.mtype_is_count", "prefix-m1.", "prefix-m2.", "prefix-m2ne.", false)
	}
}
