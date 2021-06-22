package test

import (
	"io/ioutil"
	"os"
	"testing"
)

// TempFdOrFatal provides a temp file with the given contents, or Fatals if any error happens
func TempFdOrFatal(fPattern, contents string, t *testing.T) *os.File {
	fd, err := ioutil.TempFile("", fPattern)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fd.Write([]byte(contents)); err != nil {
		t.Fatal(err)
	}
	if err := fd.Close(); err != nil {
		t.Fatal(err)
	}

	return fd
}
