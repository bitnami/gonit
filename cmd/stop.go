package cmd

import "github.com/spf13/cobra"

var stopCmd = newValidatedCommand("stop", cobra.Command{
	Use:   "stop [name|all]",
	Short: "Stop service",
	Long:  "Stop a service by name or all of them",
}, 0, 1, func(cmd *cobra.Command, args []string) {
	runCheckCommandAndExit("stop", args)
})

func init() {
	RootCmd.AddCommand(stopCmd)
}
