package main

import (
	"fmt"
	"os"

	"github.com/bitnami/gonit/cmd"
)

var version = "0.2.2"
var buildDate = ""
var commit = ""

func main() {
	cmd.SetVersionInformation(version, buildDate, commit)
	if err := cmd.RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}
}
