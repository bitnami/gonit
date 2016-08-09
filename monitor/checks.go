package monitor

import (
	"fmt"
	"os/exec"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bitnami/gonit/utils"

	"github.com/bitnami/gonit/log"
)

// CheckMaxStartTries defines a global default value for how many failed start attempts are made
// before automatically unmonitoring a check
const CheckMaxStartTries = 5

type syncValue struct {
	sync.RWMutex
	value interface{}
}

func (sv *syncValue) get() interface{} {
	return sv.value
}
func (sv *syncValue) Get() interface{} {
	defer sv.RUnlock()
	sv.RLock()
	return sv.get()
}
func (sv *syncValue) set(v interface{}) {
	sv.value = v
}
func (sv *syncValue) Set(v interface{}) {
	defer sv.Unlock()
	sv.Lock()
	sv.set(v)
}

type syncBool struct {
	syncValue
}

func (b *syncBool) Get() bool {
	return b.syncValue.Get().(bool)
}

type syncTime struct {
	syncValue
}

func (t *syncTime) Get() time.Time {
	defer t.RUnlock()
	t.RLock()
	v := t.syncValue.get()
	if v == nil {
		return time.Time{}
	}
	return v.(time.Time)
}

type checkState struct {
	syncBool
}

func (cs *checkState) SetInProgressState(state bool) {
	cs.Set(state)
}

func (cs *checkState) Take() (success bool) {
	cs.Lock()
	defer cs.Unlock()
	if cs.get().(bool) {
		return false
	}
	cs.set(true)
	return true
}

var (
	checkStates = make(map[string]*checkState)
	mutex       = &sync.Mutex{}
)

func startProcess(p interface {
	CheckableProcess
}) error {
	return p.Start()
}

func stopProcess(p interface {
	CheckableProcess
}) error {
	return p.Stop()
}

func restartProcess(p interface {
	CheckableProcess
}) error {
	return p.Restart()
}

func doOnce(id string, cb func(), timeout time.Duration, opts Opts) (blocked bool) {
	logger := opts.Logger
	if logger == nil {
		logger = log.DummyLogger()
	}
	mutex.Lock()
	s, ok := checkStates[id]
	if !ok || s == nil {
		s = &checkState{}
		s.value = false
		checkStates[id] = s
	}
	mutex.Unlock()
	if !s.Take() {
		logger.Warnf("A previous operation for %s is still in process", id)
		return true
	}
	cleanUp := func() {
		s.SetInProgressState(false)
	}

	go func() {
		timer := time.AfterFunc(timeout, func() {
			logger.Debugf("Execution mutex for %s expired. Cleaning up...", id)
			cleanUp()
		})
		defer func() {
			timer.Stop()
			cleanUp()
		}()
		cb()
	}()
	return false
}

// CheckOnce calls the provided check Perform operation once, ignoring the call
// if a previous call is still in process
func CheckOnce(c interface {
	Checkable
}, opts Opts) bool {

	id := c.GetUniqueID()

	timeout := c.GetTimeout() + (5 * time.Second)
	cb := c.Perform
	return doOnce(id, cb, timeout, opts)
}

// Checkable defines the interface the every Check must provide
type Checkable interface {
	GetID() string
	GetUniqueID() string
	GetTimeout() time.Duration
	Perform()
	SetMonitored(bool)
	IsMonitored() bool
	Parse(data string)
	String() string
	Initialize(Opts)
	SummaryText() string
}

// CheckableProcess defines the interface the every process Check (type service) must provide
type CheckableProcess interface {
	Checkable
	Start() error
	Stop() error
	Restart() error
	Status() string
	IsRunning() bool
	IsNotRunning() bool
	Uptime() time.Duration
	Pid() int
}

// Check implements the basic check type
type check struct {
	// ID contains the check identifier
	ID string
	// Timeout configures for how long to wait for the check to finish its operation
	// before aborting it
	Timeout time.Duration
	// Monitored configures wether the check is taken into account by the monitor or not.
	// If not, it won't be automatically started in case of unhandled stops
	monitored syncBool
	logger    Logger
	lastCheck time.Time
}

// GetTimeout returns the check Timeout
func (c *check) GetTimeout() time.Duration {
	return c.Timeout
}

// GetUniqueID returns a globally unique ide for the check.
// Two instances of the same check with the same ID will have
// different UniqueId
func (c *check) GetUniqueID() string {
	return fmt.Sprintf("%s-%d", c.GetID(), reflect.ValueOf(c).Pointer())
}

