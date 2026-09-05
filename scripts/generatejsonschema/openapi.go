package main

import (
	"strings"

	"github.com/hilather/go-lab-syslog/internal/capabilities"
)

func openAPI() obj {
	pathOrder := []string{}
	pathMap := map[string]obj{}
	for _, row := range capabilities.Table() {
		if _, ok := pathMap[row.RESTPath]; !ok {
			pathOrder = append(pathOrder, row.RESTPath)
			pathMap[row.RESTPath] = obj{}
		}
		pathMap[row.RESTPath] = append(pathMap[row.RESTPath], kv{strings.ToLower(row.RESTMethod), openAPIOp(row)})
	}
	paths := obj{}
	for _, p := range pathOrder {
		paths = append(paths, kv{p, pathMap[p]})
	}
	return obj{
		{"openapi", "3.1.0"},
		{"info", obj{
			{"title", "LabSyslog REST API"},
			{"version", "v1"},
			{"description", "Native /v1 adapter over app.Service. Errors are application/problem+json with type https://labsyslog.dev/errors/{code}."},
		}},
		{"security", []any{obj{{"bearerAuth", []any{}}}}},
		{"paths", paths},
		{"components", obj{
			{"securitySchemes", obj{
				{"bearerAuth", obj{
					{"type", "http"},
					{"scheme", "bearer"},
				}},
			}},
			{"schemas", obj{
				{"Problem", obj{
					{"type", "object"},
					{"required", []string{"type", "title", "status", "code"}},
					{"properties", obj{
						{"type", obj{{"type", "string"}, {"examples", []string{"https://labsyslog.dev/errors/validation_failed"}}}},
						{"title", obj{{"type", "string"}}},
						{"status", obj{{"type", "integer"}}},
						{"code", obj{{"type", "string"}}},
						{"detail", obj{{"type", "string"}}},
					}},
				}},
			}},
		}},
	}
}

func openAPIOp(row capabilities.Row) obj {
	op := obj{
		{"operationId", row.ID},
	}
	if row.Scope != "" {
		op = append(op, kv{"description", "scope " + row.Scope})
	}
	switch row.ID {
	case "health.live", "health.ready", "metrics.scrape":
		op = append(op, kv{"security", []any{obj{}}})
	}
	switch row.ID {
	case "change.plan", "change.apply", "state.validate", "messages.wait":
		op = append(op, kv{"requestBody", obj{
			{"required", true},
			{"content", obj{
				{"application/json", obj{{"schema", obj{{"type", "object"}}}}},
			}},
		}})
	}
	responses := obj{}
	switch {
	case row.ID == "messages.wait":
		responses = append(responses,
			kv{"200", obj{{"description", "matched existing or inserted"}}},
			kv{"504", problemResp("wait_timeout")},
			kv{"409", problemResp("store_wiped")},
		)
	case row.ID == "session.create":
		responses = append(responses, kv{"200", obj{{"description", "session cookie and csrf"}}})
	case row.RESTMethod == "DELETE" || row.ID == "messages.clear" || row.ID == "session.delete":
		responses = append(responses, kv{"204", obj{{"description", "no content"}}})
	case row.ID == "metrics.scrape":
		responses = append(responses,
			kv{"200", obj{{"description", "OpenMetrics text when publicPath is true"}}},
			kv{"404", problemResp("not_found")},
		)
	case row.ID == "events.stream":
		responses = append(responses, kv{"200", obj{{"description", "text/event-stream; syslog.received, syslog.deleted, store.wiped; heartbeat 15s"}}})
	case row.ID == "message.raw.get":
		responses = append(responses, kv{"200", obj{{"description", "application/octet-stream"}}})
	default:
		responses = append(responses, kv{"200", obj{{"description", "success"}}})
	}
	responses = append(responses, kv{"default", obj{
		{"description", "application/problem+json"},
		{"content", obj{
			{"application/problem+json", obj{
				{"schema", obj{{"$ref", "#/components/schemas/Problem"}}},
			}},
		}},
	}})
	op = append(op, kv{"responses", responses})
	return op
}

func problemResp(code string) obj {
	return obj{
		{"description", code},
		{"content", obj{
			{"application/problem+json", obj{
				{"schema", obj{{"$ref", "#/components/schemas/Problem"}}},
			}},
		}},
	}
}
