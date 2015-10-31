package metrics20

import (
	"github.com/graphite-ng/carbon-relay-ng/_third_party/github.com/bmizerany/assert"
	"strings"
	"testing"
)

type VersionCase struct {
	in      string
	version metricVersion
	valid   bool
}

func TestValidate(t *testing.T) {
	cases := []VersionCase{
		VersionCase{"foo.bar.aunit=no.baz", M20, false},
		VersionCase{"foo.bar.UNIT=no.baz", M20, false},
		VersionCase{"foo.bar.unita=no.bar", M20, false},
		VersionCase{"foo.bar.target_type_is_count.baz", M20NoEquals, false},
		VersionCase{"foo.bar.target_type_is_count", M20NoEquals, false},
		VersionCase{"target_type_is_count.foo.bar", M20NoEquals, false},
	}
	for _, c := range cases {
		version := GetVersion(c.in)
		assert.Equal(t, c.version, version)
		assert.Equal(t, InitialValidation(c.in, version) == nil, c.valid)
	}
}

type VersionCaseB struct {
	in               string
	version          metricVersion
	legacyValidation LegacyMetricValidation
	valid            bool
}

func TestByteValidation(t *testing.T) {
	cases := []VersionCaseB{
		VersionCaseB{"foo.bar", Legacy, Strict, true},
		VersionCaseB{"foo.bar", Legacy, Medium, true},
		VersionCaseB{"foo.bar", Legacy, None, true},
		VersionCaseB{"foo..bar", Legacy, Strict, false},
		VersionCaseB{"foo..bar", Legacy, Medium, true},
		VersionCaseB{"foo..bar", Legacy, None, true},
		VersionCaseB{"foo..bar.ba::z", Legacy, Strict, false},
		VersionCaseB{"foo..bar.ba::z", Legacy, Medium, true},
		VersionCaseB{"foo..bar.ba::z", Legacy, None, true},
		VersionCaseB{"foo..bar.b\xbdz", Legacy, Strict, false},
		VersionCaseB{"foo..bar.b\xbdz", Legacy, Medium, false},
		VersionCaseB{"foo..bar.b\xbdz", Legacy, None, true},
		VersionCaseB{"foo..bar.b\x00z", Legacy, Strict, false},
		VersionCaseB{"foo..bar.b\x00z", Legacy, Medium, false},
		VersionCaseB{"foo..bar.b\x00z", Legacy, None, true},
		VersionCaseB{"foo.bar.aunit=no.baz", M20, Strict, false},
		VersionCaseB{"foo.bar.UNIT=no.baz", M20, Strict, false},
		VersionCaseB{"foo.bar.unita=no.bar", M20, None, false},
		VersionCaseB{"foo.bar.target_type_is_count.baz", M20NoEquals, Medium, false},
		VersionCaseB{"foo.bar.target_type_is_count", M20NoEquals, None, false},
		VersionCaseB{"target_type_is_count.foo.bar", M20NoEquals, Strict, false},
	}
	for _, c := range cases {
		version := GetVersion(c.in)
		assert.Equal(t, c.version, version)
		assert.Equal(t, InitialValidationB([]byte(c.in), version, c.legacyValidation) == nil, c.valid)
	}
}

type Case struct {
	in  string
	out string
}

func TestDeriveCount(t *testing.T) {
	// metrics 2.0 cases with equals
	cases := []Case{
		Case{"foo.bar.unit=yes.baz", "foo.bar.unit=yesps.baz"},
		Case{"foo.bar.unit=yes", "foo.bar.unit=yesps"},
		Case{"unit=yes.foo.bar", "unit=yesps.foo.bar"},
		Case{"target_type=count.foo.unit=ok.bar", "target_type=rate.foo.unit=okps.bar"},
	}
	for _, c := range cases {
		assert.Equal(t, DeriveCount(c.in, "prefix."), c.out)
	}

	// same but with equals
	for i, c := range cases {
		cases[i] = Case{
			strings.Replace(c.in, "=", "_is_", -1),
			strings.Replace(c.out, "=", "_is_", -1),
		}
	}
	for _, c := range cases {
		assert.Equal(t, DeriveCount(c.in, "prefix."), c.out)
	}
}

// only 1 kind of stat is enough, cause they all behave the same
func TestStat(t *testing.T) {
	cases := []Case{
		Case{"foo.bar.unit=yes.baz", "foo.bar.unit=yes.baz.stat=max_90"},
		Case{"foo.bar.unit=yes", "foo.bar.unit=yes.stat=max_90"},
		Case{"unit=yes.foo.bar", "unit=yes.foo.bar.stat=max_90"},
		Case{"target_type=count.foo.unit=ok.bar", "target_type=count.foo.unit=ok.bar.stat=max_90"},
	}
	for _, c := range cases {
		assert.Equal(t, Max(c.in, "prefix.", "90", ""), c.out)
	}
	// same but with equals and no percentile
	for i, c := range cases {
		cases[i] = Case{
			strings.Replace(c.in, "=", "_is_", -1),
			strings.Replace(strings.Replace(c.out, "=", "_is_", -1), "max_90", "max", 1),
		}
	}
	for _, c := range cases {
		assert.Equal(t, Max(c.in, "prefix.", "", ""), c.out)
	}
}
func TestRateCountPckt(t *testing.T) {
	cases := []Case{
		Case{"foo.bar.unit=yes.baz", "foo.bar.unit=Pckt.baz.orig_unit=yes.pckt_type=sent.direction=in"},
		Case{"foo.bar.unit=yes", "foo.bar.unit=Pckt.orig_unit=yes.pckt_type=sent.direction=in"},
		Case{"unit=yes.foo.bar", "unit=Pckt.foo.bar.orig_unit=yes.pckt_type=sent.direction=in"},
		Case{"target_type=count.foo.unit=ok.bar", "target_type=count.foo.unit=Pckt.bar.orig_unit=ok.pckt_type=sent.direction=in"},
	}
	for _, c := range cases {
		assert.Equal(t, CountPckt(c.in, "prefix."), c.out)
		c = Case{
			c.in,
			strings.Replace(strings.Replace(c.out, "unit=Pckt", "unit=Pcktps", -1), "target_type=count", "target_type=rate", -1),
		}
		assert.Equal(t, RatePckt(c.in, "prefix."), c.out)
	}
	for _, c := range cases {
		c = Case{
			strings.Replace(c.in, "=", "_is_", -1),
			strings.Replace(c.out, "=", "_is_", -1),
		}
		assert.Equal(t, CountPckt(c.in, "prefix."), c.out)
		c = Case{
			c.in,
			strings.Replace(strings.Replace(c.out, "unit_is_Pckt", "unit_is_Pcktps", -1), "target_type_is_count", "target_type_is_rate", -1),
		}
		assert.Equal(t, RatePckt(c.in, "prefix."), c.out)
	}
}

func BenchmarkManyDeriveCounts(t *testing.B) {
	for i := 0; i < 1000000; i++ {
		DeriveCount("foo.bar.unit=yes.baz", "prefix.")
		DeriveCount("foo.bar.unit=yes", "prefix.")
		DeriveCount("unit=yes.foo.bar", "prefix.")
		DeriveCount("foo.bar.unita=no.bar", "prefix.")
		DeriveCount("foo.bar.aunit=no.baz", "prefix.")
	}
}
