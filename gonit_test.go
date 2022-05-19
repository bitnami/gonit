package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"syscall"
	"testing"
	"time"

	gt "github.com/bitnami/gonit/gonittest"
	"github.com/bitnami/gonit/monitor"
	tu "github.com/bitnami/gonit/testutils"
	"github.com/bitnami/gonit/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

var (
	cannotFindDaemonStr = `^Cannot find any running daemon to contact. If it is running, make sure you are pointing to the right pid file \(.*/var/run/gonit.pid\)\n`
)

var (
	bitnamiRoot = "/opt/bitnami"
	bitnamiConf = "/opt/bitnami/conf"
	bitnamiTmp  = "/opt/bitnami/tmp"
)

func formatGonitFlags(pidFile, logFile, socketFile, ctrlFile, stateFile string) []string {
	return []string{
		"--controlfile", ctrlFile, "--pidfile", pidFile,
		"--statefile", stateFile, "--socketfile", socketFile,
		"--logfile", logFile,
	}
}

func tearDown(pid int) bool {
	if pid != -1 && utils.IsProcessRunning(pid) {
		syscall.Kill(pid, syscall.SIGTERM)
		time.Sleep(100 * time.Millisecond)
		syscall.Kill(pid, syscall.SIGKILL)
		time.Sleep(100 * time.Millisecond)
		return true
	}
	return false
}

func RenderScenario(sb *tu.Sandbox, id string, dest string, opts gt.CfgOpts) error {
	return gt.RenderConfiguration(filepath.Join("testdata/scenarios", id, "/*"), dest, opts)
}
func prepareRootDir(rootDir string) (pidFile, logFile, socketFile, ctrlFile, stateFile string) {
	for _, p := range []string{"/etc/gonit", "/var/log", "/var/run"} {
		os.MkdirAll(filepath.Join(rootDir, p), os.FileMode(0755))
	}

	pidFile = filepath.Join(rootDir, "/var/run/gonit.pid")
	logFile = filepath.Join(rootDir, "/var/log/gonit.log")
	socketFile = filepath.Join(rootDir, "/var/run/gonit.sock")
	ctrlFile = filepath.Join(rootDir, "conf/gonit/bitnami.conf")
	stateFile = filepath.Join(rootDir, "/var/lib/gonit/state")

	os.MkdirAll(filepath.Dir(ctrlFile), os.FileMode(0755))
	return pidFile, logFile, socketFile, ctrlFile, stateFile
}

type GonitDaemon struct {
	stateFile  string
	pidFile    string
	logFile    string
	socketFile string
	ctrlFile   string
}

func (g *GonitDaemon) Pid() int {
	pid, _ := utils.ReadPid(g.pidFile)
	return pid
}
func (g *GonitDaemon) RequireRunning(t *testing.T) {
	require.True(t,
		IsProcessRunning(g.pidFile), "Gonit Daemon should be running")
}

func (g *GonitDaemon) RequireStopped(t *testing.T) {
	require.False(t,
		IsProcessRunning(g.pidFile), "Gonit Daemon should be stopped")
}

func NewGonitDaemon(pidFile, logFile, socketFile, ctrlFile, stateFile string) *GonitDaemon {
	return &GonitDaemon{
		pidFile: pidFile, logFile: logFile, socketFile: socketFile,
		ctrlFile: ctrlFile, stateFile: stateFile,
	}
}
func (g *GonitDaemon) Start() CmdResult {
	flags := formatGonitFlags(g.pidFile, g.logFile, g.socketFile, g.ctrlFile, g.stateFile)
	return gonit(flags)
}

func (g *GonitDaemon) TearDown() {
	KillProcess(g.pidFile)
}

type ConfigSuite struct {
	suite.Suite
	sb *tu.Sandbox
}

func (suite *ConfigSuite) AssertPanicsMatch(fn func(), re *regexp.Regexp) bool {
	return tu.AssertPanicsMatch(suite.T(), fn, re)
}

func (suite *ConfigSuite) SetupSuite() {
	suite.sb = tu.NewSandbox()
}

func (suite *ConfigSuite) TearDownSuite() {
	suite.sb.Cleanup()
}

func (suite *ConfigSuite) TestValidations() {
	t := suite.T()

	pidFile, logFile, socketFile, ctrlFile, stateFile := prepareRootDir(suite.sb.Root)

	daemon := NewGonitDaemon(pidFile, logFile, socketFile, ctrlFile, stateFile)

	daemon.Start().AssertErrorMatch(t, fmt.Sprintf("Control file '%s' does not exists", ctrlFile))

	id := "sample"

	script := filepath.Join(suite.sb.Root, fmt.Sprintf("loop-%s.sh", id))

	cfg := gt.CfgOpts{
		Name:         id,
		Timeout:      "2",
		TimeoutUnits: "seconds",
		RootDir:      suite.sb.Normalize(bitnamiRoot),
		ConfDir:      suite.sb.Normalize(bitnamiConf),
		StartCmd:     script,
		StopCmd:      "touch {{.RootDir}}/{{.Name}}.stop",
		TempDir:      suite.sb.Normalize(bitnamiTmp),
	}

	suite.NoError(gt.RenderTemplate("sample-ctl-script", script, cfg))
	os.Chmod(script, os.FileMode(0755))

	suite.NoError(gt.RenderTemplate("service-check", ctrlFile, cfg))

	// Make sure it has wrong permissions
	os.Chmod(ctrlFile, os.FileMode(0755))

	daemon.Start().AssertErrorMatch(t,
		fmt.Sprintf(
			"file '%s' must have permissions no more than -rwx------; right now permissions are -rwxr-xr-x",
			ctrlFile))

	// Now too restrictive
	os.Chmod(ctrlFile, os.FileMode(0000))
	daemon.Start().AssertErrorMatch(t,
		fmt.Sprintf(
			"Configuration file '%s' is not readable",
			ctrlFile))

	os.Chmod(ctrlFile, os.FileMode(0700))

	daemon.Start().AssertSuccess(t)
	defer daemon.TearDown()
}

