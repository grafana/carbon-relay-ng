package manager

import (
	"time"

	"github.com/grafana/carbon-relay-ng/input"
	log "github.com/sirupsen/logrus"
)

// Stop shuts down all given input plugins and returns whether it was successfull.
func Stop(inputs []input.Plugin, timeout time.Duration) bool {
	results := make(chan bool)
	for _, plugin := range inputs {
		go func(plugin input.Plugin) {
			log.Infof("Shutting down %s input", plugin.Name())
			res := plugin.Stop()
			if res {
				log.Infof("%s input finished shutdown successfully", plugin.Name())
			} else {
				log.Errorf("%s input failed to shutdown cleanly", plugin.Name())
			}
			results <- res
		}(plugin)
	}
	complete := make(chan bool)
	go func() {
		count := 0
		success := true
		for res := range results {

			count++
			success = success && res

			if count == len(inputs) {
				complete <- success
				return
			}
		}
	}()

	select {
	case res := <-complete:
		return res
	case <-time.After(timeout):
		log.Error("Input plugins taking too long to shutdown, not waiting any longer.")
		return false
	}
}
