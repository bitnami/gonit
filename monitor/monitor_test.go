package monitor

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"sync"
	"syscall"
	"testing"
	"time"

	gt "github.com/bitnami/gonit/gonittest"
	"github.com/bitnami/gonit/log"
	tu "github.com/bitnami/gonit/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	sb *tu.Sandbox
)

var (
	statusTextHeaderPattern = `(.*(Uptime|Last Check|Next Check|Pid|Pid File|Control File|Socket File|Log File).*\n)+`
)

type trackedLogger struct {
	sync.RWMutex
	log.Logger
	debug      []string
	debugTrack *regexp.Regexp
}

func (tl *trackedLogger) GetEntries() []string {
	defer tl.RUnlock()
	tl.RLock()
	return tl.debug
}
func (tl *trackedLogger) MDebugf(format string, args ...interface{}) {
	if tl.debugTrack != nil && tl.debugTrack.MatchString(format) {
		defer tl.Unlock()
		tl.Lock()
		tl.debug = append(tl.debug, format)
	}
}

func newTrackedLogger() *trackedLogger {
	return &trackedLogger{Logger: *log.DummyLogger()}
}

func TestMain(m *testing.M) {

	sb = tu.NewSandbox()
	c := m.Run()

	sb.Cleanup()
	os.Exit(c)
}
func TestNew(t *testing.T) {
	app, err := New(Config{})
	require.NoError(t, err)
	assert.Equal(t, app.Pid, syscall.Getpid(), "Invalid Pid field value")

	// CheckInterval defaults to 100ms
	assert.Equal(t, app.CheckInterval, 100*time.Millisecond)
	// But can be modified
	for _, d := range []time.Duration{time.Millisecond, 5 * time.Second} {
		a, _ := New(Config{CheckInterval: d})
		assert.Equal(t, a.CheckInterval, d)
	}

}

func TestNewWithConfigFile(t *testing.T) {
	dir, _ := sb.Mkdir(sb.TempFile(), os.FileMode(0755))

	file := filepath.Join(dir, "gonit.cfg")
	pidFile := filepath.Join(dir, "temp/sample.pid")

	require.NoError(t, gt.RenderSampleProcessCheck(file, gt.CfgOpts{
		Name:    "sample",
		RootDir: dir,
		PidFile: pidFile,
		Timeout: "1",
	}))
	os.Chmod(file, os.FileMode(0700))
	app, err := New(Config{ControlFile: file})
	assert.NoError(t, err)
	assert.Len(t, app.checks, 1)
	c := app.FindCheck("sample").(*ProcessCheck)

	for actual, expected := range map[string]string{
		c.GetID(): "sample",
		c.PidFile: pidFile,
	} {
		assert.Equal(t, actual, expected)
	}
}

func TestUptime(t *testing.T) {
	t.Parallel()
	start := time.Now()
	ellapsed := 1000 * time.Millisecond
	app, err := New(Config{})
	require.NoError(t, err)

	time.Sleep(ellapsed)
	end := time.Now()
	assert.WithinDuration(t, start.Add(app.Uptime()), end, 100*time.Millisecond)
}

func TestUpdateDatabase(t *testing.T) {
	dbFile := sb.TempFile()
	ids := []string{"foo", "bar"}
	tu.AssertFileDoesNotExist(t, dbFile)
	app, err := New(Config{StateFile: dbFile})
	require.NoError(t, err)
	// Initializing the app does not automatically create the database
	tu.AssertFileDoesNotExist(t, dbFile)
	// It gets created at first update
	app.UpdateDatabase()
	tu.AssertFileExists(t, dbFile)

	db := app.database
	assert.Len(t, db.Keys(), 0)
	for _, id := range ids {
		app.AddCheck(&ProcessCheck{check: &check{ID: id}})
	}
	db.Deserialize()
	assert.Len(t, db.Keys(), 0)

	// Propertly registers new checks
	app.UpdateDatabase()
	db.Deserialize()
	sort.Strings(ids)
	assert.Equal(t, db.Keys(), ids)

	// Deregisters unknown checks
	app.checks = nil
	app.AddCheck(&ProcessCheck{check: &check{ID: "foo"}})
	app.UpdateDatabase()
	db.Deserialize()
	assert.Equal(t, db.Keys(), []string{"foo"})
}

