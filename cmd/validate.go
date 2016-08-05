package cmd

var validateCmd = UnimplementedCommand("validate")

func init() {
	RootCmd.AddCommand(validateCmd)
}
