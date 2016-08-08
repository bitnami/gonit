// Package monitor provides the tools to monitor a set of system properties, for example, services
package monitor

import (
	"fmt"
	"os"
	"runtime"
	"syscall"
	"time"

	"github.com/bitnami/gonit/log"
	"github.com/bitnami/gonit/utils"
)

// Logger defines the required interface to support to be able to log
// messages
type Logger interface {
	Debugf(format string, args ...interface{})
	Infof(format string, args ...interface{})
	Printf(format string, args ...interface{})
	Warnf(format string, args ...interface{})
	Errorf(format string, args ...interface{})
	Fatalf(format string, args ...interface{})
	Panicf(format string, args ...interface{})
	Debug(args ...interface{})
	Info(args ...interface{})
	Print(args ...interface{})
	Warn(args ...interface{})
	Error(args ...interface{})
	Fatal(args ...interface{})
	Panic(args ...interface{})

	MDebugf(format string, args ...interface{})
}

// Opts defines a set of common configuration options for many of the package
// functions
type Opts struct {
	// Logger allows customizing the logger to used
	Logger Logger
}

// ChecksManager defines the interface provided by objects being able to
// manipulate checks
type ChecksManager interface {
	//	Uptime() time.Duration
	// 	UpdateDatabase() error
	//	Reload() error
	Monitor(id string) error
	Unmonitor(id string) error
	Start(id string) error
	Stop(id string) error
	Restart(id string) error
	MonitorAll() []error
	UnmonitorAll() []error
	StartAll() []error
	StopAll() []error
	RestartAll() []error
	SummaryText() string
	StatusText() string
}

// Monitor represents an instance of the monitor application
type Monitor struct {
	// Pid is the Pid of the running monitor process
	Pid int
	// PidFile contains the path in which the Monitor PID is stored
	PidFile string
	// LogFile constains the path to the Monitor log file
	LogFile string
	// StartTime is the moment in which the monitor was started
	StartTime time.Time
	// ControlFile points to the configuration file used to load the checks to perform
	ControlFile string
	// CheckInterval configures the interval between checks
	CheckInterval time.Duration
	// SocketFile contains the path to he listening Unix domain socket when the HTTP server is enabled
	SocketFile string

	lastCheck syncTime

	// Checks contains the list of registered system checks
	checks []interface {
		Checkable
	}
	logger   Logger
	database *ChecksDatabase
	server   *monitorServer
}

// New returns a new Monitor instance
func New(c Config) (*Monitor, error) {
	var logger *log.Logger
	if c.LogFile == "-" {
		logger = log.StreamLogger(os.Stdout)
	} else if c.LogFile != "" {
		logger = log.FileLogger(c.LogFile)
	} else {
		logger = log.DummyLogger()
	}

	if c.Verbose {
		logger.Level = log.DebugLevel
	}

	db, err := NewDatabase(c.StateFile)
	if err != nil {
		logger.Warnf("Error loading database file '%s': %s", c.StateFile, err.Error())
	}

	var maxCheckInterval time.Duration
	if c.CheckInterval != 0 {
		maxCheckInterval = c.CheckInterval
	} else {
		maxCheckInterval = 100 * time.Millisecond
	}

	mon := &Monitor{
		Pid:           syscall.Getpid(),
		LogFile:       c.LogFile,
		PidFile:       c.PidFile,
		StartTime:     time.Now(),
		CheckInterval: maxCheckInterval,
		ControlFile:   c.ControlFile,
		logger:        logger,
		database:      db,
	}

	// Do we need this? Should it be calle uptime or start time?
	// if db != nil {
	//	db.Set("uptime", time.Now())
	// }

	if c.ControlFile != "" {
		utils.EnsureSafePermissions(c.ControlFile)
		loader := &configLoader{app: mon, Logger: logger}

		if err := new(configParser).ParseConfigFile(c.ControlFile, loader, logger); err != nil {
			return mon, err
		}
	}
	// Give preference to the cli provided SocketFile
	if c.SocketFile != "" {
		mon.SocketFile = c.SocketFile
	}

	if mon.SocketFile != "" {
		utils.EnsurePermissions(mon.SocketFile, 0660)
	}
	return mon, nil
}

// LastCheck return the time in which the last monitor check was performed
func (m *Monitor) LastCheck() time.Time {
	return m.lastCheck.Get()
}

// Uptime returns for how long the monitor have been running
func (m *Monitor) Uptime() time.Duration {
	// Not yet initialized
	if m.StartTime.Equal(time.Time{}) {
		return time.Duration(0)
	}
	return time.Now().Sub(m.StartTime)
}