func TestFindCheck(t *testing.T) {
	ids := []string{"foo", "bar"}
	app, err := New(Config{})
	require.NoError(t, err)
	checks := make(map[string]interface {
		Checkable
	})
	for _, id := range ids {
		ch := &ProcessCheck{check: &check{ID: id}}
		checks[id] = ch
		app.AddCheck(ch)
	}

	for _, id := range ids {
		assert.Equal(t, app.FindCheck(id), checks[id])
	}
	for _, id := range []string{"", "asdf"} {
		assert.Equal(t, app.FindCheck(id), nil)
	}
}

func TestAddCheck(t *testing.T) {
	ids := []string{"foo", "bar"}
	app, err := New(Config{})
	require.NoError(t, err)
	logger := app.logger
	for _, id := range ids {
		ch := &ProcessCheck{check: &check{ID: id}}
		assert.NoError(t, app.AddCheck(ch))
		// When adding the check, the app set it up to use its logger
		// TODO: Maybe this should not be the case, so a check can be registered
		// in many monitors?
		assert.Equal(t, ch.logger, logger)
	}

	for _, id := range ids {
		ch := &ProcessCheck{check: &check{ID: id}}
		tu.AssertErrorMatch(t, app.AddCheck(ch), regexp.MustCompile("Service name conflict,.*already defined"))
	}
}

func TestReaload(t *testing.T) {
	rootDir, _ := sb.Mkdir(sb.TempFile(), os.FileMode(0755))
	cfgFile := filepath.Join(rootDir, "gonit.cfg")
	require.NoError(t,
		gt.RenderSampleProcessCheck(cfgFile, gt.CfgOpts{
			Name:    "foo",
			RootDir: rootDir,
		}),
	)
	os.Chmod(cfgFile, os.FileMode(0700))
	app, err := New(Config{ControlFile: cfgFile})
	require.NoError(t, err)
	require.NotEqual(t, app.FindCheck("foo"), nil)
	require.Equal(t, app.FindCheck("bar"), nil)

	// Reloads configuration
	require.NoError(t,
		gt.RenderSampleProcessCheck(cfgFile, gt.CfgOpts{
			Name:    "bar",
			RootDir: rootDir,
		}),
	)
	os.Chmod(cfgFile, os.FileMode(0700))
	err = app.Reload()
	assert.NoError(t, err)

	require.NotEqual(t, app.FindCheck("bar"), nil)
	require.Equal(t, app.FindCheck("foo"), nil)

	// Do not reload incorrect config
	sb.Write(cfgFile, "check process foo\ncheck process foo\ncheck process bar\n")
	os.Chmod(cfgFile, os.FileMode(0700))
	existingChecks := app.checks
	err = app.Reload()
	tu.AssertErrorMatch(t, err, regexp.MustCompile("Refusing to reload"))
	assert.Equal(t, existingChecks, app.checks)
}

func TestLoopForever(t *testing.T) {
	t.Parallel()
	dbFile := sb.TempFile()
	checkInterval := 10 * time.Millisecond
	app, err := New(Config{CheckInterval: checkInterval, StateFile: dbFile})

	require.NoError(t, err)

	dc1 := newDummyCheck("dummy1")

	dc1.waitTime = time.Millisecond
	dc2 := newDummyCheck("dummy2")
	dc2.waitTime = 20 * time.Millisecond

	app.AddCheck(dc1)
	app.AddCheck(dc2)

	assert.Equal(t, dc1.getTimesCalled(), 0)
	assert.Equal(t, dc2.getTimesCalled(), 0)

	assert.NoError(t, app.Unmonitor("dummy2"))

	stopCh := make(chan bool)
	go app.LoopForever(stopCh)

	// We add some tolerance
	time.Sleep(20*checkInterval + (20 * time.Millisecond))
	stopCh <- true
	// Give it some time to abort the loop
	time.Sleep(100 * time.Millisecond)

	tc1 := dc1.getTimesCalled()
	tc2 := dc2.getTimesCalled()

	// At least it should have been called 5 times
	assert.True(t, tc1 > 5, "Expected to be called at least 5 times but got %d", tc1)

	// it is stopped so it should not change
	time.Sleep(100 * time.Millisecond)

	assert.Equal(t, tc1, dc1.getTimesCalled())
	assert.Equal(t, tc2, dc2.getTimesCalled())
}

