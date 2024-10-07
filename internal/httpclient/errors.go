package httpclient

type ErrorLog struct {
	OK    bool          `json:"ok"`
	Error string        `json:"error,omitempty"`
	Data  []interface{} `json:"data,omitempty"`
}

func (h *HTTPClient) ok(data ...interface{}) *ErrorLog {
	return &ErrorLog{
		OK:   true,
		Data: data,
	}
}
func (h *HTTPClient) error(data ...interface{}) *ErrorLog {
	return &ErrorLog{
		OK:   false,
		Data: data,
	}
}
