package cmd

import "github.com/spf13/cobra"

var unmonitorCmd = NewValidatedCommand("unmonitor", cobra.Command{
	Use:   "unmonitor [name|all]",
	Short: "Unmonitor service",
	Long:  "Unmonitor a service by name or all of them",
}, 0, 1, func(cmd *cobra.Command, args []string) {
	RunCheckCommandAndExit("unmonitor", args)
})

func init() {
	RootCmd.AddCommand(unmonitorCmd)
}