func TestConfigSuite(t *testing.T) {
	suite.Run(t, new(ConfigSuite))
}

type CmdSuite struct {
	suite.Suite
	sb       *tu.Sandbox
	pidFiles []string
}

func (suite *CmdSuite) TearDownSuite() {
	for _, f := range suite.pidFiles {
		KillProcess(f)
	}
	suite.sb.Cleanup()
}
func (suite *CmdSuite) TrackPidFiles(pidFiles ...string) {
	suite.pidFiles = append(suite.pidFiles, pidFiles...)
}
func (suite *CmdSuite) AssertPanicsMatch(fn func(), re *regexp.Regexp) bool {
	return tu.AssertPanicsMatch(suite.T(), fn, re)
}

func (suite *CmdSuite) SetupSuite() {
	suite.sb = tu.NewSandbox()
}

func IsProcessMonitored(stateFile string, id string) bool {
	db, _ := monitor.NewDatabase(stateFile)
	c := db.GetEntry(id)
	if c == nil {
		return false
	}
	return c.Monitored
}

func IsProcessRunning(pidFile string) bool {
	pid, err := utils.ReadPid(pidFile)
	if err != nil {
		return false
	}
	return utils.IsProcessRunning(pid)
}

func KillProcess(pidFile string) bool {
	if !IsProcessRunning(pidFile) {
		return true
	}
	pid, err := utils.ReadPid(pidFile)
	if err != nil {
		return false
	}
	return tearDown(pid)
}

func (suite *CmdSuite) TestMonitorAndUnmonitorCommand() {
	require := suite.Require()
	rootDir := suite.sb.TempFile()
	suite.RenderScenario("scenario1", rootDir, gt.CfgOpts{
		Name:    "scenario1",
		RootDir: rootDir,
	})

	pidFile, logFile, socketFile, ctrlFile, stateFile := prepareRootDir(rootDir)

	flags := formatGonitFlags(pidFile, logFile, socketFile, ctrlFile, stateFile)

	require.False(IsProcessRunning(pidFile), "Expected process to not be running")

	apachePidFile := filepath.Join(rootDir, "apache2/tmp/apache2.pid")
	mysqlPidFile := filepath.Join(rootDir, "mysql/tmp/mysql.pid")
	suite.TrackPidFiles(apachePidFile, mysqlPidFile)

	require.False(IsProcessRunning(apachePidFile))
	require.False(IsProcessRunning(mysqlPidFile))

	// There is no database, so the offline mode returns false
	require.False(IsProcessMonitored(stateFile, "apache"))
	require.False(IsProcessMonitored(stateFile, "mysql"))
	tu.AssertFileDoesNotExist(suite.T(), stateFile)

	gonit(flags, "monitor").AssertSuccess(suite.T())
	tu.AssertFileExists(suite.T(), stateFile)
	require.True(IsProcessMonitored(stateFile, "apache"))
	require.True(IsProcessMonitored(stateFile, "mysql"))

	gonit(flags, "unmonitor").AssertSuccess(suite.T())
	tu.AssertFileExists(suite.T(), stateFile)
	require.False(IsProcessMonitored(stateFile, "apache"))
	require.False(IsProcessMonitored(stateFile, "mysql"))

	// Monitor individual processes
	gonit(flags, "monitor", "foobar").AssertErrorMatch(suite.T(), "Failed to monitor foobar: Cannot find check with id foobar")
	gonit(flags, "monitor", "hello", "world").AssertErrorMatch(suite.T(), "Command monitor requires at most 1 arguments but 2 were provided")

	gonit(flags, "monitor", "apache").AssertSuccessMatch(suite.T(), "Monitored apache")
	require.True(IsProcessMonitored(stateFile, "apache"))

	// Make sure mysql was not also monitored
	require.False(IsProcessMonitored(stateFile, "mysql"))
	gonit(flags, "monitor", "mysql").AssertSuccessMatch(suite.T(), "Monitored mysql")
	require.True(IsProcessMonitored(stateFile, "mysql"))

	// Unmonitor individual processes
	gonit(flags, "unmonitor", "foobar").AssertErrorMatch(suite.T(), "Failed to unmonitor foobar: Cannot find check with id foobar")
	gonit(flags, "unmonitor", "hello", "world").AssertErrorMatch(suite.T(), "Command unmonitor requires at most 1 arguments but 2 were provided")

	gonit(flags, "unmonitor", "apache").AssertSuccessMatch(suite.T(), "Unmonitored apache")
	require.False(IsProcessMonitored(stateFile, "apache"))

	// Make sure mysql was not also unmonitored
	require.True(IsProcessMonitored(stateFile, "mysql"))
	gonit(flags, "unmonitor", "mysql").AssertSuccessMatch(suite.T(), "Unmonitored mysql")
	require.False(IsProcessMonitored(stateFile, "mysql"))
}

