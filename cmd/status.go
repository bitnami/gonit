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
		if !IsDaemonRunning() {
			fmt.Fprintf(os.Stderr, "Cannot find any running daemon to contact. If it is running, make sure you are pointing to the right pid file (%s)\n", DaemonPidFile())
			os.Exit(1)

		} else if cm := GetChecksManager(); cm != nil {
			str := cm.StatusText()
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
