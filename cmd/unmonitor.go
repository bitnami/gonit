package cmd

import "github.com/spf13/cobra"

var unmonitorCmd = newValidatedCommand("unmonitor", cobra.Command{
	Use:   "unmonitor [name|all]",
	Short: "Unmonitor service",
	Long:  "Unmonitor a service by name or all of them",
}, 0, 1, func(cmd *cobra.Command, args []string) {
	runCheckCommandAndExit("unmonitor", args)
})

func init() {
	RootCmd.AddCommand(unmonitorCmd)
}
