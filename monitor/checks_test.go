package monitor

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type dummyService struct {
	sync.RWMutex
	ProcessCheck
	doError      bool
	startTime    time.Duration
	stopTime     time.Duration
	running      bool
	timesStarted int
}

func (ds *dummyService) getTimesStarted() int {
	defer ds.RUnlock()
	ds.RLock()
	return ds.timesStarted
}

func (ds *dummyService) setTimesStarted(t int) {
	defer ds.Unlock()
	ds.Lock()
	ds.timesStarted = t
}

func newDummyService(id string) *dummyService {
	s := dummyService{ProcessCheck: ProcessCheck{check: &check{ID: id}}}
	s.startTime = 5 * time.Millisecond
	s.stopTime = 5 * time.Millisecond
	s.doError = false
	return &s
}

func (ds *dummyService) IsRunning() bool {
	defer ds.RUnlock()
	ds.RLock()
	return ds.running
}
func (ds *dummyService) IsNotRunning() bool {
	return !ds.IsRunning()
}
func (ds *dummyService) Status() (str string) {
	if ds.IsRunning() {
		str = "running"
	} else {
		str = "stopped"
	}
	return str
}

func (ds *dummyService) Start() error {
	if ds.IsRunning() {
		return nil
	}
	defer ds.Unlock()
	ds.Lock()

	if ds.doError {
		return fmt.Errorf("Error starting service %s", ds.GetID())
	}
	time.Sleep(ds.startTime)
	ds.running = true
	ds.timesStarted++
	return nil
}
func (ds *dummyService) Stop() error {
	if ds.IsNotRunning() {
		return nil
	}
	defer ds.Unlock()
	ds.Lock()
	if ds.doError {
		return fmt.Errorf("Error stopping service %s", ds.GetID())
	}
	time.Sleep(ds.stopTime)
	ds.running = false
	return nil
}

func (ds *dummyService) Restart() error {
	if err := ds.Stop(); err != nil {
		return err
	}
	if err := ds.Start(); err != nil {
		return err
	}
	return nil
}

type dummyCheck struct {
	sync.RWMutex
	ProcessCheck
	timesCalled int
	waitTime    time.Duration
}

func (dc *dummyCheck) getWaitTime() time.Duration {
	defer dc.RUnlock()
	dc.RLock()
	return dc.waitTime
}

func (dc *dummyCheck) setWaitTime(d time.Duration) {
	defer dc.Unlock()
	dc.Lock()
	dc.waitTime = d
}

func (dc *dummyCheck) Perform() {
	time.Sleep(dc.waitTime)
	defer dc.Unlock()
	dc.Lock()
	dc.timesCalled++
}

func (dc *dummyCheck) getTimesCalled() int {
	defer dc.RUnlock()
	dc.RLock()
	return dc.timesCalled
}

func (dc *dummyCheck) setTimesCalled(t int) {
	defer dc.Unlock()
	dc.Lock()
	dc.timesCalled = t
}

func newDummyCheck(id string) *dummyCheck {
	return &dummyCheck{ProcessCheck: ProcessCheck{check: &check{ID: id}}}
}

func TestCheckOnceOneSimultaneousInvocationPerObject(t *testing.T) {
	t.Parallel()
	dc := newDummyCheck("dummy")
	dc.Initialize(Opts{})

	dc.setWaitTime(20 * time.Millisecond)
	maxCalls := 5
	for i := 0; i < maxCalls; i++ {
		CheckOnce(dc, Opts{})
	}
	time.Sleep(100 * time.Millisecond)
	tc := dc.getTimesCalled()
	assert.Equal(t, tc, 1, "Expected the number of times called to be 1 but got %d", tc)

	dc.setTimesCalled(0)
	dc.setWaitTime(1 * time.Millisecond)
	CheckOnce(dc, Opts{})
	time.Sleep(20 * time.Millisecond)
	CheckOnce(dc, Opts{})
	time.Sleep(20 * time.Millisecond)
	tc = dc.getTimesCalled()
	assert.Equal(t, tc, 2, "Expected the number of times called to be 2 but got %d", tc)
}

func TestCheckOnceTwoChecksWithSameId(t *testing.T) {
	t.Parallel()
	dc1 := newDummyCheck("dummy")
	dc1.Initialize(Opts{})
	dc1.setWaitTime(20 * time.Millisecond)

	dc2 := newDummyCheck("dummy")
	dc2.Initialize(Opts{})
	dc2.setWaitTime(20 * time.Millisecond)

	assert.Equal(t, dc1.GetID(), dc2.GetID(), "Expected to get same id in dummy checks")

	maxCalls := 10
	for i := 0; i < maxCalls; i++ {
		CheckOnce(dc1, Opts{})
		CheckOnce(dc2, Opts{})
	}

	time.Sleep(100 * time.Millisecond)
	for _, c := range []*dummyCheck{dc1, dc2} {
		tc := c.getTimesCalled()
		assert.Equal(t, tc, 1, "Expected the number of times called to be 1 but got %d", tc)
	}

	dc1.setTimesCalled(0)
	dc1.setWaitTime(1 * time.Millisecond)

	dc2.setTimesCalled(0)
	dc2.setWaitTime(1 * time.Millisecond)

	for i := 0; i < maxCalls; i++ {
		CheckOnce(dc1, Opts{})
		CheckOnce(dc2, Opts{})
		// This should give plenty of time for all calls to finish
		time.Sleep(100 * time.Millisecond)
	}

	for _, c := range []*dummyCheck{dc1, dc2} {
		tc := c.getTimesCalled()
		assert.Equal(t, tc, maxCalls,
			"Expected the number of times called to be %d but got %d", maxCalls, tc)
	}
}