// UpdateDatabase updates the file database with the in-memory state
func (m *Monitor) UpdateDatabase() error {
	whileList := make(map[string]struct{}, 0)
	for _, c := range m.checks {
		e := m.database.GetEntry(c.GetID())
		if e == nil {
			e = m.database.AddEntry(c.GetID())
		}
		defer e.unlock()
		e.lock()
		e.Monitored = c.IsMonitored()
		whileList[c.GetID()] = struct{}{}
	}
	// TODO: Maybe we should just leave old entries alone or
	// just recreate the db from scratch
	for _, id := range m.database.Keys() {
		if _, ok := whileList[id]; !ok {
			m.database.Delete(id)
		}
	}
	return m.database.Serialize()
}

// FindCheck looks for a registered Check by id
func (m *Monitor) FindCheck(id string) interface {
	Checkable
} {

	for _, c := range m.checks {
		if c.GetID() == id {
			return c
		}
	}
	return nil
}

// AddCheck registers a new Check in the Monitor.
// It will return an error if an existing check is already registered with
// the same id
func (m *Monitor) AddCheck(c interface {
	Checkable
}) error {
	if m.FindCheck(c.GetID()) != nil {
		return fmt.Errorf("Error: Service name conflict, %s already defined", c.GetID())
	}
	// Allows PerformOnce to properly call the proper Perform, without redefining
	// it everywhere
	c.Initialize(Opts{
		Logger: m.logger,
	})

	e := m.database.GetEntry(c.GetID())
	if e == nil {
		e = m.database.AddEntry(c.GetID())
		defer e.unlock()
		e.lock()
		e.Monitored = c.IsMonitored()
	} else {
		defer e.rUnlock()
		e.rLock()
		c.SetMonitored(e.Monitored)
	}
	m.checks = append(m.checks, c)
	return nil
}

// Reload makes the running app re-parse the configuration file, updating the
// set of monitored checks
func (m *Monitor) Reload() error {
	m.logger.Printf("Reloading")
	validator := newValidator()
	validator.Logger = m.logger
	new(configParser).ParseConfigFile(m.ControlFile, validator, m.logger)
	if validator.Success == true {
		m.logger.Printf("Configuration validates, loading it....")
		m.checks = nil
		for _, c := range validator.Checks {
			if err := m.AddCheck(c); err != nil {
				m.logger.Warnf(err.Error())
			}
		}
	} else {
		m.logger.Warnf("Refusing to reload incorrect configuration")
		return fmt.Errorf("Refusing to reload incorrect configuration")
	}
	return nil
}

// RuntimeDebugStats returns a summary text with currently
// running Go Routines and memory consume
func (m *Monitor) RuntimeDebugStats() string {
	stats := runtime.MemStats{}
	runtime.ReadMemStats(&stats)
	str := fmt.Sprintf("RUNTIME DEBUG:\n")
	for title, value := range map[string]interface{}{
		"Routines Running": runtime.NumGoroutine(),
		"Memory":           fmt.Sprintf("%dKB", stats.Alloc/1024),
	} {
		str += fmt.Sprintf("%-40s %15v\n", title, value)
	}
	return str
}

// LoopForever allows performing all registerd checks in a loop
func (m *Monitor) LoopForever(finish chan bool) {
	for {
		select {
		case <-finish:
			return
		default:
			go func() {
				if os.Getenv("BITNAMI_DEBUG") != "" {
					m.logger.MDebugf(m.RuntimeDebugStats())
				}
				m.Perform()
				if err := m.UpdateDatabase(); err != nil {
					m.logger.Warnf("Error updating database: %s", err.Error())
				}
			}()
		}
		time.Sleep(m.CheckInterval)
	}
}

// Perform calls the Perform method for all managed checks currently
// monitored
func (m *Monitor) Perform() {
	m.logger.Infof("Performing checks")

	m.lastCheck.Set(time.Now())
	for _, c := range m.checks {
		if c.IsMonitored() {
			// The slow part of CheckOnce is already in a goroutine
			// but we don't need to wait for the rest either
			go CheckOnce(c, Opts{Logger: m.logger})
		}
	}
}

// HTTPServerSupported return wether the HTTP interface can be
// enabled or not
func (m *Monitor) HTTPServerSupported() bool {
	return m.SocketFile != ""
}

// StartServer starts the HTTP intterface
func (m *Monitor) StartServer() error {
	if m.SocketFile != "" {
		m.server = createServer(m)
		err := m.server.Start()
		if err != nil {
			fullError := fmt.Errorf("Error listening to socket %s", err.Error())
			m.logger.Errorf(fullError.Error())
			return fullError
		}
	} else {
		m.logger.Warnf("Don't know how to start the HTTP server (missing socket)")
		return fmt.Errorf("Don't know how to start the HTTP server (missing socket)")
	}
	return nil
}

func (m *Monitor) doMultiProcessOperation(cb func(interface {
	CheckableProcess
}) error) []error {
	res := []error{}
	for _, check := range m.checks {
		if pc, ok := check.(interface {
			CheckableProcess
		}); ok {
			if err := cb(pc); err != nil {
				res = append(res, err)
			}
		}
	}
	return res
}

