// Copyright 2009 The Go Authors. All rights reserved.
// Copyright 2012 The Gorilla Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rpc

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"
	"testing"
)

type Service1Request struct {
	A int
	B int
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

type Service2 struct {
}

func TestRegisterService(t *testing.T) {
	var err error
	s := NewServer()
	service1 := new(Service1)
	service2 := new(Service2)

	// Inferred name.
	err = s.RegisterService(service1, "")
	if err != nil || !s.HasMethod("Service1.Multiply") {
		t.Errorf("Expected to be registered: Service1.Multiply")
	}
	// Provided name.
	err = s.RegisterService(service1, "Foo")
	if err != nil || !s.HasMethod("Foo.Multiply") {
		t.Errorf("Expected to be registered: Foo.Multiply")
	}
	// No methods.
	err = s.RegisterService(service2, "")
	if err == nil {
		t.Errorf("Expected error on service2")
	}
}

// MockCodec decodes to Service1.Multiply.
type MockCodec struct {
	A, B int
}

func (c MockCodec) NewRequest(*http.Request) CodecRequest {
	return MockCodecRequest(c)
}

type MockCodecRequest struct {
	A, B int
}

func (r MockCodecRequest) Method() (string, error) {
	return "Service1.Multiply", nil
}

func (r MockCodecRequest) ReadRequest(args interface{}) error {
	req := args.(*Service1Request)
	req.A, req.B = r.A, r.B
	return nil
}

func (r MockCodecRequest) WriteResponse(w http.ResponseWriter, reply interface{}) {
	res := reply.(*Service1Response)
	if _, err := w.Write([]byte(strconv.Itoa(res.Result))); err != nil {
		log.Fatal(err)
	}
}

func (r MockCodecRequest) WriteError(w http.ResponseWriter, status int, err error) {
	w.WriteHeader(status)
	_, er := w.Write([]byte(err.Error()))
	if er != nil {
		log.Fatal(er)
	}
}

type MockCodecJson struct {
}

func (c MockCodecJson) NewRequest(r *http.Request) CodecRequest {
	if r.Body == nil {
		return MockCodecRequest{}
	}

	inp := new(Service1Request)
	b, err := io.ReadAll(r.Body)
	if err != nil {
		return MockCodecRequest{}
	}
	r.Body.Close()

	if err := json.Unmarshal(b, inp); err != nil {
		return MockCodecRequest{}
	}

	r.Body = io.NopCloser(bytes.NewBuffer(b))

	return MockCodecRequest{inp.A, inp.B}
}

type MockResponseWriter struct {
	header http.Header
	Status int
	Body   string
}

func NewMockResponseWriter() *MockResponseWriter {
	header := make(http.Header)
	return &MockResponseWriter{header: header}
}

func (w *MockResponseWriter) Header() http.Header {
	return w.header
}

func (w *MockResponseWriter) Write(p []byte) (int, error) {
	w.Body = string(p)
	if w.Status == 0 {
		w.Status = 200
	}
	return len(p), nil
}

func (w *MockResponseWriter) WriteHeader(status int) {
	w.Status = status
}

func TestServeHTTP(t *testing.T) {
	const (
		A = 2
		B = 3
	)
	expected := A * B

	s := NewServer()
	if err := s.RegisterService(new(Service1), ""); err != nil {
		t.Fatal(err)
	}
	s.RegisterCodec(MockCodec{A, B}, "mock")
	r, err := http.NewRequest("POST", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	r.Header.Set("Content-Type", "mock; dummy")
	w := NewMockResponseWriter()
	s.ServeHTTP(w, r)
	if w.Status != 200 {
		t.Errorf("Status was %d, should be 200.", w.Status)
	}
	if w.Body != strconv.Itoa(expected) {
		t.Errorf("Response body was %s, should be %s.", w.Body, strconv.Itoa(expected))
	}

	// Test wrong Content-Type
	r.Header.Set("Content-Type", "invalid")
	w = NewMockResponseWriter()
	s.ServeHTTP(w, r)
	if w.Status != 415 {
		t.Errorf("Status was %d, should be 415.", w.Status)
	}
	if w.Body != "rpc: unrecognized Content-Type: invalid" {
		t.Errorf("Wrong response body.")
	}

	// Test omitted Content-Type; codec should default to the sole registered one.
	r.Header.Del("Content-Type")
	w = NewMockResponseWriter()
	s.ServeHTTP(w, r)
	if w.Status != 200 {
		t.Errorf("Status was %d, should be 200.", w.Status)
	}
	if w.Body != strconv.Itoa(expected) {
		t.Errorf("Response body was %s, should be %s.", w.Body, strconv.Itoa(expected))
	}
}

func TestInterception(t *testing.T) {
	const (
		A = 2
		B = 3
	)
	expected := A * B

	r2, err := http.NewRequest("POST", "mocked/request", nil)
	if err != nil {
		t.Fatal(err)
	}

	s := NewServer()
	if err = s.RegisterService(new(Service1), ""); err != nil {
		t.Fatal(err)
	}
	s.RegisterCodec(MockCodec{A, B}, "mock")
	s.RegisterInterceptFunc(func(i *RequestInfo) *http.Request {
		return r2
	})
	s.RegisterValidateRequestFunc(func(info *RequestInfo, v interface{}) error { return nil })
	s.RegisterAfterFunc(func(i *RequestInfo) {
		if i.Request != r2 {
			t.Errorf("Request was %v, should be %v.", i.Request, r2)
		}
	})

	r, err := http.NewRequest("POST", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	r.Header.Set("Content-Type", "mock; dummy")
	w := NewMockResponseWriter()
	s.ServeHTTP(w, r)
	if w.Status != 200 {
		t.Errorf("Status was %d, should be 200.", w.Status)
	}
	if w.Body != strconv.Itoa(expected) {
		t.Errorf("Response body was %s, should be %s.", w.Body, strconv.Itoa(expected))
	}
}

func TestInterceptionWithChange(t *testing.T) {
	const (
		A = 2
		B = 3
		C = 5
	)
	expectedBeforeChange := A * B
	expectedAfterChange := A * C

	r2, err := http.NewRequest("POST", "mocked/request", bytes.NewBuffer([]byte(`{"A": 2, "B":5}`)))
	if err != nil {
		t.Fatal(err)
	}

	s := NewServer()
	s.RegisterService(new(Service1), "")
	s.RegisterCodec(MockCodecJson{}, "mock")
	s.RegisterInterceptFunc(func(i *RequestInfo) *http.Request {
		return r2
	})

	r, err := http.NewRequest("POST", "", bytes.NewBuffer([]byte(`{A: 2, B:3}`)))
	if err != nil {
		t.Fatal(err)
	}
	r.Header.Set("Content-Type", "mock; dummy")
	w := NewMockResponseWriter()
	s.ServeHTTP(w, r)
	if w.Status != 200 {
		t.Errorf("Status was %d, should be 200.", w.Status)
	}

	if w.Body != strconv.Itoa(expectedBeforeChange) && w.Body == strconv.Itoa(expectedAfterChange) {
		return
	}

	t.Errorf("Response body was %s, should be %s.", w.Body, strconv.Itoa(expectedAfterChange))
}

func TestBeforeFunc(t *testing.T) {
	const (
		A = 2
		B = 3
		C = 5
	)
	expectedBeforeChange := A * B
	expectedAfterChange := A * C

	s := NewServer()
	s.RegisterService(new(Service1), "")
	s.RegisterCodec(MockCodecJson{}, "mock")
	s.RegisterBeforeFunc(func(i *RequestInfo) {
		r := i.Request

		inp := new(Service1Request)
		err := json.NewDecoder(r.Body).Decode(inp)
		if err != nil {
			t.Error(err)
			t.Fail()
		}

		inp.B = C

		b, err := json.Marshal(inp)
		if err != nil {
			t.Error(err)
			t.Fail()
		}

		r.Body = io.NopCloser(bytes.NewBuffer(b))
		i.Request = r
	})

	r, err := http.NewRequest("POST", "", bytes.NewBuffer([]byte(`{"A":2, "B":10}`)))
	if err != nil {
		t.Fatal(err)
	}
	r.Header.Set("Content-Type", "mock; dummy")
	w := NewMockResponseWriter()
	s.ServeHTTP(w, r)
	if w.Status != 200 {
		t.Errorf("Status was %d, should be 200.", w.Status)
	}

	if w.Body != strconv.Itoa(expectedBeforeChange) && w.Body == strconv.Itoa(expectedAfterChange) {
		return
	}

	t.Errorf("Response body was %s, should be %s.", w.Body, strconv.Itoa(expectedAfterChange))
}

func TestValidationSuccessful(t *testing.T) {
	const (
		A = 2
		B = 3

		expected = A * B
	)

	validate := func(info *RequestInfo, v interface{}) error { return nil }

	s := NewServer()
	if err := s.RegisterService(new(Service1), ""); err != nil {
		t.Fatal(err)
	}

	s.RegisterCodec(MockCodec{A, B}, "mock")
	s.RegisterValidateRequestFunc(validate)

	r, err := http.NewRequest("POST", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	r.Header.Set("Content-Type", "mock; dummy")
	w := NewMockResponseWriter()
	s.ServeHTTP(w, r)
	if w.Status != 200 {
		t.Errorf("Status was %d, should be 200.", w.Status)
	}
	if w.Body != strconv.Itoa(expected) {
		t.Errorf("Response body was %s, should be %s.", w.Body, strconv.Itoa(expected))
	}
}

func TestValidationFails(t *testing.T) {
	const expected = "this instance only supports zero values"

	validate := func(_ *RequestInfo, v interface{}) error {
		req := v.(*Service1Request)
		if req.A != 0 || req.B != 0 {
			return errors.New(expected)
		}
		return nil
	}

	s := NewServer()
	if err := s.RegisterService(new(Service1), ""); err != nil {
		t.Fatal(err)
	}

	s.RegisterCodec(MockCodec{1, 2}, "mock")
	s.RegisterValidateRequestFunc(validate)

	r, err := http.NewRequest("POST", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	r.Header.Set("Content-Type", "mock; dummy")
	w := NewMockResponseWriter()
	s.ServeHTTP(w, r)
	if w.Status != 400 {
		t.Errorf("Status was %d, should be 200.", w.Status)
	}
	if w.Body != expected {
		t.Errorf("Response body was %s, should be %s.", w.Body, expected)
	}
}