func (suite *CmdSuite) TestRestartCommandNoDaemon() {
	require := suite.Require()
	rootDir := suite.sb.TempFile()
	suite.RenderScenario("scenario1", rootDir, gt.CfgOpts{
		Name:    "scenario1",
		RootDir: rootDir,
	})

	pidFile, logFile, socketFile, ctrlFile, stateFile := prepareRootDir(rootDir)
	flags := formatGonitFlags(pidFile, logFile, socketFile, ctrlFile, stateFile)

	require.False(IsProcessRunning(pidFile), "Expected process to not be running")

	apachePidFile := filepath.Join(rootDir, "apache2/tmp/apache2.pid")
	mysqlPidFile := filepath.Join(rootDir, "mysql/tmp/mysql.pid")
	suite.TrackPidFiles(apachePidFile, mysqlPidFile)

	require.False(IsProcessRunning(apachePidFile))
	require.False(IsProcessRunning(mysqlPidFile))

	// Restart stopped services
	gonit(flags, "restart").AssertSuccess(suite.T())
	require.True(IsProcessRunning(apachePidFile))
	require.True(IsProcessRunning(mysqlPidFile))

	apachePid, _ := utils.ReadPid(apachePidFile)
	require.NotEqual(apachePid, -1)
	mysqlPid, _ := utils.ReadPid(mysqlPidFile)
	require.NotEqual(mysqlPid, -1)

	// Restart running services
	gonit(flags, "restart").AssertSuccess(suite.T())
	require.True(IsProcessRunning(apachePidFile))
	require.True(IsProcessRunning(mysqlPidFile))
	// The PIDs must be different now
	newApachePid, _ := utils.ReadPid(apachePidFile)
	require.NotEqual(newApachePid, -1)
	require.NotEqual(newApachePid, apachePid)
	newMysqlPid, _ := utils.ReadPid(mysqlPidFile)
	require.NotEqual(newMysqlPid, -1)
	require.NotEqual(newMysqlPid, mysqlPid)

	// Restart individual processes
	gonit(flags, "restart", "foobar").AssertErrorMatch(suite.T(), "Failed to restart foobar: Cannot find check with id foobar")
	gonit(flags, "restart", "hello", "world").AssertErrorMatch(suite.T(), "Command restart requires at most 1 arguments but 2 were provided")

	gonit(flags, "restart", "apache").AssertSuccessMatch(suite.T(), "Restarted apache")
	require.True(IsProcessRunning(apachePidFile))
	newApachePid2, _ := utils.ReadPid(apachePidFile)
	require.NotEqual(newApachePid2, -1)
	require.NotEqual(newApachePid, newApachePid2)

	// Make sure mysql was not also restarted
	require.True(IsProcessRunning(mysqlPidFile))
	newMysqlPid2, _ := utils.ReadPid(mysqlPidFile)
	require.Equal(newMysqlPid, newMysqlPid2)

	gonit(flags, "restart", "mysql").AssertSuccessMatch(suite.T(), `^Restarted mysql\n$`)
	require.True(IsProcessRunning(mysqlPidFile))

}

