package fetch

import (
	"bytes"
	"io"
	"net/http"
	"net/url"
)

type FetchRequest struct {
	client  *http.Client
	method  string
	url     string
	headers *http.Header
	body    []byte
}

// Setters

func (f *FetchRequest) Client(client *http.Client) *FetchRequest {
	f.client = client
	return f
}

func (f *FetchRequest) Method(method string) *FetchRequest {
	f.method = method
	return f
}

func (f *FetchRequest) Url(urlSegments ...string) *FetchRequest {
	if len(urlSegments) >= 0 {
		f.url, _ = url.JoinPath(urlSegments[0], urlSegments[1:]...)
	} else {
		f.url = urlSegments[0]
	}
	return f
}

func (f *FetchRequest) Headers(headers *http.Header) *FetchRequest {
	f.headers = headers
	return f
}

func (f *FetchRequest) Header(key, value string) *FetchRequest {
	if f.headers == nil {
		headers := http.Header{}
		f.Headers(&headers)
	}
	f.headers.Set(key, value)
	return f
}

func (f *FetchRequest) Body(body []byte) *FetchRequest {
	f.body = body
	return f
}

func (f *FetchRequest) BodyStr(body string) *FetchRequest {
	return f.Body([]byte(body))
}

func (f *FetchRequest) Do() (result *FetchResult) {
	result = &FetchResult{}
	var client *http.Client
	if f.client != nil {
		client = f.client
	} else {
		client = &http.Client{}
	}
	var body io.Reader
	if f.body != nil {
		body = bytes.NewBuffer(f.body)
	}
	req, _ := http.NewRequest(f.method, f.url, body)
	if f.headers != nil {
		req.Header = *f.headers
	}
	if f.headers == nil || f.headers.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}
	result.Response, result.err = client.Do(req)
	if result.err != nil {
		return
	}
	defer result.Response.Body.Close()
	buf := new(bytes.Buffer)
	_, result.err = buf.ReadFrom(result.Response.Body)
	if result.err != nil {
		return
	}
	result.Body = buf.Bytes()
	return
}

func (f *FetchRequest) Get(urlSegments ...string) *FetchRequest {
	return f.Method(http.MethodGet).Url(urlSegments...)
}

func (f *FetchRequest) Patch(urlSegments ...string) *FetchRequest {
	return f.Method(http.MethodPatch).Url(urlSegments...)
}

func (f *FetchRequest) Post(urlSegments ...string) *FetchRequest {
	return f.Method(http.MethodPost).Url(urlSegments...)
}

func (f *FetchRequest) Delete(urlSegments ...string) *FetchRequest {
	return f.Method(http.MethodDelete).Url(urlSegments...)
}
