package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

type serveCommand struct {
	cmd      *cobra.Command
	httpPort int
	subdomain string
}

func ServeCommand() *serveCommand {
	serveCommand := &serveCommand{}
	serveCommand.cmd = &cobra.Command{
		Use:   "serve",
		Short: "Run the server",
		RunE:  serveCommand.run,
	}

	serveCommand.cmd.Flags().IntVar(&serveCommand.httpPort, "port", 8080, "Port to start server tunnel on")
	serveCommand.cmd.Flags().StringVar(&serveCommand.subdomain, "subdomain", GenerateRandomSubdomain(10), "Custom subdomain to serve on")

	return serveCommand
}

func (c *serveCommand) run(cmd *cobra.Command, args []string) error {
	fmt.Println(c.httpPort, c.subdomain)
	return nil
}
