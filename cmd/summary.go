package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// statusCmd represents the status command
var summaryCmd = &cobra.Command{
	Use:   "summary",
	Short: "Print short status information for each service",
	Long:  "Print short status information for each service",
	Run: func(cmd *cobra.Command, args []string) {
		if !isDaemonRunning() {
			fmt.Fprintf(os.Stderr, "Cannot find any running daemon to contact. If it is running, make sure you are pointing to the right pid file (%s)\n", daemonPidFile())
			os.Exit(1)
		} else if cm := getChecksManager(); cm != nil {
			str := cm.SummaryText()
			if str != "" {
				fmt.Println(str)
			} else {
				fmt.Fprintln(os.Stderr, "Got empty summary text")
				os.Exit(1)
			}
		} else {
			fmt.Fprintf(os.Stderr, "The daemon seems to be running but it does not seem to be accessible through socket.\n")
			os.Exit(1)
		}
	},
}

func init() {
	RootCmd.AddCommand(summaryCmd)
}
