package matcher

import (
	"testing"
)

func BenchmarkMatchPrefix(b *testing.B) {
	matcher, _ := New("abcde_fghij.klmnopqrst", "", "")
	metric70 := []byte("abcde_fghij.klmnopqrst.uv_wxyz.1234567890abcdefg 12345.6789 1234567890") // key = 48, val = 10, ts = 10 -> 70
	for i := 0; i < b.N; i++ {
		matcher.Match(metric70)
	}
}

func BenchmarkMatchSubstr(b *testing.B) {
	matcher, _ := New("", "1234567890abc", "")
	metric70 := []byte("abcde_fghij.klmnopqrst.uv_wxyz.1234567890abcdefg 12345.6789 1234567890") // key = 48, val = 10, ts = 10 -> 70
	for i := 0; i < b.N; i++ {
		matcher.Match(metric70)
	}
}

func BenchmarkMatchRegex(b *testing.B) {
	matcher, _ := New("", "", "abcde_(fghij|foo).[^\\.]+.\\.*.\\.*")
	metric70 := []byte("abcde_fghij.klmnopqrst.uv_wxyz.1234567890abcdefg 12345.6789 1234567890") // key = 48, val = 10, ts = 10 -> 70
	for i := 0; i < b.N; i++ {
		matcher.Match(metric70)
	}
}