// GetID returns the check ID
func (c *check) GetID() string {
	return c.ID
}
func (c *check) Initialize(opts Opts) {
	if opts.Logger != nil {
		c.logger = opts.Logger
	}
	c.SetMonitored(true)
}

func (c *check) getMonitoredString() (str string) {
	if c.IsMonitored() {
		str = "monitored"
	} else {
		str = "Not monitored"
	}
	return str
}

// SummaryText returns a string the a short summary of the check status:
// Check 'Check ID'      monitored
func (c *check) SummaryText() string {
	return fmt.Sprintf("Check '%s'%40s", c.ID, c.getMonitoredString())
}

// String returns a string representation for the check
func (c *check) String() string {
	return fmt.Sprintf("Check %s\n  monitoring status %40s\n", c.ID, c.getMonitoredString())
}

// Perform make the Check execute its "task". In this basic base check type
func (c *check) Perform() {
	c.logger.Infof("Performing check", c.ID)
}

func (c *check) Parse(data string) {

}

func (c *check) IsMonitored() bool {
	return c.monitored.Get()
}

func (c *check) SetMonitored(monitored bool) {
	c.monitored.Set(monitored)
}

func newCheck(id string, kind string) interface {
	Checkable
} {
	check := check{Timeout: 120 * time.Second, ID: id, logger: log.DummyLogger()}
	switch kind {
	case "process":
		return &ProcessCheck{check: check}
	default:
		return &check
	}
}

func newCheckFromData(data string) (interface {
	Checkable
}, error) {
	checkPattern := "(\n|^)check\\s+([^\\s]+)\\s+([^\\s]+)((.|\n)*)"
	re := regexp.MustCompile(checkPattern)
	if match := re.FindStringSubmatch(data); match != nil {
		kind := match[2]
		id := match[3]
		config := match[4]
		c := newCheck(id, kind)
		c.Parse(config)
		return c, nil
	}
	return nil, fmt.Errorf("Cannot parse check definition")
}

// Command defines a check command to execute
type Command struct {
	// Cmd contains the actual commad to call
	Cmd string
	// Timeout defines the time to wait for the command to trigger
	// a state change in the check (for example, running to stopped after calling stop)
	Timeout time.Duration
	logger  Logger
}

func newCommand(cmdStr string, timeout time.Duration, opts Opts) *Command {
	cmd := &Command{Cmd: unquote(cmdStr), Timeout: timeout}
	if opts.Logger != nil {
		cmd.logger = opts.Logger
	} else {
		cmd.logger = log.DummyLogger()
	}
	return cmd
}

// Exec performs the actual command execution
func (c *Command) Exec() {
	// TODO REPORT error, track std streams
	c.logger.Debugf("/bin/bash -c %s", c.Cmd)
	c.logger.Debug(exec.Command("/bin/bash", "-c", c.Cmd).Run())
}

func formatColumns(len int, args ...interface{}) string {
	res := ""
	for _, str := range args {
		res += fmt.Sprintf(fmt.Sprintf("%%-%ds", len), fmt.Sprintln(str))
	}
	return res
}
func (c *ProcessCheck) getStatusString() (str string) {
	if c.IsMonitored() {
		if c.IsRunning() {
			str = "Running"
		} else {
			str = "Stopped"
		}
	} else {
		str = c.getMonitoredString()
	}
	return str
}

// SummaryText returns a string the a short summary of the check status:
// Process id       monitored
func (c *ProcessCheck) SummaryText() string {
	return fmt.Sprintf("Process %-10s%40s", c.ID, c.getStatusString())
}

// String returns a string representation for the process check
func (c *ProcessCheck) String() string {
	s := fmt.Sprintf("Process '%s'\n", c.ID)
	if c.IsMonitored() {
		s += fmt.Sprintf("  %-40s %12s\n", "status", c.getStatusString())
		if c.IsRunning() {
			s += fmt.Sprintf("  %-40s %12d\n", "pid", c.Pid())
		}
		s += fmt.Sprintf("  %-40s %12v\n", "uptime", utils.RoundDuration(c.Uptime()))
		s += fmt.Sprintf("  %-40s %12s\n", "monitoring status", "monitored")
	} else {
		s += fmt.Sprintf("  %-40s %12s\n", "monitoring status", "Not monitored")
	}
	return s
}