func (suite *CmdSuite) TestRestartCommand() {
	t := suite.T()
	require := suite.Require()
	rootDir := suite.sb.TempFile()
	suite.RenderScenario("scenario1", rootDir, gt.CfgOpts{
		Name:    "scenario1",
		RootDir: rootDir,
	})

	pidFile, logFile, socketFile, ctrlFile, stateFile := prepareRootDir(rootDir)
	flags := formatGonitFlags(pidFile, logFile, socketFile, ctrlFile, stateFile)

	daemon := NewGonitDaemon(pidFile, logFile, socketFile, ctrlFile, stateFile)
	daemon.RequireStopped(t)

	apachePidFile := filepath.Join(rootDir, "apache2/tmp/apache2.pid")
	mysqlPidFile := filepath.Join(rootDir, "mysql/tmp/mysql.pid")
	suite.TrackPidFiles(apachePidFile, mysqlPidFile)

	require.False(IsProcessRunning(apachePidFile))
	require.False(IsProcessRunning(mysqlPidFile))

	// Start daemon

	daemon.Start().AssertSuccess(t)
	defer daemon.TearDown()

	time.Sleep(1500 * time.Millisecond)
	daemon.RequireRunning(t)

	// The first time, all configured services are started with the daemon
	require.True(IsProcessRunning(apachePidFile))
	require.True(IsProcessRunning(mysqlPidFile))
	apachePid, _ := utils.ReadPid(apachePidFile)
	require.NotEqual(apachePid, -1)
	mysqlPid, _ := utils.ReadPid(mysqlPidFile)
	require.NotEqual(mysqlPid, -1)

	// Restart running services
	gonit(flags, "restart").AssertSuccess(t)
	time.Sleep(2000 * time.Millisecond)
	require.True(IsProcessRunning(apachePidFile))
	require.True(IsProcessRunning(mysqlPidFile))
	// The PIDs must be different now
	newApachePid, _ := utils.ReadPid(apachePidFile)
	require.NotEqual(newApachePid, -1)
	require.NotEqual(newApachePid, apachePid)
	newMysqlPid, _ := utils.ReadPid(mysqlPidFile)
	require.NotEqual(newMysqlPid, -1)
	require.NotEqual(newMysqlPid, mysqlPid)

	gonit(flags, "stop").AssertSuccess(t)
	// The client-server mode is not synchronous so it take some time
	time.Sleep(500 * time.Millisecond)
	require.False(IsProcessRunning(apachePidFile))
	require.False(IsProcessRunning(mysqlPidFile))

	// Restart stopped services
	gonit(flags, "restart").AssertSuccess(t)
	time.Sleep(2000 * time.Millisecond)
	require.True(IsProcessRunning(apachePidFile))
	require.True(IsProcessRunning(mysqlPidFile))

	newApachePid, _ = utils.ReadPid(apachePidFile)
	newMysqlPid, _ = utils.ReadPid(mysqlPidFile)

	// Restart individual processes
	gonit(flags, "restart", "foobar").AssertErrorMatch(t, "Failed to restart foobar: Cannot find check with id foobar")
	gonit(flags, "restart", "hello", "world").AssertErrorMatch(t, "Command restart requires at most 1 arguments but 2 were provided")

	gonit(flags, "restart", "apache").AssertSuccessMatch(t, "Restarted apache")
	time.Sleep(2000 * time.Millisecond)
	require.True(IsProcessRunning(apachePidFile))
	newApachePid2, _ := utils.ReadPid(apachePidFile)
	require.NotEqual(newApachePid2, -1)
	require.NotEqual(newApachePid, newApachePid2)

	// Make sure mysql was not also restarted
	require.True(IsProcessRunning(mysqlPidFile))
	newMysqlPid2, _ := utils.ReadPid(mysqlPidFile)
	require.Equal(newMysqlPid, newMysqlPid2)

	gonit(flags, "restart", "mysql").AssertSuccessMatch(t, `^Restarted mysql\n$`)
	time.Sleep(2000 * time.Millisecond)
	require.True(IsProcessRunning(mysqlPidFile))
}

func (suite *CmdSuite) TestQuitCommand() {
	t := suite.T()
	require := suite.Require()
	rootDir := suite.sb.TempFile()
	suite.RenderScenario("scenario1", rootDir, gt.CfgOpts{
		Name:    "scenario1",
		RootDir: rootDir,
	})

	pidFile, logFile, socketFile, ctrlFile, stateFile := prepareRootDir(rootDir)
	flags := formatGonitFlags(pidFile, logFile, socketFile, ctrlFile, stateFile)

	daemon := NewGonitDaemon(pidFile, logFile, socketFile, ctrlFile, stateFile)
	daemon.RequireStopped(t)

	gonit(flags, "quit").AssertErrorMatch(suite.T(), `^Cannot find any running daemon to stop. If it is running, make sure you are pointing to the right pid file \(.*/var/run/gonit.pid\)\n$`)

	// Start daemon
	daemon.Start().AssertSuccess(suite.T())
	defer daemon.TearDown()

	time.Sleep(1500 * time.Millisecond)
	daemon.RequireRunning(t)

	apachePidFile := filepath.Join(rootDir, "apache2/tmp/apache2.pid")
	mysqlPidFile := filepath.Join(rootDir, "mysql/tmp/mysql.pid")
	suite.TrackPidFiles(apachePidFile, mysqlPidFile)

	require.True(IsProcessRunning(apachePidFile))
	require.True(IsProcessRunning(mysqlPidFile))

	gonit(flags, "quit").AssertSuccess(t)
	time.Sleep(1000 * time.Millisecond)
	daemon.RequireStopped(t)

	// Check should survive killing gonit
	require.True(IsProcessRunning(apachePidFile))
	require.True(IsProcessRunning(mysqlPidFile))
}

