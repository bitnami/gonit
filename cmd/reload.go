package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var reloadCmd = &cobra.Command{
	Use:   "reload",
	Short: "Reinitialize tool",
	Long:  "Reinitialize tool",
	Run: func(cmd *cobra.Command, args []string) {
		if isDaemonRunning() {
			reloadDaemon()
		} else {
			fmt.Fprintf(os.Stderr, "Cannot find any running daemon to contact. If it is running, make sure you are pointing to the right pid file (%s)\n", daemonPidFile())
			os.Exit(1)
		}
	},
}

func init() {
	RootCmd.AddCommand(reloadCmd)
}
