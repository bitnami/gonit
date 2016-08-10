package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var version = "0.0.0"
var buildDate = ""
var commit = ""

// SetVersionInformation allows providing the version details
// to be printed by the version command
func SetVersionInformation(v string, date, commitHash string) {
	version = strings.TrimSpace(v)
	buildDate = strings.TrimSpace(date)
	commit = strings.TrimSpace(commitHash)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of Gonit",
	Run: func(cmd *cobra.Command, args []string) {
		msg := fmt.Sprintf("Gonit %s\n", version)
		if buildDate != "" {
			msg += fmt.Sprintf("Built on: %s\n", buildDate)
		}
		if commit != "" {
			msg += fmt.Sprintf("Git Commit: %s\n", commit)
		}
		fmt.Printf(msg)
		os.Exit(0)
	},
}

func init() {
	RootCmd.AddCommand(versionCmd)
}
