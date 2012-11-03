// Copyright 2009 The Go Authors. All rights reserved.
// Copyright 2012 The Gorilla Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rpc

import (
	"compress/flate"
	"compress/gzip"
	"io"
	"net/http"
	"strings"
	"unicode"
)

// gzipEncoder implements the gzip compressed http encoder.
type gzipEncoder struct {
}
func (enc *gzipEncoder) Encode(w http.ResponseWriter) io.Writer {
	w.Header().Set("Content-Encoding", "gzip")
	return gzip.NewWriter(w)
}

// flateEncoder implements the flate compressed http encoder.
type flateEncoder struct {
}

func (enc *flateEncoder) Encode(w http.ResponseWriter) io.Writer {
	w.Header().Set("Content-Encoding", "deflate")
	flateWriter, err := flate.NewWriter(w, flate.DefaultCompression)
	if err != nil {
		return w
	}
	return flateWriter
}

// CompressionSelector generates the compressed http encoder.
type CompressionSelector struct {
}

// acceptedEnc returns the first compression type in "Accept-Encoding" header
// field of the request.
func acceptedEnc(req *http.Request) string {
	encHeader := req.Header.Get("Accept-Encoding")
	if encHeader == "" {
		return ""
	}
	encTypes := strings.FieldsFunc(encHeader, func(r rune) bool {
		return unicode.IsSpace(r) || r == ','
	})
	for _, enc := range encTypes {
		if enc == "gzip" || enc == "deflate" {
			return enc
		}
	}
	return ""
}

// Select method selects the correct compression encoder based on http HEADER.
func (_ *CompressionSelector) Select(r *http.Request) Encoder {
	switch acceptedEnc(r) {
	case "gzip":
		return &gzipEncoder{}
	case "flate":
		return &flateEncoder{}
	}
	return DefaultEncoder
}
