// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package redis

import (
	"reflect"
	"strings"

	"github.com/wandoulabs/codis/extern/redis-port/pkg/libs/errors"
	"github.com/wandoulabs/codis/extern/redis-port/pkg/libs/log"
)

type HandlerFunc func(arg0 interface{}, args ...[]byte) (Resp, error)

type HandlerTable map[string]HandlerFunc

func NewHandlerTable(o interface{}) (map[string]HandlerFunc, error) {
	if o == nil {
		return nil, errors.New("handler is nil")
	}
	t := make(map[string]HandlerFunc)
	r := reflect.TypeOf(o)
	for i := 0; i < r.NumMethod(); i++ {
		m := r.Method(i)
		if m.Name[0] < 'A' || m.Name[0] > 'Z' {
			continue
		}
		n := strings.ToLower(m.Name)
		if h, err := createHandlerFunc(o, &m.Func); err != nil {
			return nil, err
		} else if _, exists := t[n]; exists {
			return nil, errors.Errorf("func.name = '%s' has already exists", m.Name)
		} else {
			t[n] = h
		}
	}
	return t, nil
}

func MustHandlerTable(o interface{}) map[string]HandlerFunc {
	t, err := NewHandlerTable(o)
	if err != nil {
		log.PanicError(err, "create redis handler map failed")
	}
	return t
}

func createHandlerFunc(o interface{}, f *reflect.Value) (HandlerFunc, error) {
	t := f.Type()
	arg0Type := reflect.TypeOf((*interface{})(nil)).Elem()
	argsType := reflect.TypeOf([][]byte{})
	if t.NumIn() != 3 || t.In(1) != arg0Type || t.In(2) != argsType {
		return nil, errors.Errorf("register with invalid func type = '%s'", t)
	}
	ret0Type := reflect.TypeOf((*Resp)(nil)).Elem()
	ret1Type := reflect.TypeOf((*error)(nil)).Elem()
	if t.NumOut() != 2 || t.Out(0) != ret0Type || t.Out(1) != ret1Type {
		return nil, errors.Errorf("register with invalid func type = '%s'", t)
	}
	return func(arg0 interface{}, args ...[]byte) (Resp, error) {
		var arg0Value reflect.Value
		if arg0 == nil {
			arg0Value = reflect.ValueOf((*interface{})(nil))
		} else {
			arg0Value = reflect.ValueOf(arg0)
		}
		var input, output []reflect.Value
		input = []reflect.Value{reflect.ValueOf(o), arg0Value, reflect.ValueOf(args)}
		if t.IsVariadic() {
			output = f.CallSlice(input)
		} else {
			output = f.Call(input)
		}
		var ret0 Resp
		var ret1 error
		if i := output[0].Interface(); i != nil {
			ret0 = i.(Resp)
		}
		if i := output[1].Interface(); i != nil {
			ret1 = i.(error)
		}
		return ret0, ret1
	}, nil
}

func ParseArgs(resp Resp) (cmd string, args [][]byte, err error) {
	var array []Resp
	if o, ok := resp.(*Array); !ok {
		return "", nil, errors.Errorf("expect array, but got type = '%s'", resp.Type())
	} else if o == nil || len(o.Value) == 0 {
		return "", nil, errors.New("request is an empty array")
	} else {
		array = o.Value
	}
	slices := make([][]byte, 0, len(array))
	for i, resp := range array {
		if o, ok := resp.(*BulkBytes); !ok {
			return "", nil, errors.Errorf("args[%d], expect bulkbytes, but got '%s'", i, resp.Type())
		} else if i == 0 && len(o.Value) == 0 {
			return "", nil, errors.New("command is empty")
		} else {
			slices = append(slices, o.Value)
		}
	}
	return strings.ToLower(string(slices[0])), slices[1:], nil
}
