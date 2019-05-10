package encoding

import "testing"

func BenchmarkPlainValidatePacketsStrict(B *testing.B) {

	metric := []byte("abcde.test.test.test 21300.00 12351123")
	h := NewPlain(true)
	for i := 0; i < B.N; i++ {
		h.validateKey(metric)
	}
}
func BenchmarkPlainValidatePacketsLoose(B *testing.B) {
	metric := []byte("abcde.test.test.test 21300.00 12351123")
	h := NewPlain(false)
	for i := 0; i < B.N; i++ {
		h.validateKey(metric)
	}
}

func BenchmarkPlainLoadPackets(B *testing.B) {
	metric := []byte("abcde.test.test.test 21300.00 12351123")
	h := NewPlain(false)
	for i := 0; i < B.N; i++ {
		h.Load(metric)
	}
}
