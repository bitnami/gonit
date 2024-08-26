package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/bitnami/gonit/monitor"
	"github.com/bitnami/gonit/utils"
	"github.com/spf13/cobra"
)

var (
	// ControlFile configures the main gonit configuration file
	ControlFile string
	// Verbose increases the log verbosity when set to true
	Verbose bool
	// DaemonizeInterval configures the number of seconds between checks when
	// running in loop mode
	DaemonizeInterval int
	// PidFile configures the path to the gonit Pid file
	PidFile string
	// StateFile configures the path to the gonit state database
	StateFile string
	// LogFile configures the path to the gonit log file
	LogFile string
	// Foreground makes the process run the foreground instead of trying to daemonize it
	Foreground bool
	// SocketFile configures the path to the Unix Socket when enabling the HTTP interface
	SocketFile string
)

func addGlobalFlags() {
	RootCmd.PersistentFlags().IntVarP(&DaemonizeInterval, "daemonize", "d", 0, "Run as a daemon once per `n` seconds")
	RootCmd.PersistentFlags().BoolVarP(&Foreground, "foreground", "I", false, "Do not run in background (needed for run from init)")
	RootCmd.PersistentFlags().BoolVarP(&Verbose, "verbose", "v", false, "Verbose mode, work noisy (diagnostic output)")
	RootCmd.PersistentFlags().StringVarP(&StateFile, "statefile", "s", "/var/lib/gonit/state", "Set the `file` gonit should write state information to")
	RootCmd.PersistentFlags().StringVarP(&ControlFile, "controlfile", "c", "/etc/gonit/gonitrc", "Use this control `file`")
	RootCmd.PersistentFlags().StringVarP(&PidFile, "pidfile", "p", "/var/run/gonit.pid", "Use this `pidfile` in daemon mode")
	// For now, disable the default value of /var/run/gonit.sock and force to
	// explicitly enable
	RootCmd.PersistentFlags().StringVarP(&SocketFile, "socketfile", "S", "/var/run/gonit.sock", "Use this `socketfile` to listen for requests in daemon mode")
	RootCmd.PersistentFlags().StringVarP(&LogFile, "logfile", "l", "/var/log/gonit.log", "Print log information to this `file`.")
}

func reloadDaemon() error {
	return syscall.Kill(daemonPid(), syscall.SIGHUP)
}

func quitDaemon() {
	pid := daemonPid()
	if err := syscall.Kill(daemonPid(), syscall.SIGTERM); err != nil {
		if !utils.WaitUntil(func() bool {
			return utils.IsProcessRunning(pid)
		}, 5*time.Second) {
			syscall.Kill(pid, syscall.SIGKILL)
		}
	}
}

func daemonPidFile() string {
	return getConfig().PidFile
}
func daemonPid() int {
	pid, _ := utils.ReadPid(daemonPidFile())
	return pid
}
func isDaemonRunning() bool {
	return utils.IsProcessRunning(daemonPid())
}

func initApp(c monitor.Config) *monitor.Monitor {
	app, err := monitor.New(c)
	if err != nil {
		utils.Exit(1, "Error initializing application: %s\n", err.Error())
	}
	return app
}

func absFile(p string, root string) string {
	if p == "" {
		return p
	}
	return utils.AbsFileFromRoot(p, root)
}

func unimplementedCommand(name string) *cobra.Command {
	return &cobra.Command{
		Use: name,
		Run: func(cmd *cobra.Command, args []string) {
			utils.Exit(1, "%s not implemented!", name)
		},
	}
}

func getConfig() monitor.Config {
	cwd, _ := os.Getwd()
	if os.Getenv("GO_DAEMON_CWD") != "" {
		cwd = os.Getenv("GO_DAEMON_CWD")
	}

	// We have to find a better way of checking if a flag was provided so we can
	// use 120 as DaemonizeInterval and simply use it
	interval := 0
	if DaemonizeInterval != 0 {
		interval = DaemonizeInterval
	} else {
		interval = 120
	}

	cfg := monitor.Config{
		ControlFile:   ControlFile,
		Verbose:       Verbose,
		PidFile:       absFile(PidFile, cwd),
		StateFile:     absFile(StateFile, cwd),
		SocketFile:    absFile(SocketFile, cwd),
		CheckInterval: time.Duration(interval) * time.Second,
	}

	if LogFile == "" || LogFile == "-" {
		cfg.LogFile = LogFile
	} else {
		cfg.LogFile = absFile(LogFile, cwd)
	}

	// Some basic sanitization of file permissions. This are enforced later on
	// when opening them, but makes sense to report errors as soon as possible
	for _, f := range []string{cfg.ControlFile, cfg.StateFile} {
		utils.EnsureSafePermissions(f)
	}
	if cfg.ControlFile != "" && !utils.FileExists(cfg.ControlFile) {
		utils.Exit(1, "Control file '%s' does not exists", cfg.ControlFile)
	}

	if Foreground {
		cfg.ShouldDaemonize = false
	} else {
		cfg.ShouldDaemonize = true
	}
	return cfg
}

