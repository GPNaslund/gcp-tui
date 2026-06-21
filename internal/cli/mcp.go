package cli

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/gpnaslund/gcp-tui/internal/mcpserver"
)

func mcpCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "mcp",
		Short: "Run the Model Context Protocol server (stdio) for agent access",
		Long: "mcp serves gcp-tui's read surface and gated writes to an MCP client over\n" +
			"stdio. It is headless by construction: with no interactive terminal the write\n" +
			"gate refuses every write to a protected (confirm=true) environment, so an agent\n" +
			"can read freely but can never mutate prod.",
		Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			return mcpserver.Run(ctx)
		},
	}
}
