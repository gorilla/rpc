// Copyright 2009 The Go Authors. All rights reserved.
// Copyright 2012 The Gorilla Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rpc

import (
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"
)

var (
	// Precompute the reflect.Type of error and http.Request
	typeOfError   = reflect.TypeOf((*error)(nil)).Elem()
	typeOfRequest = reflect.TypeOf((*http.Request)(nil)).Elem()
)

// ----------------------------------------------------------------------------
// service
// ----------------------------------------------------------------------------

type service struct {
	name     string        // name of service
	rcvr     reflect.Value // receiver of methods for the service
	rcvrType reflect.Type  // type of the receiver
}

type serviceMethod struct {
	service   *service       // service
	method    reflect.Method // receiver method
	argsType  reflect.Type   // type of the request argument
	replyType reflect.Type   // type of the response argument
}

// ----------------------------------------------------------------------------
// serviceMap
// ----------------------------------------------------------------------------

// serviceMap is a registry for services.
type serviceMap struct {
	mutex   sync.Mutex
	methods map[string]*serviceMethod
	aliases map[string]string
}

// register adds a new service using reflection to extract its methods.
func (m *serviceMap) register(rcvr interface{}, name string) error {
	// Setup service.
	s := &service{
		name:     name,
		rcvr:     reflect.ValueOf(rcvr),
		rcvrType: reflect.TypeOf(rcvr),
		//methods:  make(map[string]*serviceMethod),
	}
	if name == "" {
		s.name = reflect.Indirect(s.rcvr).Type().Name()
		if !isExported(s.name) {
			return fmt.Errorf("rpc: type %q is not exported", s.name)
		}
	}
	if s.name == "" {
		return fmt.Errorf("rpc: no service name for type %q",
			s.rcvrType.String())
	}
	// Setup methods.
	methods := map[string]*serviceMethod{}
	for i := 0; i < s.rcvrType.NumMethod(); i++ {
		method := s.rcvrType.Method(i)
		mtype := method.Type
		// Method must be exported.
		if method.PkgPath != "" {
			continue
		}
		// Method needs four ins: receiver, *http.Request, *args, *reply.
		if mtype.NumIn() != 4 {
			continue
		}
		// First argument must be a pointer and must be http.Request.
		reqType := mtype.In(1)
		if reqType.Kind() != reflect.Ptr || reqType.Elem() != typeOfRequest {
			continue
		}
		// Second argument must be a pointer and must be exported.
		args := mtype.In(2)
		if args.Kind() != reflect.Ptr || !isExportedOrBuiltin(args) {
			continue
		}
		// Third argument must be a pointer and must be exported.
		reply := mtype.In(3)
		if reply.Kind() != reflect.Ptr || !isExportedOrBuiltin(reply) {
			continue
		}
		// Method needs one out: error.
		if mtype.NumOut() != 1 {
			continue
		}
		if returnType := mtype.Out(0); returnType != typeOfError {
			continue
		}
		methodName, err := toLowerCase(method.Name)
		if err != nil {
			continue
		}
		methodName = s.name + "/" + methodName
		methods[methodName] = &serviceMethod{
			service:   s,
			method:    method,
			argsType:  args.Elem(),
			replyType: reply.Elem(),
		}
	}
	if len(methods) == 0 {
		return fmt.Errorf(
			"rpc: %q has no exported methods of suitable type",
			s.name,
		)
	}
	// Add to the map.
	m.mutex.Lock()
	defer m.mutex.Unlock()
	for name, method := range methods {
		if _, ok := m.methods[name]; ok {
			return fmt.Errorf("rpc: service method already defined: %q", name)
		}
		m.methods[name] = method
	}
	return nil
}

