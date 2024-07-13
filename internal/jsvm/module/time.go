// Copyright 2024 E99p1ant. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package module

import (
	"time"

	"github.com/dop251/goja"
)

type Time struct {
	vm *goja.Runtime
}

func SetTime(vm *goja.Runtime) {
	time := Time{vm: vm}

	obj := vm.NewObject()
	_ = obj.Set("now", time.now)

	_ = vm.Set("Time", obj)
}

func (h *Time) now() goja.Value {
	return h.vm.ToValue(time.Now())
}
