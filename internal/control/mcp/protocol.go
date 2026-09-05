package mcp

import (
	"context"
	"net/http"
	"strings"

	"github.com/hilather/go-lab-syslog/internal/domainerr"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func validateProtocolVersion(r *http.Request) error {
	ver := strings.TrimSpace(r.Header.Get(headerProtocolVersion))
	if ver == "" {
		return domainerr.New(domainerr.ValidationFailed, "MCP-Protocol-Version is required; only "+ProtocolVersion+" is supported")
	}
	if ver != ProtocolVersion {
		return domainerr.Newf(domainerr.ValidationFailed, "unsupported MCP protocol version %s; only %s is supported", ver, ProtocolVersion)
	}
	return nil
}

func (s *Server) pinProtocolMiddleware(next sdk.MethodHandler) sdk.MethodHandler {
	return func(ctx context.Context, method string, req sdk.Request) (sdk.Result, error) {
		if !s.allowLegacy() {
			if sr, ok := req.(interface{ ProtocolVersion() string }); ok {
				if v := sr.ProtocolVersion(); v != "" && v != ProtocolVersion {
					return nil, rpcError(domainerr.Newf(domainerr.ValidationFailed, "unsupported MCP protocol version %s; only %s is supported", v, ProtocolVersion))
				}
			}
		}
		res, err := next(ctx, method, req)
		if err != nil {
			return nil, err
		}
		if !s.allowLegacy() {
			if dr, ok := res.(*sdk.DiscoverResult); ok && dr != nil {
				dr.SupportedVersions = []string{ProtocolVersion}
			}
		}
		return res, nil
	}
}
