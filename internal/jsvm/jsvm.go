// Copyright 2024 E99p1ant. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package jsvm

import (
	"github.com/dop251/goja"
	"github.com/pkg/errors"
)

type NewVMOptions struct {
	RequestMethod string
	RequestPath   string
	RequestQuery  map[string]interface{}
	RequestBody   map[string]interface{}
}

func NewVM(options NewVMOptions) (*goja.Runtime, error) {
	vm := goja.New()

	request := vm.NewObject()
	for k, v := range map[string]interface{}{
		"method": options.RequestMethod,
		"path":   options.RequestPath,
		"query":  options.RequestQuery,
		"body":   options.RequestBody,
	} {
		if params, ok := v.(map[string]interface{}); ok {
			obj := vm.NewObject()
			for pk, pv := range params {
				if err := obj.Set(pk, pv); err != nil {
					return nil, errors.Wrap(err, "set request params")
				}
			}
			v = obj
		}

		if err := request.Set(k, v); err != nil {
			return nil, err
		}
	}
	if err := vm.Set("$request", request); err != nil {
		return nil, errors.Wrap(err, "set $request")
	}

	return vm, nil
}