// Initialize fills up any unconfigured process attributes,
// for example, the logger
func (c *ProcessCheck) Initialize(opts Opts) {
	c.check.Initialize(opts)
	// Ensure the programs are not nil
	if c.StartProgram == nil {
		c.StartProgram = newCommand("", c.Timeout, opts)
	}

	if c.StopProgram == nil {
		c.StopProgram = newCommand("", c.Timeout, opts)
	}

	// TODO: Is this really needed?
	c.StartProgram.logger = c.logger
	c.StopProgram.logger = c.logger

	if c.StartProgram.Timeout == 0 {
		c.StartProgram.Timeout = c.Timeout
	}
	if c.StopProgram.Timeout == 0 {
		c.StopProgram.Timeout = c.Timeout
	}
	if c.IsRunning() {
		c.startedAt.Set(time.Now())
	}
}

// Perform makes the process check execute its default task. In case of
// process type checks, calling its start command and waiting for the
// status to change to "running"
func (c *ProcessCheck) Perform() {
	c.logger.Infof("Performing process check %s", c.ID)
	c.logger.MDebugf(c.String())
	if c.IsMonitored() && !c.IsRunning() {
		c.logger.Infof("Service %s is not running. Starting...", c.ID)
		go c.start()
		iterationTime := 500 * time.Millisecond
		// TODO: Use utils.WaitUntil
		timoutTimer := time.NewTimer(c.StartProgram.Timeout)

		defer timoutTimer.Stop()

		iteratorTimer := time.NewTimer(iterationTime)
		defer iteratorTimer.Stop()
		i := 0
		maxTries := c.maxStartTries
		if maxTries == 0 {
			maxTries = CheckMaxStartTries
		}

	Loop:
		for {
			if c.IsRunning() {
				c.StartTrialsCnt = 0
				c.startedAt.Set(time.Now())
				c.logger.Debugf("%s successfully started", c.ID)
				break
			}
			select {
			case <-iteratorTimer.C:
				c.logger.Debugf("Waiting for %s to be running (%ds)", c.ID, i)
				i++
				iteratorTimer.Reset(iterationTime)
			case <-timoutTimer.C:
				//				iteratorTimer.Stop()
				c.StartTrialsCnt++
				c.logger.Warnf("Timed out waiting for %s to start (%d tries left)", c.ID, maxTries-c.StartTrialsCnt)
				break Loop
			}
		}

		if c.StartTrialsCnt >= maxTries {
			c.SetMonitored(false)
			c.StartTrialsCnt = 0
			c.logger.Warnf("%s was unmonitored after %d failed tries", c.ID, CheckMaxStartTries)
		}
	}
}

// Restart restarts tge service by calling its stop and restart commands and
// waiting for the checck to be in running status
func (c *ProcessCheck) Restart() (err error) {
	c.logger.Debugf("Restarting %s", c.GetID())
	if err = c.Stop(); err == nil {
		err = c.Start()
	}
	if err != nil {
		return fmt.Errorf("Failed to restart %s", c.GetID())
	}
	return err
}

func (c *ProcessCheck) start() {
	c.logger.Debugf("Starting %s", c.GetID())
	c.SetMonitored(true)
	if c.IsRunning() {
		c.logger.Debugf("%s is already running", c.GetID())
		return
	}
	// Even if it fails, we reset the time
	// TODO: Find a better name for the variable if it doesn't
	// exactly mean "startedAt" but more "resetTime"
	defer func() {
		c.startedAt.Set(time.Now())
	}()
	c.StartProgram.Exec()
}

// Start starts the process by calling its start command and waiting
// for the checck to be in running status
func (c *ProcessCheck) Start() error {
	go c.start()
	if !utils.WaitUntil(c.IsRunning, c.StartProgram.Timeout) {
		return fmt.Errorf("Failed to start %s", c.GetID())
	}
	return nil
}

func (c *ProcessCheck) stop() {
	c.logger.Debugf("Stopping %s", c.GetID())
	c.SetMonitored(false)
	if !c.IsRunning() {
		c.logger.Debugf("%s is already stopped", c.GetID())
		return
	}
	c.StopProgram.Exec()
}

// Stop stops the  process by calling its stop command and waiting
// for the checck to be in stopped status
func (c *ProcessCheck) Stop() error {
	go c.stop()
	if !utils.WaitUntil(c.IsNotRunning, c.StopProgram.Timeout) {
		return fmt.Errorf("Failed to stop %s", c.GetID())
	}
	return nil
}

// Pid returns the pid of the process by reading its pid file.
// It will return -1 in case of no pid file found or it is malformed
func (c *ProcessCheck) Pid() int {
	pid, _ := utils.ReadPid(c.PidFile)
	return pid
}

// IsRunning returns true if the process is running
func (c *ProcessCheck) IsRunning() bool {
	return utils.IsProcessRunning(c.Pid())
}

