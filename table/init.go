package table

import logging "github.com/op/go-logging"

var log = logging.MustGetLogger("table") // for tests. overridden by main

func SetLogger(l *logging.Logger) {
	log = l
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
