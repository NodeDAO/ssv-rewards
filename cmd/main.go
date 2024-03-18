package main

import (
	logging "github.com/ipfs/go-log/v2"
	"github.com/spf13/cobra"
)

var log = logging.Logger("main")

var rootCmd = &cobra.Command{
	Use:   "ssv-reward",
	Short: "ssv-reward",
	Long:  `ssv reward calc`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
	},
}

func main() {
	_ = logging.SetLogLevel("*", "INFO")
	rootCmd.AddCommand(calcCmd)
	_ = rootCmd.Execute()
}
