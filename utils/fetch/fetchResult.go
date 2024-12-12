package fetch

import (
	"encoding/json"
	"net/http"

	"api_core/message"
)

type FetchResult struct {
	Response *http.Response
	Body     []byte
	err      error
}

func (f *FetchResult) Ok(statusCodes ...int) bool {
	if f.err != nil {
		return false
	}
	if len(statusCodes) > 0 {
		return f.Matches(statusCodes...)
	} else {
		return f.Response.StatusCode < 400
	}
}

func (f *FetchResult) Message() message.Message {
	return &message.Msg{Status: f.Response.StatusCode, Message: f.Error()}
}

func (f *FetchResult) Error() string {
	if f.err != nil {
		return f.err.Error()
	}
	return "status: " + f.Response.Status + " " + string(f.Body)
}

func (f *FetchResult) Matches(statusCodes ...int) bool {
	if len(statusCodes) == 0 {
		return true
	}
	for _, code := range statusCodes {
		if code == f.Response.StatusCode {
			return true
		}
	}
	return false
}

func (f *FetchResult) Json(v any) error {
	return json.Unmarshal(f.Body, v)
}