// register method for specific service that does not export all methods with method full name.
func (m *serviceMap) registerMethod(rcvr any, name string, methodName string) error {
	if name == "" {
		return fmt.Errorf("rpc: service method name must not be empty")
	}
	// Setup service.
	s := &service{
		rcvr:     reflect.ValueOf(rcvr),
		rcvrType: reflect.TypeOf(rcvr),
	}
	s.name = reflect.Indirect(s.rcvr).Type().Name()
	method, exists := s.rcvrType.MethodByName(methodName)
	if !exists {
		return fmt.Errorf("rpc: service method not found: %q", methodName)
	}
	mtype := method.Type
	// Method must be exported.
	if method.PkgPath != "" {
		return fmt.Errorf("rpc: service method PkgPath %s is not empty", method.PkgPath)
	}
	// Method needs four ins: receiver, *http.Request, *args, *reply.
	if mtype.NumIn() != 4 {
		return fmt.Errorf("rpc: service method NumIn %d is not 4", mtype.NumIn())
	}
	// First argument must be a pointer and must be http.Request.
	reqType := mtype.In(1)
	if reqType.Kind() != reflect.Ptr || reqType.Elem() != typeOfRequest {
		return fmt.Errorf("rpc: service method first arg must be an http Request Pointer")
	}
	// Second argument must be a pointer and must be exported.
	args := mtype.In(2)
	if args.Kind() != reflect.Ptr || !isExportedOrBuiltin(args) {
		return fmt.Errorf("rpc: service method second arg must be an Pointer")
	}
	// Third argument must be a pointer and must be exported.
	reply := mtype.In(3)
	if reply.Kind() != reflect.Ptr || !isExportedOrBuiltin(reply) {
		return fmt.Errorf("rpc: service method third arg must be an Pointer")
	}
	// Method needs one out: error.
	if mtype.NumOut() != 1 {
		return fmt.Errorf("rpc: service method NumOut must be 1, now is %d", mtype.NumOut())
	}
	if returnType := mtype.Out(0); returnType != typeOfError {
		return fmt.Errorf("rpc: service method typeof NumOut must be Error")
	}
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if _, ok := m.methods[name]; ok {
		return fmt.Errorf("rpc: service method already defined: %q", name)
	}
	m.methods[name] = &serviceMethod{
		service:   s,
		method:    method,
		argsType:  args.Elem(),
		replyType: reply.Elem(),
	}
	return nil
}

// allow to use different names to call same rpc method.
func (m *serviceMap) registerAlias(alias, target string) error {
	if _, ok := m.methods[target]; !ok {
		return fmt.Errorf("rpc: service method %s for alias not found", target)
	}
	if _, ok := m.aliases[alias]; ok {
		return fmt.Errorf("rpc: service method alias %s already defined", alias)
	}
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.aliases[alias] = target
	return nil
}

// get returns a registered service given a method name.
//
// The method name uses a dotted notation as in "Service.Method".
func (m *serviceMap) get(method string) (*service, *serviceMethod, error) {
	//parts := strings.Split(method, ".")
	//if len(parts) != 2 {
	//	err := fmt.Errorf("rpc: service/method request ill-formed: %q", method)
	//	return nil, nil, err
	//}
	m.mutex.Lock()
	target, ok := m.aliases[method]
	if ok {
		method = target
	}
	serviceMethod := m.methods[method]
	m.mutex.Unlock()
	if serviceMethod == nil {
		err := fmt.Errorf("rpc: can't find service method %q", method)
		return nil, nil, err
	}
	service := serviceMethod.service
	if service == nil {
		err := fmt.Errorf("rpc: can't find service %q", method)
		return nil, nil, err
	}
	return service, serviceMethod, nil
}

// isExported returns true of a string is an exported (upper case) name.
func isExported(name string) bool {
	rune, _ := utf8.DecodeRuneInString(name)
	return unicode.IsUpper(rune)
}

// isExportedOrBuiltin returns true if a type is exported or a builtin.
func isExportedOrBuiltin(t reflect.Type) bool {
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	// PkgPath will be non-empty even for an exported type,
	// so we need to check the type name as well.
	return isExported(t.Name()) || t.PkgPath() == ""
}

func isValidRouteName(s string) bool {
	isASCII, hasUpper := true, false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= utf8.RuneSelf {
			isASCII = false
			break
		}
		hasUpper = hasUpper || ('A' <= c && c <= 'Z')
	}
	return isASCII
}

func toLowerCase(s string) (string, error) {
	if !isValidRouteName(s) {
		return "", fmt.Errorf("invalid route name %s", s)
	}
	if len(s) == 0 {
		return "", nil
	}
	if len(s) == 1 {
		return strings.ToLower(s), nil
	}
	return strings.ToLower(string(s[0])) + s[1:], nil
}
