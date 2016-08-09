package cmd

import "github.com/spf13/cobra"

var startCmd = newValidatedCommand("start", cobra.Command{
	Use:   "start [name|all]",
	Short: "Start service",
	Long:  "Start a service by name or all of them",
}, 0, 1, func(cmd *cobra.Command, args []string) {
	runCheckCommandAndExit("start", args)
})

func init() {
	RootCmd.AddCommand(startCmd)
}
