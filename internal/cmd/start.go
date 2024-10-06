package cmd

import (
	"fmt"
	"log"
	"net/http"

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
		Short: "Run the server",
		RunE:  startCommand.run,
	}

	startCommand.cmd.Flags().StringVar(&startCommand.httpPort, "port", "8080", "Port to start tunnel server on")

	return startCommand
}

func (c *startCommand) run(cmd *cobra.Command, args []string) error {
	server := http.Server{
		Addr: fmt.Sprintf(":%s", string(c.httpPort)),
		Handler: http.HandlerFunc(func (w http.ResponseWriter, r *http.Request) {
		}),
	}

	log.Println("starting server on port", string(c.httpPort))

	err := server.ListenAndServe()
	if err != nil {
		log.Fatal("Error starting server", err)
		return err
	}

	return nil
}