// IsNotRunning returns true if the process is not running
func (c *ProcessCheck) IsNotRunning() bool {
	return !c.IsRunning()
}

// Status returns a string specifying the process
// status ("running" or "stopped")
func (c *ProcessCheck) Status() (str string) {
	if c.IsRunning() {
		str = "running"
	} else {
		str = "stopped"
	}
	return str
}

// ProcessCheck defines a service type check
type ProcessCheck struct {
	check
	Group          string
	PidFile        string
	StartProgram   *Command
	StartTrialsCnt int
	StopProgram    *Command
	startedAt      syncTime
	maxStartTries  int
}

// Uptime returns for how long the process have been running
func (c *ProcessCheck) Uptime() time.Duration {
	if !c.IsMonitored() || !c.IsRunning() {
		return time.Duration(0)
	}
	if c.startedAt.Get().Equal(time.Time{}) {
		return time.Duration(0)
	}
	return time.Now().Sub(c.startedAt.Get())
}

func unquote(str string) string {
	return strings.Trim(str, "\"")
}

// We should make this generic for all Checks
func parseWithTimeout(data string) (time.Duration, error) {

	withTimeoutRe := regexp.MustCompile("with\\s+timeout\\s+([^\\s]+)\\s+(millisecond|second|minute|hour|day)s?")
	t := withTimeoutRe.FindStringSubmatch(data)
	if t == nil {
		return 0, nil
	}

	n, err := strconv.Atoi(t[1])
	if err != nil {
		return 0, err
	}

	duration := time.Duration(n)
	unit := strings.ToLower(t[2])
	switch t[2] {
	case "millisecond":
		duration *= time.Millisecond
	case "second":
		duration *= time.Second
	case "minute":
		duration *= time.Minute
	case "hour":
		duration *= time.Hour
	case "day":
		duration *= time.Hour * 24
	default:
		return 0, fmt.Errorf("Unknown unit %s", unit)
	}
	return duration, nil
}

// Parse reads a string containing a monit-like process configuration text
// and loads the specified settings
func (c *ProcessCheck) Parse(data string) {

	withRe := regexp.MustCompile("with\\s+([^\\s]+)\\s+([^\\s]+)")
	groupRe := regexp.MustCompile("group\\s+([^\\s]+)")
	startRe := regexp.MustCompile("start\\s+program\\s+=\\s+(\"[^\"]+\"|[^\\s]+)([^\n]*)")
	stopRe := regexp.MustCompile("stop\\s+program\\s+=\\s+(\"[^\"]+\"|[^\\s]+)([^\n]*)")
	ifRe := regexp.MustCompile("if\\s+([^\n]+)")
	processOptRe := regexp.MustCompile(
		fmt.Sprintf("^[\\s\n]*(%s|%s|%s|%s|%s)",
			groupRe.String(),
			startRe.String(),
			stopRe.String(),
			ifRe.String(),
			withRe.String(),
		))

	toParse := data

	for {
		matchIdx := processOptRe.FindStringSubmatchIndex(toParse)

		if matchIdx == nil {
			break
		}

		statement := strings.TrimSpace(toParse[matchIdx[2]:matchIdx[3]])
		toParse = toParse[matchIdx[1]:]

		// TODO: Unify startRe and stopRe
		switch {
		case groupRe.MatchString(statement):
			m := groupRe.FindStringSubmatch(statement)
			c.Group = unquote(m[1])
		case startRe.MatchString(statement):
			m := startRe.FindStringSubmatch(statement)
			cmdStr := unquote(m[1])
			timeout, err := parseWithTimeout(m[2])
			if err != nil {
				c.logger.Warnf(err.Error())
			}
			c.StartProgram = newCommand(cmdStr, timeout, Opts{Logger: c.logger})
		case stopRe.MatchString(statement):
			m := stopRe.FindStringSubmatch(statement)
			cmdStr := unquote(m[1])
			timeout, err := parseWithTimeout(m[2])
			if err != nil {
				c.logger.Warnf(err.Error())
			}
			c.StopProgram = newCommand(cmdStr, timeout, Opts{Logger: c.logger})
		case withRe.MatchString(statement):
			m := withRe.FindStringSubmatch(statement)
			withKind := m[1]
			if withKind == "pidfile" {
				c.PidFile = unquote(m[2])
			} else {
				c.logger.Warnf("Don't know how to interpret \"with %s\"", withKind)
			}
		default:
			c.logger.Debugf("Ignoring statement %s", statement)
		}
	}
}
