package mcp

import (
	"github.com/hilather/go-lab-syslog/internal/app"
	"github.com/hilather/go-lab-syslog/internal/domainerr"
	"github.com/hilather/go-lab-syslog/internal/model"
)

type emptyIn struct{}

type idIn struct {
	ID string `json:"id"`
}

type reasonIn struct {
	Reason string `json:"reason,omitempty"`
}

type exportIn struct {
	Format string `json:"format,omitempty"`
}

type validateIn struct {
	Document *model.Document `json:"document,omitempty"`
}

type changeIn struct {
	ExpectedRevision string          `json:"expectedRevision"`
	IdempotencyKey   string          `json:"idempotencyKey,omitempty"`
	Reason           string          `json:"reason,omitempty"`
	Operations       []app.Operation `json:"operations,omitempty"`
	Candidate        *model.Document `json:"candidate,omitempty"`
}

type messageGetIn struct {
	ID  string `json:"id"`
	Raw bool   `json:"raw,omitempty"`
}

type messagesListIn struct {
	Limit           int    `json:"limit,omitempty"`
	Cursor          string `json:"cursor,omitempty"`
	Facility        string `json:"facility,omitempty"`
	Severity        string `json:"severity,omitempty"`
	SeverityAtLeast string `json:"severityAtLeast,omitempty"`
	AppName         string `json:"appName,omitempty"`
	Hostname        string `json:"hostname,omitempty"`
	MsgID           string `json:"msgID,omitempty"`
	ProcID          string `json:"procID,omitempty"`
	MessageContains string `json:"messageContains,omitempty"`
	Protocol        string `json:"protocol,omitempty"`
	Transport       string `json:"transport,omitempty"`
	SourceCIDR      string `json:"sourceCidr,omitempty"`
	After           string `json:"after,omitempty"`
	Before          string `json:"before,omitempty"`
	Truncated       *bool  `json:"truncated,omitempty"`
	ParseWarning    *bool  `json:"parseWarning,omitempty"`
}

type waitIn struct {
	Timeout string         `json:"timeout,omitempty"`
	Filter  *messageFilter `json:"filter,omitempty"`
}

func (in validateIn) document() (*model.Document, error) {
	if in.Document == nil {
		return nil, domainerr.New(domainerr.ValidationFailed, "document is required")
	}
	return in.Document, nil
}

func (in changeIn) planRequest(actor string) app.PlanRequest {
	return app.PlanRequest{
		ExpectedRevision: in.ExpectedRevision,
		Operations:       in.Operations,
		Candidate:        in.Candidate,
		Reason:           in.Reason,
		Actor:            actor,
	}
}

func (in changeIn) applyRequest(actor string) app.ApplyRequest {
	return app.ApplyRequest{
		ExpectedRevision: in.ExpectedRevision,
		Operations:       in.Operations,
		Candidate:        in.Candidate,
		Reason:           in.Reason,
		Actor:            actor,
		IdempotencyKey:   in.IdempotencyKey,
	}
}

func (in messagesListIn) filter() messageFilter {
	return messageFilter{
		Facility:        in.Facility,
		Severity:        in.Severity,
		SeverityAtLeast: in.SeverityAtLeast,
		AppName:         in.AppName,
		Hostname:        in.Hostname,
		MsgID:           in.MsgID,
		ProcID:          in.ProcID,
		MessageContains: in.MessageContains,
		Protocol:        in.Protocol,
		Transport:       in.Transport,
		SourceCIDR:      in.SourceCIDR,
		After:           in.After,
		Before:          in.Before,
		Truncated:       in.Truncated,
		ParseWarning:    in.ParseWarning,
	}
}
