package cmd

import (
	"log"

	"github.com/ghousemohamed/simple-tunnel/internal/server"
	"github.com/spf13/cobra"
)

type startCommand struct {
	cmd              *cobra.Command
	httpPort         string
}

func StartCommand() *startCommand {
	startCommand := &startCommand{}
	startCommand.cmd = &cobra.Command{
		Use:   "start",
		Short: "Run the client server",
		RunE:  startCommand.run,
	}

	startCommand.cmd.Flags().StringVar(&startCommand.httpPort, "port", "8080", "Port to start tunnel server on")

	return startCommand
}

func (c *startCommand) run(cmd *cobra.Command, args []string) error {
	tunnel_server := server.NewServer(c.httpPort)
	err := tunnel_server.StartServer()

	if err != nil {
		log.Fatal("Error starting server", err)
		return err
	}

	return nil
}
