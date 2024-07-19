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
	name     string                    // name of service
	rcvr     reflect.Value             // receiver of methods for the service
	rcvrType reflect.Type              // type of the receiver
	methods  map[string]*serviceMethod // registered methods
	services map[string]*service       // 保存下一级的其他服务
}

type serviceMethod struct {
	method    reflect.Method // receiver method
	argsType  reflect.Type   // type of the request argument
	replyType reflect.Type   // type of the response argument
}

// ----------------------------------------------------------------------------
// serviceMap
// ----------------------------------------------------------------------------

// serviceMap is a registry for services.
type serviceMap struct {
	mutex    sync.Mutex
	services map[string]*service
}

// 注册多个服务名的服务，每个服务名增加一个服务
func (m *serviceMap) registryService(name string) (*service, error) {
	// 切分服务全名
	parts := strings.Split(name, ".")

	var lastService *service
	var newService *service
	var serviceName string

	// 遍历服务名
	for _, part := range parts {
		// 构建一个新的服务
		newService = &service{
			name:     part,
			methods:  make(map[string]*serviceMethod),
			services: make(map[string]*service),
		}

		// 服务名字
		serviceName += part + "."
		println(serviceName)
		// 如果一开始是第一个服务名，要放到ServiceMap中
		if lastService == nil {
			lastService = newService

			m.mutex.Lock()

			if m.services == nil {
				m.services = make(map[string]*service)
			} else if _, ok := m.services[lastService.name]; ok {
				return nil, fmt.Errorf("rpc: service already defined: %q",
					serviceName)
			}
			m.services[lastService.name] = lastService

			m.mutex.Unlock()
		} else {

			if _, ok := lastService.services[newService.name]; ok {
				return nil, fmt.Errorf("rpc: service already defined: %q",
					serviceName)
			}

			lastService.services[newService.name] = newService
			lastService = newService
		}
	}

	return newService, nil
}

// register adds a new service using reflection to extract its methods.
func (m *serviceMap) register(rcvr interface{}, name string) error {
	// Setup service.
	s, err := m.registryService(name)
	if err != nil {
		return err
	}
	s.rcvr = reflect.ValueOf(rcvr)
	s.rcvrType = reflect.TypeOf(rcvr)

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
		s.methods[method.Name] = &serviceMethod{
			method:    method,
			argsType:  args.Elem(),
			replyType: reply.Elem(),
		}
	}
	if len(s.methods) == 0 {
		return fmt.Errorf("rpc: %q has no exported methods of suitable type",
			s.name)
	}
	// // Add to the map.
	// m.mutex.Lock()
	// defer m.mutex.Unlock()
	// if m.services == nil {
	// 	m.services = make(map[string]*service)
	// } else if _, ok := m.services[s.name]; ok {
	// 	return fmt.Errorf("rpc: service already defined: %q", s.name)
	// }
	// m.services[s.name] = s
	return nil
}

// get returns a registered service given a method name.
//
// The method name uses a dotted notation as in "Service.Method".
func (m *serviceMap) get(method string) (*service, *serviceMethod, error) {
	// 分割方法名，考虑到可能有多级服务名
	parts := strings.Split(method, ".")
	if len(parts) < 2 {
		err := fmt.Errorf("rpc: service/method request ill-formed: %q", method)
		return nil, nil, err
	}

	// 实际方法名
	methodName := parts[len(parts)-1]

	// 按层次遍历服务
	m.mutex.Lock()
	var service *service
	for index, part := range parts {
		if index == len(parts)-1 {
			break
		}

		if service == nil {
			service = m.services[part]
		} else {
			service = service.services[part]
		}

		if service == nil {
			err := fmt.Errorf("rpc: can't find service %q", method)
			return nil, nil, err
		}
	}
	m.mutex.Unlock()

	serviceMethod := service.methods[methodName]
	if serviceMethod == nil {
		err := fmt.Errorf("rpc: can't find method %q", method)
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
