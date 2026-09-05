package mcp

import (
	"encoding/json"
	"net/http"

	"github.com/hilather/go-lab-syslog/internal/auth"
	"github.com/hilather/go-lab-syslog/internal/domainerr"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	rpcInvalidParams   int64 = jsonrpc.CodeInvalidParams
	rpcInternal        int64 = jsonrpc.CodeInternalError
	rpcApplication     int64 = -32000
	rpcUnauthenticated int64 = -32001
	rpcForbidden       int64 = -32003
	rpcNotFound        int64 = -32004
	rpcRateLimited     int64 = -32005
	rpcConflict        int64 = -32009
	rpcTimeout         int64 = -32010
	rpcPayloadTooLarge int64 = -32013
)

func asDomain(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := domainerr.As(err); ok {
		return err
	}
	return domainerr.New(domainerr.ValidationFailed, err.Error())
}

func errorData(err error) map[string]any {
	de, ok := domainerr.As(asDomain(err))
	if !ok {
		return map[string]any{"code": string(domainerr.ValidationFailed), "detail": err.Error()}
	}
	out := map[string]any{"code": string(de.Code)}
	if de.Detail != "" {
		out["detail"] = de.Detail
	}
	return out
}

func rpcCode(err error) int64 {
	de, ok := domainerr.As(asDomain(err))
	if !ok {
		return rpcInternal
	}
	switch de.Code {
	case domainerr.Unauthorized:
		return rpcUnauthenticated
	case domainerr.Forbidden, domainerr.OriginNotAllowed:
		return rpcForbidden
	case domainerr.NotFound:
		return rpcNotFound
	case domainerr.RateLimited:
		return rpcRateLimited
	case domainerr.WaitTimeout:
		return rpcTimeout
	case domainerr.RevisionMismatch, domainerr.IdempotencyConflict, domainerr.StoreWiped:
		return rpcConflict
	case domainerr.PayloadTooLarge:
		return rpcPayloadTooLarge
	case domainerr.ValidationFailed, domainerr.UnknownField, domainerr.ReservedKey,
		domainerr.ImmutableField, domainerr.CursorStale, domainerr.TLSUnsupported,
		domainerr.Unparseable, domainerr.BootstrapInvalid:
		return rpcInvalidParams
	default:
		return rpcApplication
	}
}

func rpcError(err error) error {
	mapped := asDomain(err)
	data, mErr := json.Marshal(errorData(mapped))
	if mErr != nil {
		data = []byte(`{"code":"validation_failed","detail":"internal error"}`)
	}
	de, _ := domainerr.As(mapped)
	msg := mapped.Error()
	if de != nil && de.Detail != "" {
		msg = de.Detail
	}
	return &jsonrpc.Error{
		Code:    rpcCode(mapped),
		Message: msg,
		Data:    data,
	}
}

func toolErrorResult(err error) *sdk.CallToolResult {
	mapped := asDomain(err)
	return &sdk.CallToolResult{
		IsError:           true,
		StructuredContent: errorData(mapped),
		Content: []sdk.Content{
			&sdk.TextContent{Text: mapped.Error()},
		},
	}
}

func writeRPC(w http.ResponseWriter, status int, err error) {
	mapped := asDomain(err)
	if de, ok := domainerr.As(mapped); ok && de.Code == domainerr.Unauthorized {
		w.Header().Set("WWW-Authenticate", auth.WWWAuthenticate())
	}
	body, mErr := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      nil,
		"error": map[string]any{
			"code":    rpcCode(mapped),
			"message": mapped.Error(),
			"data":    errorData(mapped),
		},
	})
	if mErr != nil {
		http.Error(w, `{"jsonrpc":"2.0","id":null,"error":{"code":-32603,"message":"internal error"}}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}
