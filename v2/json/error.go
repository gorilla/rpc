// Copyright 2009 The Go Authors. All rights reserved.
// Copyright 2012-2013 The Gorilla Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package json

import (
	"fmt"
)

// An Error is a wrapper for a JSON interface value. It can be used by either
// a service's hanlder func to write more complex JSON data to an error field
// of a server's response, or by a client to read it.
type Error struct {
	Data interface{}
}

func (e *Error) Error() string {
	return fmt.Sprintf("%v", e.Data)
}
