// Copyright 2009 The Go Authors. All rights reserved.
// Copyright 2012 The Gorilla Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package json

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
	M string `json:"method"`
}

type Service1Response struct {
	Result int
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

func execute(t *testing.T, s *rpc.Server, method string, req, res interface{}) error {
	if !s.HasMethod(method) {
		t.Fatal("Expected to be registered:", method)
	}

	buf, _ := EncodeClientRequest(method, req)
	body := bytes.NewBuffer(buf)
	r, _ := http.NewRequest("POST", "http://localhost:8080/", body)
	r.Header.Set("Content-Type", "application/json")

	w := NewRecorder()
	s.ServeHTTP(w, r)

	return DecodeClientResponse(w.Body, res)
}

func executeRaw(t *testing.T, s *rpc.Server, req interface{}, res interface{}) int {
	j, _ := json.Marshal(req)
	r, _ := http.NewRequest("POST", "http://localhost:8080/", bytes.NewBuffer(j))
	r.Header.Set("Content-Type", "application/json")

	w := NewRecorder()
	s.ServeHTTP(w, r)

	return w.Code
}

func TestService(t *testing.T) {
	s := rpc.NewServer()
	s.RegisterCodec(NewCodec(), "application/json")
	s.RegisterService(new(Service1), "")

	var res Service1Response
	if err := execute(t, s, "Service1.Multiply", &Service1Request{4, 2}, &res); err != nil {
		t.Error("Expected err to be nil, but got:", err)
	}
	if res.Result != 8 {
		t.Errorf("Wrong response: %v.", res.Result)
	}

	if err := execute(t, s, "Service1.ResponseError", &Service1Request{4, 2}, &res); err == nil {
		t.Errorf("Expected to get %q, but got nil", ErrResponseError)
	} else if err.Error() != ErrResponseError.Error() {
		t.Errorf("Expected to get %q, but got %q", ErrResponseError, err)
	}

	if code := executeRaw(t, s, &Service1BadRequest{"Service1.Multiply"}, &res); code != 400 {
		t.Errorf("Expected http response code 400, but got %v", code)
	}
}
