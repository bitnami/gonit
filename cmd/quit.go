package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var quitCmd = &cobra.Command{
	Use:   "quit",
	Short: "Terminate the execution of a running daemon",
	Run: func(cmd *cobra.Command, args []string) {
		if isDaemonRunning() {
			quitDaemon()
		} else {
			fmt.Fprintf(os.Stderr, "Cannot find any running daemon to stop. If it is running, make sure you are pointing to the right pid file (%s)\n", daemonPidFile())
			os.Exit(1)
		}
	},
}

func init() {
	RootCmd.AddCommand(quitCmd)
}
