package cmd

import "github.com/spf13/cobra"

var restartCmd = NewValidatedCommand("restart", cobra.Command{
	Use:   "restart [name|all]",
	Short: "Restart service",
	Long:  "Restart a service by name or all of them",
}, 0, 1, func(cmd *cobra.Command, args []string) {
	RunCheckCommandAndExit("restart", args)
})

func init() {
	RootCmd.AddCommand(restartCmd)
}