func (suite *CmdSuite) TestReloadCommand() {
	t := suite.T()
	rootDir := suite.sb.TempFile()
	suite.RenderScenario("scenario1", rootDir, gt.CfgOpts{
		Name:    "scenario1",
		RootDir: rootDir,
	})

	pidFile, logFile, socketFile, ctrlFile, stateFile := prepareRootDir(rootDir)
	flags := formatGonitFlags(pidFile, logFile, socketFile, ctrlFile, stateFile)

	daemon := NewGonitDaemon(pidFile, logFile, socketFile, ctrlFile, stateFile)

	// Make sure the daemon is not running
	daemon.RequireStopped(t)

	gonit(flags, "reload").AssertErrorMatch(suite.T(), cannotFindDaemonStr)

	// Start daemon
	daemon.Start().AssertSuccess(suite.T())
	defer daemon.TearDown()

	time.Sleep(500 * time.Millisecond)
	daemon.RequireRunning(t)
	apachePidFile := filepath.Join(rootDir, "apache2/tmp/apache2.pid")
	mysqlPidFile := filepath.Join(rootDir, "mysql/tmp/mysql.pid")
	suite.TrackPidFiles(apachePidFile, mysqlPidFile)

	gonit(flags, "reload").AssertSuccess(t)
	time.Sleep(500 * time.Millisecond)
	gonit(flags, "summary").AssertSuccessMatch(t, "(?s).*Process apache.*Process mysql.*")
	mysqlConf := filepath.Join(rootDir, "conf/gonit/conf.d/mysql.conf")
	os.RemoveAll(mysqlConf)

	gonit(flags, "reload").AssertSuccess(t)
	time.Sleep(500 * time.Millisecond)
	r := gonit(flags, "summary")
	r.AssertSuccess(suite.T())
	suite.Regexp("(?s).*Process apache.*", r.stdout)
	suite.NotRegexp("(?s).*Process mysql.*", r.stdout)

	suite.NotRegexp("(?s).*Process sample_check.*", r.stdout)
	ioutil.WriteFile(
		filepath.Join(rootDir, "conf/gonit/conf.d/mysql.conf"),
		[]byte("check process sample_check"), os.FileMode(0644))

	gonit(flags, "reload").AssertSuccess(t)
	time.Sleep(500 * time.Millisecond)
	r = gonit(flags, "summary")
	r.AssertSuccess(suite.T())
	suite.Regexp("(?s).*Process sample_check.*", r.stdout)

}
func (suite *CmdSuite) TestStartCommandNoDaemon() {
	t := suite.T()
	require := suite.Require()
	rootDir := suite.sb.TempFile()
	suite.RenderScenario("scenario1", rootDir, gt.CfgOpts{
		Name:    "scenario1",
		RootDir: rootDir,
	})

	pidFile, logFile, socketFile, ctrlFile, stateFile := prepareRootDir(rootDir)
	flags := formatGonitFlags(pidFile, logFile, socketFile, ctrlFile, stateFile)

	// Make sure the daemon is not running
	require.False(IsProcessRunning(pidFile), "Expected process to not be running")

	apachePidFile := filepath.Join(rootDir, "apache2/tmp/apache2.pid")
	mysqlPidFile := filepath.Join(rootDir, "mysql/tmp/mysql.pid")
	suite.TrackPidFiles(apachePidFile, mysqlPidFile)

	require.False(IsProcessRunning(apachePidFile))
	require.False(IsProcessRunning(mysqlPidFile))

	gonit(flags, "start").AssertSuccess(t)
	require.True(IsProcessRunning(apachePidFile))
	require.True(IsProcessRunning(mysqlPidFile))

	// Does not error in case of services already running
	gonit(flags, "start").AssertSuccess(t)
	require.True(IsProcessRunning(apachePidFile))
	require.True(IsProcessRunning(mysqlPidFile))

	KillProcess(apachePidFile)
	KillProcess(mysqlPidFile)
	require.False(IsProcessRunning(apachePidFile))
	require.False(IsProcessRunning(mysqlPidFile))

	// Start individual processes
	gonit(flags, "start", "foobar").AssertErrorMatch(t,
		"Failed to start foobar: Cannot find check with id foobar")
	gonit(flags, "start", "hello", "world").AssertErrorMatch(t,
		"Command start requires at most 1 arguments but 2 were provided")

	gonit(flags, "start", "apache").AssertSuccessMatch(t, "Started apache")
	require.True(IsProcessRunning(apachePidFile))

	// Make sure mysql was not also started
	require.False(IsProcessRunning(mysqlPidFile))

	gonit(flags, "start", "mysql").AssertSuccessMatch(t, "Started mysql")
	require.True(IsProcessRunning(mysqlPidFile))
}

