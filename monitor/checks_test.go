package monitor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCheckOnceOneSimultaneousInvocationPerObject(t *testing.T) {
	t.Parallel()
	dc := &dummyCheck{}
	dc.Initialize(Opts{})

	dc.waitTime = 20 * time.Millisecond
	maxCalls := 5
	for i := 0; i < maxCalls; i++ {
		CheckOnce(dc, Opts{})
	}
	time.Sleep(100 * time.Millisecond)
	tc := dc.timesCalled
	assert.Equal(t, tc, 1, "Expected the number of times called to be 1 but got %d", tc)

	dc.timesCalled = 0
	dc.waitTime = 1 * time.Millisecond
	CheckOnce(dc, Opts{})
	time.Sleep(20 * time.Millisecond)
	CheckOnce(dc, Opts{})
	time.Sleep(20 * time.Millisecond)
	tc = dc.timesCalled
	assert.Equal(t, tc, 2, "Expected the number of times called to be 2 but got %d", tc)
}

func TestCheckOnceTwoChecksWithSameId(t *testing.T) {
	t.Parallel()
	dc1 := &dummyCheck{}
	dc1.Initialize(Opts{})
	dc1.waitTime = 20 * time.Millisecond

	dc2 := &dummyCheck{}
	dc2.Initialize(Opts{})
	dc2.waitTime = 20 * time.Millisecond

	assert.Equal(t, dc1.GetID(), dc2.GetID(), "Expected to get same id in dummy checks")

	maxCalls := 10
	for i := 0; i < maxCalls; i++ {
		CheckOnce(dc1, Opts{})
		CheckOnce(dc2, Opts{})
	}

	time.Sleep(100 * time.Millisecond)
	for _, c := range []*dummyCheck{dc1, dc2} {
		tc := c.timesCalled
		assert.Equal(t, tc, 1, "Expected the number of times called to be 1 but got %d", tc)
	}

	dc1.timesCalled = 0
	dc1.waitTime = 1 * time.Millisecond

	dc2.timesCalled = 0
	dc2.waitTime = 1 * time.Millisecond

	for i := 0; i < maxCalls; i++ {
		CheckOnce(dc1, Opts{})
		CheckOnce(dc2, Opts{})
		// This should give plenty of time for all calls to finish
		time.Sleep(100 * time.Millisecond)
	}

	for _, c := range []*dummyCheck{dc1, dc2} {
		tc := c.timesCalled
		assert.Equal(t, tc, maxCalls,
			"Expected the number of times called to be %d but got %d", maxCalls, tc)
	}
}
