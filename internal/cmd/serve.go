package cmd

import (
	"github.com/ghousemohamed/simple-tunnel/internal/client"
	"github.com/spf13/cobra"
)

type serveCommand struct {
	cmd      *cobra.Command
	httpPort string
	subdomain string
	serverAddr string
}

func ServeCommand() *serveCommand {
	serveCommand := &serveCommand{}
	serveCommand.cmd = &cobra.Command{
		Use:   "serve",
		Short: "Run the tunnel server",
		RunE:  serveCommand.run,
	}

	serveCommand.cmd.Flags().StringVar(&serveCommand.httpPort, "port", "8080", "Port to start server tunnel on")
	serveCommand.cmd.Flags().StringVar(&serveCommand.subdomain, "subdomain", GenerateRandomSubdomain(10), "Custom subdomain to serve on")
	serveCommand.cmd.Flags().StringVar(&serveCommand.serverAddr, "server", "simpletunnel.me:80", "Server through which tunnels are routed")

	return serveCommand
}

func (c *serveCommand) run(cmd *cobra.Command, args []string) error {
	client.NewClient(c.httpPort, c.serverAddr, c.subdomain).StartClient()
	return nil
}