func (suite *CmdSuite) TestStartCommand() {
	t := suite.T()
	require := suite.Require()
	rootDir := suite.sb.TempFile()
	suite.RenderScenario("scenario1", rootDir, gt.CfgOpts{
		Name:    "scenario1",
		RootDir: rootDir,
	})

	pidFile, logFile, socketFile, ctrlFile, stateFile := prepareRootDir(rootDir)
	flags := formatGonitFlags(pidFile, logFile, socketFile, ctrlFile, stateFile)

	daemon := NewGonitDaemon(pidFile, logFile, socketFile, ctrlFile, stateFile)

	// Make sure the daemon is not running
	daemon.RequireStopped(t)

	apachePidFile := filepath.Join(rootDir, "apache2/tmp/apache2.pid")
	mysqlPidFile := filepath.Join(rootDir, "mysql/tmp/mysql.pid")
	suite.TrackPidFiles(apachePidFile, mysqlPidFile)

	require.False(IsProcessRunning(apachePidFile))
	require.False(IsProcessRunning(mysqlPidFile))

	// Start daemon
	daemon.Start().AssertSuccess(t)
	defer daemon.TearDown()
	time.Sleep(1500 * time.Millisecond)
	daemon.RequireRunning(t)

	// The first time, all configured services are started with the daemon
	require.True(IsProcessRunning(apachePidFile))
	require.True(IsProcessRunning(mysqlPidFile))

	gonit(flags, "stop").AssertSuccess(t)
	// The client-server mode is not synchronous so it take some time
	time.Sleep(1500 * time.Millisecond)
	require.False(IsProcessRunning(apachePidFile))
	require.False(IsProcessRunning(mysqlPidFile))

	// Asking different things before giving time to finish produces a warning
	gonit(flags, "start").AssertSuccess(t)
	gonit(flags, "stop").AssertErrorMatch(t,
		`(?s)\[apache\] Other action already in progress -- please try again later.*\[mysql\] Other action already in progress -- please try again later`)
	time.Sleep(1500 * time.Millisecond)
	require.True(IsProcessRunning(apachePidFile))
	require.True(IsProcessRunning(mysqlPidFile))

	gonit(flags, "stop").AssertSuccess(t)

	time.Sleep(1500 * time.Millisecond)
	require.False(IsProcessRunning(apachePidFile))
	require.False(IsProcessRunning(mysqlPidFile))

	// Start individual processes
	gonit(flags, "start", "foobar").AssertErrorMatch(t, "Failed to start foobar: Cannot find check with id foobar")
	gonit(flags, "start", "hello", "world").AssertErrorMatch(t, "Command start requires at most 1 arguments but 2 were provided")

	gonit(flags, "start", "apache").AssertSuccessMatch(t, "Started apache")
	time.Sleep(1500 * time.Millisecond)
	require.True(IsProcessRunning(apachePidFile))

	// Make sure mysql was not also started
	require.False(IsProcessRunning(mysqlPidFile))

	gonit(flags, "start", "mysql").AssertSuccessMatch(t, "Started mysql")
	time.Sleep(1500 * time.Millisecond)
	require.True(IsProcessRunning(mysqlPidFile))
}
func (suite *CmdSuite) TestVersionCommand() {
	gonit([]string{}, "version").AssertSuccessMatch(suite.T(), fmt.Sprintf("^Gonit %s", version))
}

func (suite *CmdSuite) TestStopCommandNoDaemon() {
	t := suite.T()
	require := suite.Require()
	rootDir := suite.sb.TempFile()
	suite.RenderScenario("scenario1", rootDir, gt.CfgOpts{
		Name:    "scenario1",
		RootDir: rootDir,
	})

	pidFile, logFile, socketFile, ctrlFile, stateFile := prepareRootDir(rootDir)
	flags := formatGonitFlags(pidFile, logFile, socketFile, ctrlFile, stateFile)

	// Make sure the daemon is not running
	require.False(IsProcessRunning(pidFile), "Expected process to not be running")

	apachePidFile := filepath.Join(rootDir, "apache2/tmp/apache2.pid")
	mysqlPidFile := filepath.Join(rootDir, "mysql/tmp/mysql.pid")
	suite.TrackPidFiles(apachePidFile, mysqlPidFile)

	require.False(IsProcessRunning(apachePidFile))
	require.False(IsProcessRunning(mysqlPidFile))

	gonit(flags, "start").AssertSuccess(t)
	require.True(IsProcessRunning(apachePidFile))
	require.True(IsProcessRunning(mysqlPidFile))

	gonit(flags, "stop").AssertSuccess(t)
	require.False(IsProcessRunning(apachePidFile))
	require.False(IsProcessRunning(mysqlPidFile))

	// Does not error in case of services already stopped
	gonit(flags, "stop").AssertSuccess(t)
	require.False(IsProcessRunning(apachePidFile))
	require.False(IsProcessRunning(mysqlPidFile))

	gonit(flags, "start").AssertSuccess(t)
	require.True(IsProcessRunning(apachePidFile))
	require.True(IsProcessRunning(mysqlPidFile))

	// Stop individual processes
	gonit(flags, "stop", "foobar").AssertErrorMatch(t, "Failed to stop foobar: Cannot find check with id foobar")
	gonit(flags, "stop", "hello", "world").AssertErrorMatch(t, "Command stop requires at most 1 arguments but 2 were provided")

	gonit(flags, "stop", "apache").AssertSuccessMatch(t, "Stopped apache")
	require.False(IsProcessRunning(apachePidFile))

	// Make sure mysql was not also stopped
	require.True(IsProcessRunning(mysqlPidFile))

	gonit(flags, "stop", "mysql").AssertSuccessMatch(t, "Stopped mysql")
	require.False(IsProcessRunning(mysqlPidFile))
}
func (suite *CmdSuite) RenderScenario(id string, dest string, opts gt.CfgOpts) (pidFile, logFile, socketFile, ctrlFile, stateFile string) {
	suite.NoError(RenderScenario(suite.sb, id, dest, opts))
	os.Chmod(filepath.Join(dest, "conf/gonit/bitnami.conf"), os.FileMode(0700))
	return prepareRootDir(dest)
}

