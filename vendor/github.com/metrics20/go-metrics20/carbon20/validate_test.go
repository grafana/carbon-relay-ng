package carbon20

import (
	"math"
	"testing"

	"github.com/bmizerany/assert"
)

func TestValidateLegacy(t *testing.T) {
	cases := []struct {
		in    string
		level ValidationLevelLegacy
		valid bool
	}{
		{"foo.bar", StrictLegacy, true},
		{"foo.bar", MediumLegacy, true},
		{"foo.bar", NoneLegacy, true},
		{"foo..bar", StrictLegacy, false},
		{"foo..bar", MediumLegacy, true},
		{"foo..bar", NoneLegacy, true},
		{"foo..bar.ba::z", StrictLegacy, false},
		{"foo..bar.ba::z", MediumLegacy, true},
		{"foo..bar.ba::z", NoneLegacy, true},
		{"foo..bar.b\xbdz", StrictLegacy, false},
		{"foo..bar.b\xbdz", MediumLegacy, false},
		{"foo..bar.b\xbdz", NoneLegacy, true},
		{"foo..bar.b\x00z", StrictLegacy, false},
		{"foo..bar.b\x00z", MediumLegacy, false},
		{"foo..bar.b\x00z", NoneLegacy, true},

		// now some tag test cases.
		// note that we have a dedicated function to test
		// the just tag appendices.
		// but here we're more interested in the "overall"
		// behavior
		{"foo.bar;", StrictLegacy, false},
		{"foo.bar;", MediumLegacy, false},
		{"foo.bar;", NoneLegacy, true},
		{"foo.bar;k", StrictLegacy, false},
		{"foo.bar;k", MediumLegacy, false},
		{"foo.bar;k", NoneLegacy, true},
		{"foo.bar;k=", StrictLegacy, false},
		{"foo.bar;k=", MediumLegacy, false},
		{"foo.bar;k=", NoneLegacy, true},
		{"foo.bar;k=v", StrictLegacy, true},
		{"foo.bar;k=v", MediumLegacy, true},
		{"foo.bar;k=v", NoneLegacy, true},
		{"foo.bar;k!=v", StrictLegacy, false},
		{"foo.bar;k!=v", MediumLegacy, false},
		{"foo.bar;k!=v", NoneLegacy, true},
		// non-ascii also not allowed in tag appendix
		{"foo.bar;k=\xbdz", StrictLegacy, false},
		{"foo.bar;k=\xbdz", MediumLegacy, false},
		{"foo.bar;k=\xbdz", NoneLegacy, true},
		{"foo.bar;k=\x00z", StrictLegacy, false},
		{"foo.bar;k=\x00z", MediumLegacy, false},
		{"foo.bar;k=\x00z", NoneLegacy, true},
	}
	for i, c := range cases {
		err := ValidateKeyLegacy(c.in, c.level)
		if (err == nil) != c.valid {
			t.Fatalf("test %d: ValidateKeyLegacy(%q,%v): expected valid=%t, got err=%v", i, c.in, c.level, c.valid, err)
		}
		err = ValidateKeyLegacyB([]byte(c.in), c.level)
		if (err == nil) != c.valid {
			t.Fatalf("test %d: ValidateKeyLegacyB(%q,%v): expected valid=%t, got err=%v", i, c.in, c.level, c.valid, err)
		}
	}
}

func TestValidateTimestamps(t *testing.T) {
	cases := []struct {
		in    string
		valid bool
	}{
		{"foo.bar 1 123", true},
		{"foo.bar 1 123.0", true},
		{"foo.bar 1 123abc", false},
	}

	for _, c := range cases {
		_, _, _, err := ValidatePacket([]byte(c.in), NoneLegacy, NoneM20)
		valid := err == nil
		if valid != c.valid {
			t.Errorf("in='%s' valid=%v, expected %v", c.in, valid, c.valid)
		}
	}
}

