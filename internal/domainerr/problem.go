package domainerr

// Problem is the REST application/problem+json envelope (docs/05, C20).
type Problem struct {
	Type   string `json:"type"`
	Title  string `json:"title"`
	Status int    `json:"status"`
	Code   Code   `json:"code"`
	Detail string `json:"detail,omitempty"`
}

// ProblemOf maps err to a problem+json object. Domain errors use the catalog;
// unknown errors become 400 validation_failed so adapters stay fail-closed.
func ProblemOf(err error) Problem {
	if err == nil {
		return Problem{}
	}
	e, ok := As(err)
	if !ok {
		info := Lookup(ValidationFailed)
		return Problem{
			Type:   TypeURI(ValidationFailed),
			Title:  info.Title,
			Status: info.Status,
			Code:   ValidationFailed,
			Detail: err.Error(),
		}
	}
	info := Lookup(e.Code)
	return Problem{
		Type:   TypeURI(e.Code),
		Title:  info.Title,
		Status: info.Status,
		Code:   e.Code,
		Detail: e.Detail,
	}
}
