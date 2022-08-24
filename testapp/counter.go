// Copyright 2009 The Go Authors. All rights reserved.
// Copyright 2013 The Gorilla Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"log"
	"net/http"
	"wbylovesun/rpc"
	"wbylovesun/rpc/json"
)

type Counter struct {
	Count int
}

type IncrReq struct {
	Delta int
}

// Incr Notification.
func (c *Counter) Incr(r *http.Request, req *IncrReq, res *json.EmptyResponse) error {
	log.Printf("<- Incr %+v", *req)
	c.Count += req.Delta
	return nil
}

type GetReq struct {
}

func (c *Counter) Get(r *http.Request, req *GetReq, res *Counter) error {
	log.Printf("<- Get %+v", *req)
	*res = *c
	log.Printf("-> %v", *res)
	return nil
}

func main() {
	address := flag.String("address", ":63342", "")
	s := rpc.NewServer()
	s.RegisterCodec(json.NewCustomCodec(&rpc.CompressionSelector{}), "application/json")
	s.RegisterService(new(Counter), "")
	http.Handle("/", http.StripPrefix("/", http.FileServer(http.Dir("./"))))
	http.Handle("/jsonrpc/", s)
	log.Fatal(http.ListenAndServe(*address, nil))
}
