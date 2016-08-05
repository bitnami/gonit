package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"

	"github.com/bitnami/gonit/log"
	"github.com/bitnami/gonit/monitor"
	"github.com/bitnami/gonit/utils"

	"github.com/VividCortex/godaemon"
	"github.com/spf13/cobra"
)

var RootCmd = &cobra.Command{
	Use: filepath.Base(os.Args[0]),
	Run: func(cmd *cobra.Command, args []string) {
		RunMonitor()
	},
}

func init() {
	addGlobalFlags()
}

func setupSignals(app *monitor.Monitor) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		for {
			s := <-c
			switch s {
			case syscall.SIGHUP:
				app.Reload()
			case syscall.SIGINT:
				fallthrough
			case syscall.SIGTERM:
				app.Terminate()
				os.Exit(0)
			}
		}
	}()
}

func printRuntimeDebugStats(l *log.Logger) {
	stats := runtime.MemStats{}
	runtime.ReadMemStats(&stats)
	str := fmt.Sprintf("RUNTIME DEBUG:\n")
	for title, value := range map[string]interface{}{
		"Routines Running": runtime.NumGoroutine(),
		"Memory":           fmt.Sprintf("%dKB", stats.Alloc/1024),
	} {
		str += fmt.Sprintf("%-40s %15v\n", title, value)
	}
	l.MDebugf(str)
}

func RunMonitor() {
	c := GetConfig()
	if godaemon.Stage() == godaemon.StageParent {
		// Make sure we are the only ones that set this var
		os.Unsetenv("GO_DAEMON_CWD")

		if IsDaemonRunning() {
			fmt.Printf("daemon with PID %d awakened\n", DaemonPid())
			ReloadDaemon()
			os.Exit(0)
		} else if c.ShouldDaemonize {
			if err := utils.ValidatePidFilePath(c.PidFile); err != nil {
				fmt.Fprintln(os.Stderr, err.Error())
				os.Exit(1)
			}
			if c.SocketFile != "" {
				// Early abort before trying to start the daemon
				utils.EnsurePermissions(c.SocketFile, 0660)
			}

			greetMsg := fmt.Sprintf("Starting %s daemon", filepath.Base(os.Args[0]))

			// TODO: At this point, the config does not have this info, we have to improve
			// the code so GetConfig retrieves info from the conf files
			// if c.Server.IsEnabled() {
			//   greetMsg += fmt.Sprintf(" listening at %s", c.Server.ConnectionString())
			// }
			fmt.Println(greetMsg)
			dir, _ := os.Getwd()
			os.Setenv("GO_DAEMON_CWD", dir)
			_, _, _ = godaemon.MakeDaemon(&godaemon.DaemonAttr{})
		}
	}

	app := InitApp(c)
	if err := utils.WritePid(c.PidFile, app.Pid); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	setupSignals(app)
	if app.HTTPServerSupported() {
		app.StartServer()
	}
	app.LoopForever(make(chan bool))

	// go func() {
	// 	if app.HttpServerSupported() {
	// 		app.StartServer()
	// 	}
	// 	// Start serializing data (like the uptime)
	// 	app.UpdateDatabase()
	// 	for {
	// 		if os.Getenv("BITNAMI_DEBUG") != "" {
	// 			printRuntimeDebugStats(l)
	// 		}
	// 		app.Check()
	// 		app.UpdateDatabase()
	// 		time.Sleep(app.CheckInterval)
	// 	}
	// }()
	// WaitForever()
}
