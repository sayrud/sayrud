// Copyright 2024 E99p1ant. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package module

import (
	"crypto/md5"
	"encoding/hex"

	"github.com/dop251/goja"
)

type Hash struct {
	vm *goja.Runtime
}

func SetHash(vm *goja.Runtime) {
	hash := Hash{vm: vm}

	obj := vm.NewObject()
	obj.Set("md5", hash.md5Sum)

	vm.Set("Hash", obj)
}

func (h *Hash) md5Sum(call goja.FunctionCall) goja.Value {
	input := call.Argument(0).ToString().String()

	hasher := md5.New()
	hasher.Write([]byte(input))
	md5hash := hex.EncodeToString(hasher.Sum(nil))

	return h.vm.ToValue(md5hash)
}
