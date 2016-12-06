package route

import logging "github.com/op/go-logging"

var log = logging.MustGetLogger("route") // for tests. overridden by main

func SetLogger(l *logging.Logger) {
	log = l
}
