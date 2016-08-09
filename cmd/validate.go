package cmd

var validateCmd = unimplementedCommand("validate")

func init() {
	RootCmd.AddCommand(validateCmd)
}
