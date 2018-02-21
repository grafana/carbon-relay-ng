package route

import (
	"bytes"
	"os"
	"testing"
)

// go test -bench=. -benchmem ./route/...

func BenchmarkCompressIntoGzip(b *testing.B) {
	var raw bytes.Buffer
	f, err := os.Open("./fixtures/metrics.txt")
	if err != nil {
		b.Fatal(err)
	}
	if _, err := raw.ReadFrom(f); err != nil {
		b.Fatal(err)
	}

	for n := 0; n < b.N; n++ {
		var payload bytes.Buffer
		if err := compressWrite(&payload, raw.Bytes(), "gzip"); err != nil {
			b.Error(err)
		}
	}
}

func BenchmarkCompressIntoNoCompression(b *testing.B) {
	var raw bytes.Buffer
	f, err := os.Open("./fixtures/metrics.txt")
	if err != nil {
		b.Fatal(err)
	}
	if _, err := raw.ReadFrom(f); err != nil {
		b.Fatal(err)
	}

	for n := 0; n < b.N; n++ {
		var payload bytes.Buffer
		if err := compressWrite(&payload, raw.Bytes(), "none"); err != nil {
			b.Error(err)
		}
	}
}
