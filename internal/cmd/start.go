package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

type startCommand struct {
	cmd              *cobra.Command
	httpPort         int
}

func StartCommand() *startCommand {
	startCommand := &startCommand{}
	startCommand.cmd = &cobra.Command{
		Use:   "start",
		Short: "Run the server",
		RunE:  startCommand.run,
	}

	startCommand.cmd.Flags().IntVar(&startCommand.httpPort, "port", 8080, "Port to start tunnel server on")

	return startCommand
}

func (c *startCommand) run(cmd *cobra.Command, args []string) error {
	fmt.Println(c.httpPort)
	return nil
}