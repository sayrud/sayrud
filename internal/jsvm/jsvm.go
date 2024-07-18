// Copyright 2024 E99p1ant. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package jsvm

import (
	"net/http"

	"github.com/dop251/goja"
	"github.com/pkg/errors"

	"github.com/wuhan005/sayrud/internal/jsvm/module"
)

type NewVMOptions struct {
	RequestMethod string
	RequestPath   string
	RequestQuery  map[string]interface{}
	RequestBody   map[string]interface{}
	RequestHeader http.Header
	RequestIP     string

	This map[string]interface{}
	That map[string]interface{}
}

func NewVM(options NewVMOptions) (*goja.Runtime, error) {
	vm := goja.New()

	if options.RequestMethod != "" && options.RequestPath != "" {
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

		if err := request.Set("ip", options.RequestIP); err != nil {
			return nil, errors.Wrap(err, "set request ip")
		}
		if err := request.Set("userAgent", options.RequestHeader.Get("User-Agent")); err != nil {
			return nil, errors.Wrap(err, "set request ua")
		}

		if err := vm.Set("$request", request); err != nil {
			return nil, errors.Wrap(err, "set $request")
		}
	}

	if options.This != nil {
		this := vm.NewObject()
		for k, v := range options.This {
			if err := this.Set(k, v); err != nil {
				return nil, errors.Wrap(err, "set this")
			}
		}
		if err := vm.Set("$this", this); err != nil {
			return nil, errors.Wrap(err, "set this")
		}
	}
	if options.That != nil {
		that := vm.NewObject()
		for k, v := range options.That {
			if err := that.Set(k, v); err != nil {
				return nil, errors.Wrap(err, "set that")
			}
		}
		if err := vm.Set("$that", that); err != nil {
			return nil, errors.Wrap(err, "set that")
		}
	}

	module.SetHash(vm)
	module.SetString(vm)
	module.SetTime(vm)

	return vm, nil
}
