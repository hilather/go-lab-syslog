package domainerr

// Code is a frozen domain error token from docs/05.
type Code string

// Frozen 1.0 codes. REST type is https://labsyslog.dev/errors/{code}.
const (
	ValidationFailed    Code = "validation_failed"
	UnknownField        Code = "unknown_field"
	ReservedKey         Code = "reserved_key"
	ImmutableField      Code = "immutable_field"
	RevisionMismatch    Code = "revision_mismatch"
	IdempotencyConflict Code = "idempotency_conflict"
	NotFound            Code = "not_found"
	StoreWiped          Code = "store_wiped"
	StoreFull           Code = "store_full"
	WaitTimeout         Code = "wait_timeout"
	Unauthorized        Code = "unauthorized"
	Forbidden           Code = "forbidden"
	OriginNotAllowed    Code = "origin_not_allowed"
	BootstrapInvalid    Code = "bootstrap_invalid"
	PayloadTooLarge     Code = "payload_too_large"
	RateLimited         Code = "rate_limited"
	CursorStale         Code = "cursor_stale"
	TLSUnsupported      Code = "tls_unsupported"
	Unparseable         Code = "unparseable"
	ReceiveOnly         Code = "receive_only"
)

// Info is catalog metadata for one code.
type Info struct {
	Code   Code
	Title  string
	Status int
}

var catalog = map[Code]Info{
	ValidationFailed:    {ValidationFailed, "validation failed", 400},
	UnknownField:        {UnknownField, "unknown field", 400},
	ReservedKey:         {ReservedKey, "reserved key", 400},
	ImmutableField:      {ImmutableField, "immutable field", 400},
	RevisionMismatch:    {RevisionMismatch, "revision mismatch", 409},
	IdempotencyConflict: {IdempotencyConflict, "idempotency conflict", 409},
	NotFound:            {NotFound, "not found", 404},
	StoreWiped:          {StoreWiped, "store wiped", 409},
	StoreFull:           {StoreFull, "store full", 503},
	WaitTimeout:         {WaitTimeout, "wait timeout", 504},
	Unauthorized:        {Unauthorized, "unauthorized", 401},
	Forbidden:           {Forbidden, "forbidden", 403},
	OriginNotAllowed:    {OriginNotAllowed, "origin not allowed", 403},
	BootstrapInvalid:    {BootstrapInvalid, "bootstrap invalid", 400},
	PayloadTooLarge:     {PayloadTooLarge, "payload too large", 413},
	RateLimited:         {RateLimited, "rate limited", 429},
	CursorStale:         {CursorStale, "cursor stale", 400},
	TLSUnsupported:      {TLSUnsupported, "tls unsupported", 400},
	Unparseable:         {Unparseable, "unparseable", 400},
	ReceiveOnly:         {ReceiveOnly, "receive only", 403},
}

// Lookup returns catalog metadata. Unknown codes get a generic 500 entry.
func Lookup(code Code) Info {
	if info, ok := catalog[code]; ok {
		return info
	}
	return Info{Code: code, Title: string(code), Status: 500}
}

// TypeURI is the REST problem+json type for code.
func TypeURI(code Code) string {
	return "https://labsyslog.dev/errors/" + string(code)
}

// URN is the internal catalog identifier.
func URN(code Code) string {
	return "urn:labsyslog:error:" + string(code)
}

// Codes returns the frozen catalog in a stable order.
func Codes() []Code {
	return []Code{
		ValidationFailed,
		UnknownField,
		ReservedKey,
		ImmutableField,
		RevisionMismatch,
		IdempotencyConflict,
		NotFound,
		StoreWiped,
		StoreFull,
		WaitTimeout,
		Unauthorized,
		Forbidden,
		OriginNotAllowed,
		BootstrapInvalid,
		PayloadTooLarge,
		RateLimited,
		CursorStale,
		TLSUnsupported,
		Unparseable,
		ReceiveOnly,
	}
}
