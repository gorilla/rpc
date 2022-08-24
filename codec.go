// Copyright 2009 The Go Authors. All rights reserved.
// Copyright 2012 The Gorilla Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rpc

import "net/http"

// ----------------------------------------------------------------------------
// Codec
// ----------------------------------------------------------------------------

// Codec creates a CodecRequest to process each request.
type Codec interface {
	NewRequest(*http.Request) Request
}

// Request CodecRequest decodes a request and encodes a response using a specific
// serialization scheme.
type Request interface {
	// Method Reads the request and returns the RPC method name.
	Method() (string, error)
	// ReadRequest Reads the request filling the RPC method args.
	ReadRequest(interface{}) error
	// WriteResponse Writes the response using the RPC method reply.
	WriteResponse(http.ResponseWriter, interface{})
	// WriteError Writes an error produced by the server.
	WriteError(w http.ResponseWriter, status int, err error)
}