func (suite *CmdSuite) TestStopCommand() {
	t := suite.T()
	require := suite.Require()
	rootDir := suite.sb.TempFile()
	suite.RenderScenario("scenario1", rootDir, gt.CfgOpts{
		Name:    "scenario1",
		RootDir: rootDir,
	})

	pidFile, logFile, socketFile, ctrlFile, stateFile := prepareRootDir(rootDir)
	flags := formatGonitFlags(pidFile, logFile, socketFile, ctrlFile, stateFile)

	daemon := NewGonitDaemon(pidFile, logFile, socketFile, ctrlFile, stateFile)

	// Make sure the daemon is not running
	daemon.RequireStopped(t)

	apachePidFile := filepath.Join(rootDir, "apache2/tmp/apache2.pid")
	mysqlPidFile := filepath.Join(rootDir, "mysql/tmp/mysql.pid")
	suite.TrackPidFiles(apachePidFile, mysqlPidFile)

	require.False(IsProcessRunning(apachePidFile))
	require.False(IsProcessRunning(mysqlPidFile))

	// Start daemon
	daemon.Start().AssertSuccess(suite.T())
	defer daemon.TearDown()

	time.Sleep(1500 * time.Millisecond)
	daemon.RequireRunning(t)

	// The first time, all configured services are started with the daemon
	require.True(IsProcessRunning(apachePidFile))
	require.True(IsProcessRunning(mysqlPidFile))

	// Asking different things before giving time to finish produces a warning
	gonit(flags, "stop").AssertSuccess(suite.T())
	gonit(flags, "start").AssertErrorMatch(suite.T(),
		`(?s)\[apache\] Other action already in progress -- please try again later.*\[mysql\] Other action already in progress -- please try again later`)
	// The client-server mode is not synchronous so it take some time
	time.Sleep(2000 * time.Millisecond)
	require.False(IsProcessRunning(apachePidFile))
	require.False(IsProcessRunning(mysqlPidFile))

	gonit(flags, "start").AssertSuccess(suite.T())
	time.Sleep(500 * time.Millisecond)
	require.True(IsProcessRunning(apachePidFile))
	require.True(IsProcessRunning(mysqlPidFile))

	// Stop individual processes
	gonit(flags, "stop", "foobar").AssertErrorMatch(suite.T(), "Failed to stop foobar: Cannot find check with id foobar")
	gonit(flags, "stop", "hello", "world").AssertErrorMatch(suite.T(), "Command stop requires at most 1 arguments but 2 were provided")

	gonit(flags, "stop", "apache").AssertSuccessMatch(suite.T(), "Stopped apache")
	require.True(IsProcessRunning(apachePidFile))
	time.Sleep(500 * time.Millisecond)
	require.False(IsProcessRunning(apachePidFile))

	// Make sure mysql was not also stopped
	require.True(IsProcessRunning(mysqlPidFile))

	gonit(flags, "stop", "mysql").AssertSuccessMatch(suite.T(), "Stopped mysql")
	time.Sleep(500 * time.Millisecond)
	require.False(IsProcessRunning(mysqlPidFile))
}

func (suite *CmdSuite) TestBasicConfigChecks() {
	rootDir := suite.sb.TempFile()
	suite.RenderScenario("scenario1", rootDir, gt.CfgOpts{
		Name:    "scenario1",
		RootDir: rootDir,
	})

	pidFile, _, _, ctrlFile, stateFile := prepareRootDir(rootDir)

	for _, cmd := range []string{"status", "summary"} {
		flags := []string{}
		gonit(flags, cmd).AssertErrorMatch(suite.T(), "^Control file '/etc/gonit/gonitrc' does not exists\n$")

		flags = []string{"--controlfile", ctrlFile, "--pidfile", pidFile, "--statefile", stateFile}
		gonit(flags, cmd).AssertErrorMatch(suite.T(), cannotFindDaemonStr)
	}
}

func (suite *CmdSuite) TestStatusCommand() {
	t := suite.T()
	rootDir := suite.sb.TempFile()
	suite.RenderScenario("scenario1", rootDir, gt.CfgOpts{
		Name:    "scenario1",
		RootDir: rootDir,
	})
	pidFile, logFile, socketFile, ctrlFile, stateFile := prepareRootDir(rootDir)
	flags := formatGonitFlags(pidFile, logFile, socketFile, ctrlFile, stateFile)

	// Start daemon
	daemon := NewGonitDaemon(pidFile, logFile, socketFile, ctrlFile, stateFile)
	daemon.Start().AssertSuccess(t)

	time.Sleep(1500 * time.Millisecond)
	daemon.RequireRunning(t)
	defer daemon.TearDown()

	apachePidFile := filepath.Join(rootDir, "apache2/tmp/apache2.pid")
	mysqlPidFile := filepath.Join(rootDir, "mysql/tmp/mysql.pid")
	suite.TrackPidFiles(apachePidFile, mysqlPidFile)
	statusHeaderPattern := fmt.Sprintf(`Uptime\s+\d+.*.+Pid\s+%d.+Pid File\s+%s.+Control File\s+%s.+Socket File\s+%s.*Log File\s+%s\n\s*`, daemon.Pid(), pidFile, ctrlFile, socketFile, logFile)
	apachePattern := `Process 'apache'.+status\s+Running.+uptime\s+\d+.+monitoring status\s+monitored\n\s*`
	mysqlPattern := `Process 'mysql'.+status\s+Running.+uptime\s+\d+.+monitoring status\s+monitored\n\s*`

	gonit(flags, "status").AssertSuccessMatch(t, `(?s)`+
		statusHeaderPattern+
		apachePattern+
		mysqlPattern+`$`,
	)
	gonit(flags, "status", "apache").AssertSuccessMatch(t, `(?s)`+
		statusHeaderPattern+
		apachePattern+`$`,
	)
	gonit(flags, "status", "mysql").AssertSuccessMatch(t, `(?s)`+
		statusHeaderPattern+
		mysqlPattern+`$`,
	)

	gonit(flags, "status", "apache", "mysql").AssertErrorMatch(suite.T(), "Too many arguments provided. Only an optional service name is allowed")

	daemon.TearDown()
	daemon.RequireStopped(t)

	gonit(flags, "status").AssertErrorMatch(t, cannotFindDaemonStr)
}