// TODO: This is slow, move it to the main monit tests once ready
func TestLoopForeverStopsCheckingMultipleErrors(t *testing.T) {
	t.Parallel()
	rootDir, _ := sb.Mkdir(sb.TempFile("forever_root"), os.FileMode(0755))
	id1 := "sample1"
	id2 := "sample2"
	app, err := loadDummyScenario(t, rootDir, []string{id1, id2})
	require.NoError(t, err)
	v1 := app.FindCheck(id1)
	require.NotNil(t, v1)
	v2 := app.FindCheck(id2)
	require.NotNil(t, v2)
	ch1 := v1.(*ProcessCheck)
	ch2 := v2.(*ProcessCheck)
	assert.False(t, ch1.IsRunning())
	assert.False(t, ch2.IsRunning())
	ch1.maxStartTries = 1
	app.CheckInterval = 100 * time.Millisecond
	stopCh := make(chan bool)
	go app.LoopForever(stopCh)
	time.Sleep(10 * app.CheckInterval)
	assert.True(t, ch1.IsRunning())
	assert.True(t, ch2.IsRunning())

	assert.Equal(t, ch1.startTriesCnt.Get(), 0)
	assert.Equal(t, ch2.startTriesCnt.Get(), 0)

	doErrorFile := filepath.Join(rootDir, fmt.Sprintf("%s.doerror", id1))
	sb.Touch(doErrorFile)
	sb.Touch(filepath.Join(rootDir, fmt.Sprintf("%s.stop", id1)))

	time.Sleep(3 * time.Second)
	assert.True(t, ch2.IsRunning())
	assert.False(t, ch1.IsRunning())
	assert.False(t, ch1.IsMonitored())
	os.Remove(doErrorFile)
	app.Start(ch1.GetID())
	time.Sleep(3 * time.Second)
	assert.True(t, ch1.IsMonitored())
}

func TestHTTPServerSupported(t *testing.T) {
	app, err := New(Config{})
	require.NoError(t, err)
	assert.False(t, app.HTTPServerSupported())
	app.SocketFile = sb.TempFile()
	assert.True(t, app.HTTPServerSupported())

	newApp, err := New(Config{SocketFile: sb.TempFile()})
	require.NoError(t, err)
	assert.True(t, newApp.HTTPServerSupported())
}

func TestStartServer(t *testing.T) {
	app, err := New(Config{})
	require.NoError(t, err)
	tu.AssertErrorMatch(t, app.StartServer(), regexp.MustCompile(`Don't know how to start the HTTP server \(missing socket\)`))
	nonWritableDir, _ := sb.Mkdir(sb.TempFile(), os.FileMode(0555))
	app.SocketFile = filepath.Join(nonWritableDir, "sample.sock")
	tu.AssertErrorMatch(t, app.StartServer(), regexp.MustCompile("Error listening to socket"))

	app.SocketFile = sb.TempFile()
	assert.NoError(t, app.StartServer())
	defer app.Terminate()
	// 	app, err := New(Config{CheckInterval: time.Millisecond})

	client := &http.Client{
		Timeout: 15 * time.Second,
		Transport: &http.Transport{
			Dial: func(_ string, _ string) (net.Conn, error) {
				return net.Dial("unix", app.SocketFile)
			},
		},
	}
	r, err := client.Get("http://localhost/status")
	assert.NoError(t, err)
	cmdResp := cmdResponse{}
	assert.NoError(t, json.NewDecoder(r.Body).Decode(&cmdResp))
	assert.True(t, cmdResp.Success)
	assert.Regexp(t, regexp.MustCompile(".*Uptime.*\nLast Check.*\nNext Check.*"),
		cmdResp.Msg)
}
func TestTerminate(t *testing.T) {
	app, err := New(Config{SocketFile: sb.TempFile()})
	require.NoError(t, err)
	assert.Nil(t, app.server)
	assert.NoError(t, app.StartServer())
	assert.NotNil(t, app.server)
	client := &http.Client{
		Timeout: 15 * time.Second,
		Transport: &http.Transport{
			Dial: func(_ string, _ string) (net.Conn, error) {
				return net.Dial("unix", app.SocketFile)
			},
		},
	}
	_, err = client.Get("http://localhost/status")
	assert.NoError(t, err)

	assert.NoError(t, app.Terminate())

	_, err = client.Get("http://localhost/status")
	// The sockect is gone
	tu.AssertErrorMatch(t, err, regexp.MustCompile("dial.*connect: no such file or directory"))
	assert.Nil(t, app.server)

	app2, err := New(Config{SocketFile: sb.TempFile()})
	require.NoError(t, err)
	assert.NoError(t, app2.StartServer())
	require.NoError(t, app2.server.Stop())
	// At this point the server is stopped, terminate should complain
	tu.AssertErrorMatch(t, app2.Terminate(), regexp.MustCompile("close unix.*use of closed network connection"))
}