func getChecksManager() interface {
	monitor.ChecksManager
} {
	var manager interface {
		monitor.ChecksManager
	}
	if isDaemonRunning() {
		// TODO, this is just to get the config..., we should mot need the App
		app := initApp(getConfig())
		if app.HTTPServerSupported() {
			manager = monitor.NewClient(app.SocketFile)
		}
	}
	if manager == nil {
		manager = initApp(getConfig())
	}
	return manager
}
func flattenErrors(errList []error) error {
	if len(errList) == 0 {
		return nil
	}
	msgs := []string{}
	for _, err := range errList {
		msgs = append(msgs, err.Error())
	}
	return errors.New(strings.Join(msgs, "\n"))
}

type serviceCommand struct {
	cmd           string
	singleCheckCb func(string) error
	multicheckCb  func() []error
}

func (sc *serviceCommand) Execute(arg string) (string, error) {
	statuses := map[string]string{
		"start":     "Started",
		"stop":      "Stopped",
		"restart":   "Restarted",
		"monitor":   "Monitored",
		"unmonitor": "Unmonitored",
	}
	status := statuses[sc.cmd]

	var msg string
	var err error
	if arg == "" || arg == "all" {
		err = flattenErrors(sc.multicheckCb())
	} else {
		err = sc.singleCheckCb(arg)
		if err != nil {
			err = fmt.Errorf("Failed to %s %s: %s", sc.cmd, arg, err.Error())
		} else {
			msg = fmt.Sprintf("%s %s", status, arg)
		}
	}

	if err != nil {
		return "", err
	}
	return msg, nil
}

func runCheckCommandAndExit(cmd string, args []string) {
	cm := getChecksManager()
	msg, code, err := runCheckCommand(cm, cmd, args)
	if code != 0 {
		msg = err.Error()
	}
	utils.Exit(code, msg)
}
func runCheckCommand(cm interface {
	monitor.ChecksManager
}, cmd string, args []string) (msg string, code int, err error) {
	var arg string
	if len(args) == 0 {
		arg = ""
	} else {
		arg = args[0]
	}

	var sc *serviceCommand

	switch cmd {
	case "start":
		sc = &serviceCommand{
			cmd:           cmd,
			singleCheckCb: cm.Start,
			multicheckCb:  cm.StartAll,
		}
	case "stop":
		sc = &serviceCommand{
			cmd:           cmd,
			singleCheckCb: cm.Stop,
			multicheckCb:  cm.StopAll,
		}
	case "restart":
		sc = &serviceCommand{
			cmd:           cmd,
			singleCheckCb: cm.Restart,
			multicheckCb:  cm.RestartAll,
		}
	case "monitor":
		sc = &serviceCommand{
			cmd:           cmd,
			singleCheckCb: cm.Monitor,
			multicheckCb:  cm.MonitorAll,
		}

	case "unmonitor":
		sc = &serviceCommand{
			cmd:           cmd,
			singleCheckCb: cm.Unmonitor,
			multicheckCb:  cm.UnmonitorAll,
		}
	default:
		return "", -1, fmt.Errorf("Unknown command %s", cmd)
	}
	msg, err = sc.Execute(arg)
	if err != nil && code == 0 {
		code = 1
	}
	return msg, code, err
}

func newValidatedCommand(name string, baseCmd cobra.Command, minArgs int, maxArgs int, cb func(*cobra.Command, []string)) *cobra.Command {
	baseCmd.Run = func(cmd *cobra.Command, args []string) {
		if minArgs == maxArgs && len(args) != minArgs {
			utils.Exit(2, "Command %s requires exactly %d arguments but %d were provided", name, maxArgs, len(args))
		} else {
			if len(args) > maxArgs {
				utils.Exit(2, "Command %s requires at most %d arguments but %d were provided", name, maxArgs, len(args))
			} else if len(args) < minArgs {
				utils.Exit(2, "Command %s requires at least %d arguments but %d were provided", name, minArgs, len(args))
			}
		}
		cb(cmd, args)
	}
	return &baseCmd
}
