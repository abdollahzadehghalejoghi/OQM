package cli

import (
	"fmt"

	"github.com/abdollahzadehghalejoghi/oqm/internal/storage"
	"github.com/abdollahzadehghalejoghi/oqm/internal/webui"
	"github.com/abdollahzadehghalejoghi/oqm/pkg/config"
	"github.com/spf13/cobra"
)

// NewWebCommand creates the web command
func NewWebCommand() *cobra.Command {
	var port int

	cmd := &cobra.Command{
		Use:   "web",
		Short: "Start the web UI server",
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := storage.NewStorage(config.GetDataFilePath())
			if err != nil {
				return err
			}

			if port == 0 {
				port = store.GetConfig().WebPort
			}

			server := webui.NewServer(store)
			fmt.Printf("Starting web UI on http://0.0.0.0:%d\n", port)
			return server.Start(port)
		},
	}

	cmd.Flags().IntVar(&port, "port", 0, "Port to listen on (default from config)")
	return cmd
}