func testServiceCommands(t *testing.T, app *Monitor, cm interface {
	ChecksManager
}) {
	var ts1, ts2 int
	ch1 := newDummyService("foo")
	ch2 := newDummyService("bar")
	nonProcessCheck := &check{ID: "nonprocess"}

	require.NoError(t, app.AddCheck(ch1))
	require.NoError(t, app.AddCheck(ch2))
	require.NoError(t, app.AddCheck(nonProcessCheck))

	for _, fn := range [](func(string) error){
		cm.Start, cm.Stop, cm.Restart, cm.Monitor, cm.Unmonitor,
	} {
		tu.AssertErrorMatch(t,
			fn("notfoundcheck"),
			regexp.MustCompile("Cannot find check with id.*"),
		)
	}

	for _, fn := range [](func(string) error){
		cm.Start, cm.Stop, cm.Restart,
	} {
		tu.AssertErrorMatch(t, fn(nonProcessCheck.GetID()), regexp.MustCompile("Check.*is not a process"))
	}

	assert.False(t, ch1.IsRunning())
	assert.False(t, ch2.IsRunning())

	assert.NoError(t, cm.Start(ch1.GetID()))
	assert.True(t, ch1.IsRunning())
	assert.NoError(t, cm.Stop(ch1.GetID()))
	assert.False(t, ch1.IsRunning())
	ts1 = ch1.getTimesStarted()
	assert.NoError(t, cm.Restart(ch1.GetID()))
	assert.True(t, ch1.IsRunning())
	assert.Equal(t, ch1.getTimesStarted(), ts1+1)

	assert.Len(t, cm.StartAll(), 0, "It should return an empty set of errors")
	assert.True(t, ch1.IsRunning())
	assert.True(t, ch2.IsRunning())

	ts1 = ch1.getTimesStarted()
	ts2 = ch2.getTimesStarted()
	assert.Len(t, cm.RestartAll(), 0, "It should return an empty set of errors")
	assert.True(t, ch1.IsRunning())
	assert.True(t, ch2.IsRunning())
	assert.Equal(t, ch1.getTimesStarted(), ts1+1)
	assert.Equal(t, ch2.getTimesStarted(), ts2+1)

	assert.Len(t, cm.StopAll(), 0, "It should return an empty set of errors")
	assert.False(t, ch1.IsRunning())
	assert.False(t, ch2.IsRunning())

	assert.True(t, ch1.IsMonitored())
	assert.True(t, ch2.IsMonitored())
	assert.NoError(t, cm.Unmonitor(ch1.GetID()))
	assert.NoError(t, cm.Unmonitor(ch2.GetID()))

	assert.False(t, ch1.IsMonitored())
	assert.False(t, ch2.IsMonitored())
	assert.NoError(t, cm.Monitor(ch1.GetID()))
	assert.NoError(t, cm.Monitor(ch2.GetID()))

	assert.True(t, ch1.IsMonitored())
	assert.True(t, ch2.IsMonitored())

	assert.Len(t, cm.UnmonitorAll(), 0, "It should return an empty set of errors")
	assert.False(t, ch1.IsMonitored())
	assert.False(t, ch2.IsMonitored())

	assert.Len(t, cm.MonitorAll(), 0, "It should return an empty set of errors")
	assert.True(t, ch1.IsMonitored())
	assert.True(t, ch2.IsMonitored())

}

