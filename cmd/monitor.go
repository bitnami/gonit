package cmd

import "github.com/spf13/cobra"

var monitorCmd = newValidatedCommand("monitor", cobra.Command{
	Use:   "monitor [name|all]",
	Short: "Monitor service",
	Long:  "Monitor a service by name or all of them",
}, 0, 1, func(cmd *cobra.Command, args []string) {
	runCheckCommandAndExit("monitor", args)
})

func init() {
	RootCmd.AddCommand(monitorCmd)
}
