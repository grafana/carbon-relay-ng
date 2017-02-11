package aggregator

import logging "github.com/op/go-logging"

var log = logging.MustGetLogger("aggregator") // for tests. overridden by main

func SetLogger(l *logging.Logger) {
	log = l
}