func TestMonitorClient(t *testing.T) {
	t.Parallel()
	dbFile := sb.TempFile()
	app, err := New(Config{
		CheckInterval: time.Millisecond,
		SocketFile:    sb.TempFile(),
		StateFile:     dbFile,
	})
	require.NoError(t, err)
	assert.NoError(t, app.StartServer())
	defer app.Terminate()

	cm := NewClient(app.SocketFile)

	var ts1, ts2 int
	ch1 := newDummyService("foo")
	ch2 := newDummyService("bar")
	nonProcessCheck := &check{ID: "nonprocess"}

	require.NoError(t, app.AddCheck(ch1))
	require.NoError(t, app.AddCheck(ch2))
	require.NoError(t, app.AddCheck(nonProcessCheck))

	for _, fn := range [](func(string) error){
		cm.Start, cm.Stop, cm.Restart, cm.Monitor, cm.Unmonitor,
	} {
		tu.AssertErrorMatch(t,
			fn("notfoundcheck"),
			regexp.MustCompile("Cannot find check with id.*"),
		)
	}

	for _, fn := range [](func(string) error){
		cm.Start, cm.Stop, cm.Restart,
	} {
		tu.AssertErrorMatch(t, fn(nonProcessCheck.GetID()), regexp.MustCompile("Check.*is not a process"))
	}

	assert.False(t, ch1.IsRunning())
	assert.False(t, ch2.IsRunning())

	assert.NoError(t, cm.Start(ch1.GetID()))
	time.Sleep(300 * time.Millisecond)
	assert.True(t, ch1.IsRunning())

	assert.NoError(t, cm.Stop(ch1.GetID()))
	time.Sleep(300 * time.Millisecond)
	assert.False(t, ch1.IsRunning())

	ts1 = ch1.getTimesStarted()

	assert.NoError(t, cm.Restart(ch1.GetID()))
	time.Sleep(300 * time.Millisecond)
	assert.True(t, ch1.IsRunning())
	time.Sleep(300 * time.Millisecond)
	assert.Equal(t, ts1+1, ch1.getTimesStarted())
	assert.Len(t, cm.StartAll(), 0, "It should return an empty set of errors")
	time.Sleep(300 * time.Millisecond)
	assert.True(t, ch1.IsRunning())
	assert.True(t, ch2.IsRunning())

	ts1 = ch1.getTimesStarted()
	ts2 = ch2.getTimesStarted()
	assert.Len(t, cm.RestartAll(), 0, "It should return an empty set of errors")
	time.Sleep(300 * time.Millisecond)
	assert.True(t, ch1.IsRunning())
	assert.True(t, ch2.IsRunning())
	assert.Equal(t, ch1.getTimesStarted(), ts1+1)
	assert.Equal(t, ch2.getTimesStarted(), ts2+1)

	assert.Len(t, cm.StopAll(), 0, "It should return an empty set of errors")
	time.Sleep(300 * time.Millisecond)
	assert.False(t, ch1.IsRunning())
	assert.False(t, ch2.IsRunning())

	assert.True(t, ch1.IsMonitored())
	assert.True(t, ch2.IsMonitored())

	assert.NoError(t, cm.Unmonitor(ch1.GetID()))
	assert.NoError(t, cm.Unmonitor(ch2.GetID()))

	assert.False(t, ch1.IsMonitored())
	assert.False(t, ch2.IsMonitored())
	assert.NoError(t, cm.Monitor(ch1.GetID()))
	assert.NoError(t, cm.Monitor(ch2.GetID()))

	assert.True(t, ch1.IsMonitored())
	assert.True(t, ch2.IsMonitored())

	assert.Len(t, cm.UnmonitorAll(), 0, "It should return an empty set of errors")
	assert.False(t, ch1.IsMonitored())
	assert.False(t, ch2.IsMonitored())

	assert.Len(t, cm.MonitorAll(), 0, "It should return an empty set of errors")
	assert.True(t, ch1.IsMonitored())
	assert.True(t, ch2.IsMonitored())

	//	testServiceCommands(t, app, cm)

	assert.Regexp(t,
		regexp.MustCompile(`^\s*Uptime \d+s?\s*\n\n(Process\s+[^\s]+\s+.*\n)+`),
		cm.SummaryText())

	assert.Regexp(t,
		regexp.MustCompile(statusTextHeaderPattern),
		cm.StatusText())
}

