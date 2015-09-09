// Copyright 2009 The Go Authors. All rights reserved.
// Copyright 2012 The Gorilla Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package json2

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/rpc/v2"
)

var null = json.RawMessage([]byte("null"))
var Version = "2.0"

// ----------------------------------------------------------------------------
// Request and Response
// ----------------------------------------------------------------------------

// ServerRequest represents a JSON-RPC request received by the server.
type ServerRequest struct {
	// JSON-RPC protocol.
	Version string `json:"jsonrpc"`

	// A String containing the name of the method to be invoked.
	Method string `json:"method"`

	// A Structured value to pass as arguments to the method.
	Params *json.RawMessage `json:"params"`

	// The request id. MUST be a string, number or null.
	// Our implementation will not do type checking for id.
	// It will be copied as it is.
	Id *json.RawMessage `json:"id"`
}

// ServerResponse represents a JSON-RPC response returned by the server.
type ServerResponse struct {
	// JSON-RPC protocol.
	Version string `json:"jsonrpc"`

	// The Object that was returned by the invoked method. This must be null
	// in case there was an error invoking the method.
	// As per spec the member will be omitted if there was an error.
	Result interface{} `json:"result,omitempty"`

	// An Error object if there was an error invoking the method. It must be
	// null if there was no error.
	// As per spec the member will be omitted if there was no error.
	Error *Error `json:"error,omitempty"`

	// This must be the same id as the request it is responding to.
	Id *json.RawMessage `json:"id"`
}

// ----------------------------------------------------------------------------
// Codec
// ----------------------------------------------------------------------------

// NewcustomCodec returns a new JSON Codec based on passed encoder selector.
func NewCustomCodec(encSel rpc.EncoderSelector) *Codec {
	return &Codec{encSel: encSel}
}

// NewCodec returns a new JSON Codec.
func NewCodec() *Codec {
	return NewCustomCodec(rpc.DefaultEncoderSelector)
}

// Codec creates a CodecRequest to process each request.
type Codec struct {
	encSel rpc.EncoderSelector
}

// NewRequest returns a CodecRequest.
func (c *Codec) NewRequest(r *http.Request) rpc.CodecRequest {
	return newCodecRequest(r, c.encSel.Select(r))
}

// ----------------------------------------------------------------------------
// CodecRequest
// ----------------------------------------------------------------------------

// newCodecRequest returns a new CodecRequest.
func newCodecRequest(r *http.Request, encoder rpc.Encoder) rpc.CodecRequest {
	// Decode the request body and check if RPC method is valid.
	req := new(ServerRequest)
	err := json.NewDecoder(r.Body).Decode(req)
	if err != nil {
		err = &Error{
			Code:    E_PARSE,
			Message: err.Error(),
			Data:    req,
		}
	}
	if req.Version != Version {
		err = &Error{
			Code:    E_INVALID_REQ,
			Message: "jsonrpc must be " + Version,
			Data:    req,
		}
	}
	r.Body.Close()
	return &CodecRequest{Request: req, Err: err, encoder: encoder}
}

// CodecRequest decodes and encodes a single request.
type CodecRequest struct {
	// The request being handled
	Request *ServerRequest

	// If the request couldn't be decoded, or was the wrong version, this will
	// be non-nil
	Err     error
	encoder rpc.Encoder
}

// Method returns the RPC method for the current request.
//
// The method uses a dotted notation as in "Service.Method".
func (c *CodecRequest) Method() (string, error) {
	if c.Err == nil {
		return c.Request.Method, nil
	}
	return "", c.Err
}

// ReadRequest fills the request object for the RPC method.
func (c *CodecRequest) ReadRequest(args interface{}) error {
	if c.Err == nil && c.Request.Params != nil {
		// Note: if c.Request.Params is nil it's not an error, it's an optional member.
		// JSON params structured object. Unmarshal to the args object.
		err := json.Unmarshal(*c.Request.Params, args)
		if err != nil {
			c.Err = &Error{
				Code:    E_INVALID_REQ,
				Message: err.Error(),
				Data:    c.Request.Params,
			}
		}
	}
	return c.Err
}

// WriteResponse encodes the response and writes it to the ResponseWriter.
func (c *CodecRequest) WriteResponse(w http.ResponseWriter, reply interface{}) {
	res := &ServerResponse{
		Version: Version,
		Result:  reply,
		Id:      c.Request.Id,
	}
	c.writeServerResponse(w, res)
}

func (c *CodecRequest) WriteError(w http.ResponseWriter, status int, err error) {
	jsonErr, ok := err.(*Error)
	if !ok {
		jsonErr = &Error{
			Code:    E_SERVER,
			Message: err.Error(),
		}
	}
	res := &ServerResponse{
		Version: Version,
		Error:   jsonErr,
		Id:      c.Request.Id,
	}
	c.writeServerResponse(w, res)
}

func (c *CodecRequest) writeServerResponse(w http.ResponseWriter, res *ServerResponse) {
	// Id is null for notifications and they don't have a response.
	if c.Request.Id != nil {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		encoder := json.NewEncoder(c.encoder.Encode(w))
		err := encoder.Encode(res)

		// Not sure in which case will this happen. But seems harmless.
		if err != nil {
			rpc.WriteError(w, 400, err.Error())
		}
	}
}

type EmptyResponse struct {
}
