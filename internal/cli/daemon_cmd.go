package cli

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/abdollahzadehghalejoghi/oqm/internal/daemon"
	"github.com/abdollahzadehghalejoghi/oqm/internal/storage"
	"github.com/abdollahzadehghalejoghi/oqm/pkg/config"
	"github.com/abdollahzadehghalejoghi/oqm/pkg/logger"
	"github.com/spf13/cobra"
)

// NewDaemonRunCommand creates the daemon run command
func NewDaemonRunCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "run",
		Short: "Run the daemon (foreground mode)",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Initialize logger
			cfg := storage.DefaultConfig()
			if err := logger.Initialize(cfg.LogFile); err != nil {
				return fmt.Errorf("failed to initialize logger: %w", err)
			}

			// Load storage
			store, err := storage.NewStorage(config.GetDataFilePath())
			if err != nil {
				return fmt.Errorf("failed to load storage: %w", err)
			}

			// Create daemon
			d, err := daemon.New(store)
			if err != nil {
				return fmt.Errorf("failed to create daemon: %w", err)
			}

			// Start daemon
			if err := d.Start(); err != nil {
				return fmt.Errorf("failed to start daemon: %w", err)
			}

			// Setup signal handling for graceful shutdown
			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

			// Wait for signal
			sig := <-sigCh
			logger.Info("Received signal: %v", sig)

			// Stop daemon
			d.Stop()

			logger.Info("Daemon stopped")
			return nil
		},
	}
}