func TestServiceCommands(t *testing.T) {
	dbFile := sb.TempFile()
	app, err := New(Config{CheckInterval: time.Millisecond, StateFile: dbFile})
	require.NoError(t, err)
	testServiceCommands(t, app, app)
}
func TestLoopForeverLogStatsInDebugMode(t *testing.T) {
	t.Parallel()

	app, err := New(Config{CheckInterval: time.Millisecond})
	require.NoError(t, err)
	l := newTrackedLogger()
	l.debugTrack = regexp.MustCompile(".*RUNTIME DEBUG.*")
	app.logger = l

	stopCh := make(chan bool)
	go app.LoopForever(stopCh)
	time.Sleep(100 * time.Millisecond)
	assert.Len(t, l.GetEntries(), 0)
	os.Setenv("BITNAMI_DEBUG", "1")
	time.Sleep(100 * time.Millisecond)
	n := len(l.GetEntries())

	assert.True(t, n > 0, "It should have logged some RUNTIME debug but got %d messages", n)
	stopCh <- true
}

func TestSummaryText(t *testing.T) {
	app, err := New(Config{})
	require.NoError(t, err)
	// With no checks
	assert.Regexp(t, regexp.MustCompile(`^\s*Uptime 0s?\s*$`), app.SummaryText())
	dc1 := newDummyCheck("dummy1")
	// Make it to be running
	f, _ := sb.Write(sb.TempFile(), fmt.Sprintf("%d", syscall.Getpid()))
	dc1.PidFile = f

	dc2 := newDummyCheck("dummy2")

	app.AddCheck(dc1)
	app.AddCheck(dc2)
	headerPattern := `^\s*Uptime 0s?\s*\n\n`
	dummy1Pattern := `Process\s+dummy1\s+Running\n`
	dummy2Pattern := `Process\s+dummy2\s+Stopped\n`
	assert.Regexp(t, regexp.MustCompile(
		headerPattern+dummy1Pattern+dummy2Pattern+`$`,
	), app.SummaryText())
	assert.Regexp(t, regexp.MustCompile(
		headerPattern+dummy1Pattern+dummy2Pattern+`$`,
	), app.SummaryText("dummy1", "dummy2"))
	assert.Regexp(t, regexp.MustCompile(
		headerPattern+dummy2Pattern+dummy1Pattern+`$`,
	), app.SummaryText("dummy2", "dummy1"))
	assert.Regexp(t, regexp.MustCompile(
		headerPattern+dummy1Pattern+`$`,
	), app.SummaryText("dummy1"))
	assert.Regexp(t, regexp.MustCompile(
		headerPattern+dummy2Pattern+`$`,
	), app.SummaryText("dummy2"))

}

func TestStatusText(t *testing.T) {
	app, err := New(Config{})
	require.NoError(t, err)
	// With no checks
	assert.Regexp(t,
		regexp.MustCompile(
			statusTextHeaderPattern,
		), app.StatusText())

	dc1 := newDummyCheck("dummy1")
	// Make it to be running
	f, _ := sb.Write(sb.TempFile(), fmt.Sprintf("%d", syscall.Getpid()))
	dc1.PidFile = f

	dc2 := newDummyCheck("dummy2")

	app.AddCheck(dc1)
	app.AddCheck(dc2)
	dummy1Pattern := `\s*Process\s+'dummy1'\s*\n\s*status\s+Running\n\s*pid\s*\d+\n\s*uptime\s*\d+s?\n\s*monitoring status\s*monitored\n\s*`
	dummy2Pattern := `\s*Process\s+'dummy2'\s*\n\s*status\s+Stopped\n\s*uptime\s*0s?\n\s*monitoring status\s*monitored\n\s*`
	assert.Regexp(t, regexp.MustCompile(
		statusTextHeaderPattern+dummy1Pattern+`\n`+dummy2Pattern+`$`,
	), app.StatusText())
	assert.Regexp(t, regexp.MustCompile(
		statusTextHeaderPattern+dummy1Pattern+`\n`+dummy2Pattern+`$`,
	), app.StatusText("dummy1", "dummy2"))
	assert.Regexp(t, regexp.MustCompile(
		statusTextHeaderPattern+dummy2Pattern+`\n`+dummy1Pattern+`$`,
	), app.StatusText("dummy2", "dummy1"))
	assert.Regexp(t, regexp.MustCompile(
		statusTextHeaderPattern+dummy1Pattern+"$",
	), app.StatusText("dummy1"))
	assert.Regexp(t, regexp.MustCompile(
		statusTextHeaderPattern+dummy2Pattern+"$",
	), app.StatusText("dummy2"))
}