func (m *Monitor) findProcessCheck(id string) (interface {
	CheckableProcess
}, error) {
	c := m.FindCheck(id)
	if c == nil {
		return nil, fmt.Errorf("Cannot find check with id %s", id)
	}
	pc, ok := c.(interface {
		CheckableProcess
	})
	if !ok {
		return nil, fmt.Errorf("Check %s is not a process", id)
	}
	return pc, nil
}

func (m *Monitor) findAndExecProcessCheck(id string, cb func(interface {
	CheckableProcess
}) error) error {
	pc, err := m.findProcessCheck(id)
	if err != nil {
		return err
	}
	return cb(pc)
}

func (m *Monitor) monitorCheck(c interface {
	Checkable
}) error {
	c.SetMonitored(true)
	return m.UpdateDatabase()
}
func (m *Monitor) unmonitorCheck(c interface {
	Checkable
}) error {
	c.SetMonitored(false)
	return m.UpdateDatabase()
}

// Monitor looks for the Check with the provide id and set its monitored status to true
func (m *Monitor) Monitor(id string) error {
	c := m.FindCheck(id)
	if c == nil {
		return fmt.Errorf("Cannot find check with id %s", id)
	}
	return m.monitorCheck(c)
}

// Unmonitor looks for the Check with the provide id and set its monitored status to false
func (m *Monitor) Unmonitor(id string) error {
	c := m.FindCheck(id)
	if c == nil {
		return fmt.Errorf("Cannot find check with id %s", id)
	}
	return m.unmonitorCheck(c)
}

// Start allows starting a process check by ID
func (m *Monitor) Start(id string) error {
	return m.findAndExecProcessCheck(id, startProcess)
}

// Stop allows stopping a process check by ID
func (m *Monitor) Stop(id string) error {
	return m.findAndExecProcessCheck(id, stopProcess)
}

// Restart allows restarting a process check by ID
func (m *Monitor) Restart(id string) error {
	return m.findAndExecProcessCheck(id, restartProcess)
}

// MonitorAll set all checks monitored status to true
func (m *Monitor) MonitorAll() (errors []error) {
	for _, c := range m.checks {
		if err := m.monitorCheck(c); err != nil {
			errors = append(errors, err)
		}
	}
	return errors
}

// UnmonitorAll set all checks monitored status to false
func (m *Monitor) UnmonitorAll() (errors []error) {
	for _, c := range m.checks {
		if err := m.unmonitorCheck(c); err != nil {
			errors = append(errors, err)
		}
	}
	return errors
}

// StartAll allows starting all process checks
func (m *Monitor) StartAll() []error {
	return m.doMultiProcessOperation(startProcess)
}

// StopAll allows stopping all process checks
func (m *Monitor) StopAll() []error {
	return m.doMultiProcessOperation(stopProcess)
}

// RestartAll allows restarting all process checks
func (m *Monitor) RestartAll() []error {
	return m.doMultiProcessOperation(restartProcess)
}

// SummaryText returns a string containing a short status summary for every
// check registered
func (m *Monitor) SummaryText() string {
	s := ""
	s += fmt.Sprintf("Uptime %v\n\n", utils.RoundDuration(m.Uptime()))
	for _, c := range m.checks {
		s += fmt.Sprintln(c.SummaryText())
	}
	return s
}

// StatusText returns a string containing a long description of all checks
// and Monitor attributes
func (m *Monitor) StatusText() string {
	s := ""
	lc := m.LastCheck()
	s += fmt.Sprintf(`
%-30s %v
%-30s %s
%-30s %s
%-30s %d
%-30s %s
%-30s %s
%-30s %s
%-30s %s
`,
		"Uptime", utils.RoundDuration(m.Uptime()),
		"Last Check", lc,
		"Next Check", lc.Add(m.CheckInterval),
		"Pid", m.Pid,
		"Pid File", m.PidFile,
		"Control File", m.ControlFile,
		"Socket File", m.SocketFile,
		"Log File", m.LogFile,
	)

	for _, c := range m.checks {
		s += fmt.Sprintln(c)
	}
	return s
}

// Terminate allows cleaning quitting a monitor (ie. stopping the HTTP server)
func (m *Monitor) Terminate() (err error) {
	m.logger.Info("Terminating application...")
	err = m.cleanup()
	if err != nil {
		m.logger.Warn(err.Error())
	}
	m.logger.Info("Terminated.")
	return err
}

func (m *Monitor) cleanup() error {
	if m.server != nil {
		if err := m.server.Stop(); err != nil {
			return fmt.Errorf("Error tearing down HTTP server: %s", err.Error())
		}
		m.server = nil
	}
	return nil
}
