package mcp

import (
	"context"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// RunStdio serves the same registry over stdio. Logs go to stderr (never
// stdout). This is a developer adapter; --token-file is required.
func (s *Server) RunStdio(ctx context.Context) error {
	return s.sdk.Run(ctx, &sdk.StdioTransport{})
}
