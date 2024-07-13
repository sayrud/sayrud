// Copyright 2024 E99p1ant. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package module

import (
	"strings"

	"github.com/dop251/goja"
)

type String struct {
	vm *goja.Runtime
}

func SetString(vm *goja.Runtime) {
	string := String{vm: vm}

	obj := vm.NewObject()
	_ = obj.Set("lower", string.toLower)
	_ = obj.Set("upper", string.toUpper)

	_ = vm.Set("String", obj)
}

func (s *String) toLower(call goja.FunctionCall) goja.Value {
	input := call.Argument(0).ToString().String()
	return s.vm.ToValue(strings.ToLower(input))
}

func (s *String) toUpper(call goja.FunctionCall) goja.Value {
	input := call.Argument(0).ToString().String()
	return s.vm.ToValue(strings.ToUpper(input))
}
