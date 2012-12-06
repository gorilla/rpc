// Copyright 2009 The Go Authors. All rights reserved.
// Copyright 2012 The Gorilla Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package endpoints

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/gorilla/rpc"
)

// ResponseRecorder is an implementation of http.ResponseWriter that
// records its mutations for later inspection in tests.
type ResponseRecorder struct {
	Code      int           // the HTTP response code from WriteHeader
	HeaderMap http.Header   // the HTTP response headers
	Body      *bytes.Buffer // if non-nil, the bytes.Buffer to append written data to
	Flushed   bool
}

// NewRecorder returns an initialized ResponseRecorder.
func NewRecorder() *ResponseRecorder {
	return &ResponseRecorder{
		HeaderMap: make(http.Header),
		Body:      new(bytes.Buffer),
	}
}

// DefaultRemoteAddr is the default remote address to return in RemoteAddr if
// an explicit DefaultRemoteAddr isn't set on ResponseRecorder.
const DefaultRemoteAddr = "1.2.3.4"

// Header returns the response headers.
func (rw *ResponseRecorder) Header() http.Header {
	return rw.HeaderMap
}

// Write always succeeds and writes to rw.Body, if not nil.
func (rw *ResponseRecorder) Write(buf []byte) (int, error) {
	if rw.Body != nil {
		rw.Body.Write(buf)
	}
	if rw.Code == 0 {
		rw.Code = http.StatusOK
	}
	return len(buf), nil
}

// WriteHeader sets rw.Code.
func (rw *ResponseRecorder) WriteHeader(code int) {
	rw.Code = code
}

// Flush sets rw.Flushed to true.
func (rw *ResponseRecorder) Flush() {
	rw.Flushed = true
}

// ----------------------------------------------------------------------------

var ErrResponseError = errors.New("response error")

type Service1Request struct {
	A int
	B int
}

type Service1BadRequest struct {
}

type Service1Response struct {
	Result int
	ErrorMessage string `json:"error_message"`
}

type Service1 struct {
}

func (t *Service1) Multiply(r *http.Request, req *Service1Request, res *Service1Response) error {
	res.Result = req.A * req.B
	return nil
}

func (t *Service1) ResponseError(r *http.Request, req *Service1Request, res *Service1Response) error {
	return ErrResponseError
}

func execute(t *testing.T, s *rpc.Server, method string, req, res interface{}) (int, error) {
	if !s.HasMethod(method) {
		t.Fatal("Expected to be registered:", method)
	}

	buf, _ := json.Marshal(req)
	body := bytes.NewBuffer(buf)
	r, _ := http.NewRequest("POST", "http://localhost:8080/"+method, body)
	r.Header.Set("Content-Type", "application/json")

	w := NewRecorder()
	s.ServeHTTP(w, r)

	err := json.NewDecoder(w.Body).Decode(res)
	return w.Code, err
}

func TestService(t *testing.T) {
	s := rpc.NewServer()
	s.RegisterCodec(NewCodec(), "application/json")
	s.RegisterService(new(Service1), "")

	var res Service1Response
	if _, err := execute(t, s, "Service1.Multiply", &Service1Request{4, 2}, &res); err != nil {
		t.Error("Expected err to be nil, but got:", err)
	}
	if res.Result != 8 {
		t.Errorf("Wrong response: %v.", res.Result)
	}
	if res.ErrorMessage != "" {
		t.Errorf("Expected error_message to be empty, but got:", res.ErrorMessage)
	}

	if code, err := execute(t, s, "Service1.ResponseError", &Service1Request{4, 2}, &res); err != nil || code != 500 {
		t.Errorf("Expected code to be 500 and error to be nil, but got", code, err)
	}
	if res.ErrorMessage == "" {
		t.Errorf("Expected error_message to be %q, but got %q", ErrResponseError, res.ErrorMessage)
	}

	if code, _ := execute(t, s, "Service1.Multiply", nil, &res); code != 400 {
		t.Errorf("Expected http response code 400, but got %v", code)
	}
}
