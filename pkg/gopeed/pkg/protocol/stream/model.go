package stream

type ReqExtra struct {
	Method string            `json:"method"`
	Header map[string]string `json:"header"`
	Body   string            `json:"body"`
}

type OptsExtra struct {
}

// Stats for download
type Stats struct {
	Downloaded int64 `json:"downloaded"`
}