func (suite *CmdSuite) TestSummaryCommand() {
	t := suite.T()
	rootDir := suite.sb.TempFile()
	suite.RenderScenario("scenario1", rootDir, gt.CfgOpts{
		Name:    "scenario1",
		RootDir: rootDir,
	})

	pidFile, logFile, socketFile, ctrlFile, stateFile := prepareRootDir(rootDir)
	flags := formatGonitFlags(pidFile, logFile, socketFile, ctrlFile, stateFile)

	// Start daemon
	daemon := NewGonitDaemon(pidFile, logFile, socketFile, ctrlFile, stateFile)
	daemon.Start().AssertSuccess(t)

	time.Sleep(1500 * time.Millisecond)
	daemon.RequireRunning(t)
	defer daemon.TearDown()
	summaryHeaderPattern := `Uptime\s+\d+[^\s]+\n.+`
	apachePattern := `Process apache\s+Running\n\s*`
	mysqlPattern := `Process mysql\s+Running\n\s*`

	gonit(flags, "summary").AssertSuccessMatch(t, `(?s)`+
		summaryHeaderPattern+
		apachePattern+
		mysqlPattern+`$`)

	gonit(flags, "summary", "apache").AssertSuccessMatch(t, `(?s)`+
		summaryHeaderPattern+
		apachePattern+`$`)

	gonit(flags, "summary", "mysql").AssertSuccessMatch(t, `(?s)`+
		summaryHeaderPattern+
		mysqlPattern+`$`)

	gonit(flags, "summary", "apache", "mysql").AssertErrorMatch(suite.T(), "Too many arguments provided. Only an optional service name is allowed")

	// This is not a cleanup operation, we are synchronously requesting
	// the daemon to be stopped
	daemon.TearDown()
	daemon.RequireStopped(t)

	gonit(flags, "summary").AssertErrorMatch(t, cannotFindDaemonStr)
}

func gonit(flags []string, cmdArgs ...string) CmdResult {
	return execCommand(append(flags, cmdArgs...)...)
}

func TestStatusCommand(t *testing.T) {
	suite.Run(t, new(CmdSuite))

}

type CmdResult struct {
	code   int
	stdout string
	stderr string
}

func (r CmdResult) AssertErrorMatch(t *testing.T, re interface{}) bool {
	if r.AssertError(t) {
		return assert.Regexp(t, re, r.stderr)
	}
	return true
}

func (r CmdResult) AssertSuccessMatch(t *testing.T, re interface{}) bool {
	if r.AssertSuccess(t) {
		return assert.Regexp(t, re, r.stdout)
	}
	return true
}
func (r CmdResult) AssertCode(t *testing.T, code int) bool {
	return assert.Equal(t, code, r.code, "Expected %d code but got %d", code, r.code)
}
func (r CmdResult) AssertSuccess(t *testing.T) bool {
	return assert.True(t, r.Success(), "Expected command to success but got code=%d stderr=%s", r.code, r.stderr)
}

func (r CmdResult) AssertError(t *testing.T) bool {
	return assert.False(t, r.Success(), "Expected command to fail")
}

func (r CmdResult) Success() bool {
	return r.code == 0
}

func execCommand(args ...string) CmdResult {
	var buffStdout, buffStderr bytes.Buffer
	code := 0

	cmd := exec.Command(os.Args[0], args...)
	cmd.Stdout = &buffStdout
	cmd.Stderr = &buffStderr

	cmd.Env = append(os.Environ(), "BE_GONIT=1")

	err := cmd.Run()

	if err != nil {
		code = err.(*exec.ExitError).Sys().(syscall.WaitStatus).ExitStatus()
	}

	return CmdResult{code: code, stdout: buffStdout.String(), stderr: buffStderr.String()}
}

func TestMain(m *testing.M) {
	if os.Getenv("BE_GONIT") == "1" {
		main()
		os.Exit(0)
		return
	}
	flag.Parse()
	c := m.Run()
	os.Exit(c)
}