func loadDummyScenario(t *testing.T, rootDir string, ids []string) (*Monitor, error) {
	os.Chmod(rootDir, os.FileMode(0755))
	confDir, _ := sb.Mkdir(filepath.Join(rootDir, "conf"), os.FileMode(0755))
	os.Chmod(confDir, os.FileMode(0755))
	tmpDir, _ := sb.Mkdir(filepath.Join(rootDir, "temp"), os.FileMode(0755))
	os.Chmod(tmpDir, os.FileMode(0755))

	mainCfgTxt := ""

	for _, id := range ids {
		confFile := filepath.Join(confDir, fmt.Sprintf("%s.cfg", id))
		mainCfgTxt += fmt.Sprintf("include %s\n", confFile)

		script := filepath.Join(rootDir, fmt.Sprintf("loop-%s.sh", id))
		cfg := gt.CfgOpts{
			Name:         id,
			Timeout:      "2",
			TimeoutUnits: "seconds",
			RootDir:      rootDir,
			ConfDir:      confDir,
			StartCmd:     script,
			StopCmd:      "touch {{.RootDir}}/{{.Name}}.stop",
			TempDir:      tmpDir,
		}

		require.NoError(t, gt.RenderTemplate("sample-ctl-script", script, cfg))
		os.Chmod(script, os.FileMode(0755))
		require.NoError(t, gt.RenderTemplate("service-check", confFile, cfg))
	}
	cfgFile := filepath.Join(confDir, "gonit.conf")
	sb.Write(cfgFile, mainCfgTxt)
	os.Chmod(cfgFile, os.FileMode(0700))
	app, err := New(Config{ControlFile: cfgFile})
	for _, id := range ids {
		if c := app.FindCheck(id); c == nil {
			assert.Fail(t, "Could not find check for id %s", id)
			fmt.Println(app.checks)
		}
	}

	return app, err
}

func assertChecks(t *testing.T, app *Monitor, ids []string) (res map[string]interface {
	Checkable
}) {
	res = make(map[string]interface {
		Checkable
	})
	for _, id := range ids {
		check := app.FindCheck(id)
		assert.NotEqual(t, check, nil)
		res[id] = check
	}
	return res
}
func TestNewFromConfig(t *testing.T) {
	rootDir, _ := sb.Mkdir(sb.TempFile(), os.FileMode(0755))
	id1 := "sample1"
	id2 := "sample2"
	app, err := loadDummyScenario(t, rootDir, []string{id1, id2})
	assert.NoError(t, err)
	assertChecks(t, app, []string{id1, id2})
}

func TestServiceManagement(t *testing.T) {
	t.Parallel()
	rootDir, _ := sb.Mkdir(sb.TempFile("service_management"), os.FileMode(0755))
	id1 := "sample1"
	id2 := "sample2"
	app, err := loadDummyScenario(t, rootDir, []string{id1, id2})
	require.NoError(t, err)

	checks := assertChecks(t, app, []string{id1, id2})
	//	check1 := checks[id1].(*ProcessCheck)
	//	check2 := checks[id2].(*ProcessCheck)
	for id, check := range checks {
		assert.False(t, check.(*ProcessCheck).IsRunning(),
			"Expected %s to not be running", id)
	}
	app.Perform()
	time.Sleep(1500 * time.Millisecond)
	for id, check := range checks {
		assert.True(t, check.(*ProcessCheck).IsRunning(),
			"Expected %s to be running", id)
	}
}

func TestChecksPerformsOnlyOneTimeSimultaneously(t *testing.T) {
	//	t.Parallel()
	app, err := New(Config{})
	assert.NoError(t, err)
	dc := newDummyCheck("sample")
	//	dc.Initialize(Opts{})
	app.AddCheck(dc)
	dc.waitTime = 100 * time.Millisecond
	maxCalls := 10

	for i := 0; i < maxCalls; i++ {
		app.Perform()
	}
	time.Sleep(300 * time.Millisecond)

	tc := dc.getTimesCalled()
	assert.Equal(t, 1, tc,
		"Expected the number of times called to be 1 but got %d", tc)

	dc.setTimesCalled(0)
	dc.waitTime = 1 * time.Millisecond
	for i := 0; i < maxCalls; i++ {
		app.Perform()

		// This should give plenty of time for all calls to finish
		time.Sleep(100 * time.Millisecond)
	}

	tc = dc.getTimesCalled()
	assert.Equal(t, tc, maxCalls,
		"Expected the number of times called to be %d but got %d", maxCalls, tc)
}
