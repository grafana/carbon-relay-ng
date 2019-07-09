package destination

import (
	"testing"

	"github.com/graphite-ng/carbon-relay-ng/encoding"
	"github.com/stretchr/testify/assert"
)

func buildKeepSafe() *keepSafe {
	return &keepSafe{
		initialCap: keepsafe_initial_cap,
		safeOld:    make([]encoding.Datapoint, 0, keepsafe_initial_cap),
		safeRecent: make([]encoding.Datapoint, 0, keepsafe_initial_cap),
		periodKeep: keepsafe_keep_duration,
		closed:     make(chan struct{}),
	}
}

func buildDatapoint() encoding.Datapoint {
	return encoding.Datapoint{
		Name:      "toto",
		Timestamp: 1,
		Value:     1,
	}
}

func TestAddedDatapointShouldDisappearWhenExpire(t *testing.T) {
	ks := buildKeepSafe()
	dp := buildDatapoint()

	ks.Add(dp)
	assert.Equal(t, len(ks.safeRecent), 1)
	assert.Equal(t, len(ks.safeOld), 0)

	ks.expire()
	assert.Equal(t, len(ks.safeRecent), 0)
	assert.Equal(t, len(ks.safeOld), 1)

	ks.expire()
	assert.Equal(t, len(ks.safeRecent), 0)
	assert.Equal(t, len(ks.safeOld), 0)
}

func TestGetAllShouldReturnRecentAndNewDatapointsWhenCalled(t *testing.T) {
	ks := buildKeepSafe()
	dp := buildDatapoint()

	ks.Add(dp)
	ks.expire()

	ks.Add(dp)
	assert.Equal(t, len(ks.safeRecent), 1)
	assert.Equal(t, len(ks.safeOld), 1)

	all := ks.GetAll()
	assert.Equal(t, len(all), 2)
	assert.Equal(t, len(ks.safeRecent), 0)
	assert.Equal(t, len(ks.safeOld), 0)
}

func TestSafeOldSliceShouldBeReallocatedWhenCapacityIsTooHigh(t *testing.T) {
	ks := buildKeepSafe()
	ks.safeOld = make([]encoding.Datapoint, 0, keepsafe_initial_cap*3)

	ks.expire()
	assert.Equal(t, cap(ks.safeOld), keepsafe_initial_cap)
}
