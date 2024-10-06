package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use: "simple-tunnel",
	Short: "Simple HTTP Tunnel",
	SilenceUsage: true,
}

func Execute() {
	rootCmd.AddCommand(StartCommand().cmd)
	rootCmd.AddCommand(ServeCommand().cmd)

	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}