func TestValidateValues(t *testing.T) {
	cases := []struct {
		in    string
		value float64
		valid bool
	}{
		{"foo.bar -1 123", -1, true},
		{"foo.bar 1e5 123", 1e5, true},
		{"foo.bar 1E+5 123", 1e5, true},
		{"foo.bar +1E-5 123", 1e-5, true},
		{"foo.bar 1e100 123", 1e100, true},
		{"foo.bar z1 123", 0, false},
		{"foo.bar ++1 123", 0, false},
	}

	for _, c := range cases {
		_, val, _, err := ValidatePacket([]byte(c.in), NoneLegacy, NoneM20)
		valid := err == nil
		if valid != c.valid {
			t.Errorf("in='%s' valid=%v, expected %v", c.in, valid, c.valid)
		}
		if math.Abs(val-c.value) > 0.0001 {
			t.Errorf("in='%s', value=%v, expected %v", c.in, val, c.value)
		}
	}
}

func TestValidateM20(t *testing.T) {
	cases := []struct {
		in    string
		level ValidationLevelM20
		valid bool
	}{
		{"foo.bar.aunit=no.baz", MediumM20, false},
		{"foo.bar.UNIT=no.baz", MediumM20, false},
		{"foo.bar.unita=no.bar", MediumM20, false},
		{"foo.bar.aunit=no.baz", NoneM20, true},
		{"foo.bar.UNIT=no.baz", NoneM20, true},
		{"foo.bar.unita=no.bar", NoneM20, true},
	}
	for _, c := range cases {
		assert.Equal(t, ValidateKeyM20(c.in, c.level) == nil, c.valid)
		assert.Equal(t, ValidateKeyM20B([]byte(c.in), c.level) == nil, c.valid)
	}
}
func TestValidateM20NoEquals(t *testing.T) {
	cases := []struct {
		in    string
		level ValidationLevelM20
		valid bool
	}{
		{"foo.bar.mtype_is_count.baz", MediumM20, false},
		{"foo.bar.mtype_is_count", MediumM20, false},
		{"mtype_is_count.foo.bar", MediumM20, false},
		{"foo.bar.mtype_is_count.baz", NoneM20, true},
		{"foo.bar.mtype_is_count", NoneM20, true},
		{"mtype_is_count.foo.bar", NoneM20, true},
	}
	for _, c := range cases {
		assert.Equal(t, ValidateKeyM20NoEquals(c.in, c.level) == nil, c.valid)
		assert.Equal(t, ValidateKeyM20NoEqualsB([]byte(c.in), c.level) == nil, c.valid)
	}
}

func TestValidateTagAppendixB(t *testing.T) {
	cases := []struct {
		in    string
		valid bool
	}{
		{";k=v", true},
		{";123456=89710", true},
		{";k=v;k2=v2", true},
		{";k=v;;k2=v2", false},
		{";!=v", false},
		{";k!=v", false},
		{";!k=v", false},
		{";k=v;", false},
		{";k=v;=", false},
		{";k=v;k2=", false},
		{";k=", false},
		{";k=v;k2=", false},
		{";;k=v", false},
		{";k=v=", false},
		{";k=v;k2==v2", false},
	}
	for _, c := range cases {
		err := ValidateTagAppendixB([]byte(c.in))
		if (err == nil) != c.valid {
			t.Fatalf("case %q : expected valid=%t, got err=%s", c.in, c.valid, err)
		}
	}
}

func BenchmarkValidatePacketNone(b *testing.B) {
	in := []byte("carbon.agents.foo.cache.overflow 123.456 1234567890")
	for i := 0; i < b.N; i++ {
		_, _, _, err := ValidatePacket(in, NoneLegacy, NoneM20)
		if err != nil {
			panic(err)
		}
	}
}
func BenchmarkValidatePacketMedium(b *testing.B) {
	in := []byte("carbon.agents.foo.cache.overflow 123.456 1234567890")
	for i := 0; i < b.N; i++ {
		_, _, _, err := ValidatePacket(in, MediumLegacy, NoneM20)
		if err != nil {
			panic(err)
		}
	}
}
func BenchmarkValidatePacketStrict(b *testing.B) {
	in := []byte("carbon.agents.foo.cache.overflow 123.456 1234567890")
	for i := 0; i < b.N; i++ {
		_, _, _, err := ValidatePacket(in, StrictLegacy, NoneM20)
		if err != nil {
			panic(err)
		}
	}
}
