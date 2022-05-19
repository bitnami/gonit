package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// statusCmd represents the status command
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Print full status information for each service",
	Long:  "Print full status information for each service",
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) > 1 {
			fmt.Fprintln(os.Stderr, "Too many arguments provided. Only an optional service name is allowed")
			os.Exit(1)
		}
		if !isDaemonRunning() {
			fmt.Fprintf(os.Stderr, "Cannot find any running daemon to contact. If it is running, make sure you are pointing to the right pid file (%s)\n", daemonPidFile())
			os.Exit(1)

			//lint:ignore SA4023 The process is not expected to be running when performing static code check
		} else if cm := getChecksManager(); cm != nil {
			str := cm.StatusText(args...)
			if str != "" {
				fmt.Println(str)
			} else {
				fmt.Fprintln(os.Stderr, "Got empty status text")
				os.Exit(1)
			}
		} else {
			fmt.Fprintf(os.Stderr, "The daemon seems to be running but it does not seem to be accessible through socket.\n")
			os.Exit(1)
		}
	},
}

func init() {
	RootCmd.AddCommand(statusCmd)
}
