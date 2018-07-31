// Copyright (C) 2017 Google Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package resolve

import (
	"context"
	"fmt"
	"reflect"

	"github.com/google/gapid/core/data/dictionary"
	"github.com/google/gapid/core/image"
	"github.com/google/gapid/core/math/sint"
	"github.com/google/gapid/core/os/device"
	"github.com/google/gapid/core/os/device/bind"
	"github.com/google/gapid/gapis/api"
	"github.com/google/gapid/gapis/capture"
	"github.com/google/gapid/gapis/database"
	"github.com/google/gapid/gapis/messages"
	"github.com/google/gapid/gapis/service"
	"github.com/google/gapid/gapis/service/box"
	"github.com/google/gapid/gapis/service/path"
	"github.com/google/gapid/gapis/trace"
)

// Capture resolves and returns the capture from the path p.
func Capture(ctx context.Context, p *path.Capture) (*service.Capture, context.Context, error) {
	c, err := capture.ResolveFromPath(ctx, p)
	if err != nil {
		return nil, ctx, err
	}
	return c.Service(ctx, p), ctx, nil
}

// Device resolves and returns the device from the path p.
func Device(ctx context.Context, p *path.Device) (*device.Instance, context.Context, error) {
	device := bind.GetRegistry(ctx).Device(p.ID.ID())
	if device == nil {
		return nil, ctx, &service.ErrDataUnavailable{Reason: messages.ErrUnknownDevice(ctx)}
	}
	return device.Instance(), ctx, nil
}

// DeviceTraceConfiguration resolves and returns the trace config for a device.
func DeviceTraceConfiguration(ctx context.Context, p *path.DeviceTraceConfiguration) (*service.DeviceTraceConfiguration, context.Context, error) {
	c, err := trace.TraceConfiguration(ctx, p.Device)

	if err != nil {
		return nil, ctx, err
	}

	config := &service.DeviceTraceConfiguration{
		ServerLocalPath:      c.ServerLocalPath,
		CanSpecifyCwd:        c.CanSpecifyCwd,
		CanUploadApplication: c.CanUploadApplication,
		HasCache:             c.HasCache,
		CanSpecifyEnv:        c.CanSpecifyEnv,
		PreferredRootUri:     c.PreferredRootUri,
		Apis:                 make([]*service.DeviceAPITraceConfiguration, len(c.Apis)),
	}

	for i, opt := range c.Apis {
		config.Apis[i] = &service.DeviceAPITraceConfiguration{
			Api:                        opt.APIName,
			CanDisablePcs:              opt.CanDisablePCS,
			MidExecutionCaptureSupport: opt.MidExecutionCaptureSupport,
		}
	}
	return config, ctx, nil
}

// ImageInfo resolves and returns the ImageInfo from the path p.
func ImageInfo(ctx context.Context, p *path.ImageInfo) (*image.Info, context.Context, error) {
	obj, err := database.Resolve(ctx, p.ID.ID())
	if err != nil {
		return nil, ctx, err
	}
	ii, ok := obj.(*image.Info)
	if !ok {
		return nil, ctx, fmt.Errorf("Path %s gave %T, expected *image.Info", p, obj)
	}
	return ii, ctx, err
}

// Blob resolves and returns the byte slice from the path p.
func Blob(ctx context.Context, p *path.Blob) ([]byte, context.Context, error) {
	obj, err := database.Resolve(ctx, p.ID.ID())
	if err != nil {
		return nil, ctx, err
	}
	bytes, ok := obj.([]byte)
	if !ok {
		return nil, ctx, fmt.Errorf("Path %s gave %T, expected []byte", p, obj)
	}
	return bytes, ctx, nil
}

// Field resolves and returns the field from the path p.
func Field(ctx context.Context, p *path.Field) (interface{}, context.Context, error) {
	obj, ctx, err := ResolveInternal(ctx, p.Parent())
	if err != nil {
		return nil, ctx, err
	}
	v, err := field(ctx, reflect.ValueOf(obj), p.Name, p)
	if err != nil {
		return nil, ctx, err
	}
	return v.Interface(), ctx, nil
}

func field(ctx context.Context, s reflect.Value, name string, p path.Node) (reflect.Value, error) {
	for {
		if isNil(s) {
			return reflect.Value{}, &service.ErrInvalidPath{
				Reason: messages.ErrNilPointerDereference(ctx),
				Path:   p.Path(),
			}
		}

		if pp, ok := s.Interface().(api.PropertyProvider); ok {
			if p := pp.Properties().Find(name); p != nil {
				return reflect.ValueOf(p.Get()), nil
			}
		}

		switch s.Kind() {
		case reflect.Struct:
			f := s.FieldByName(name)
			if !f.IsValid() {
				return reflect.Value{}, &service.ErrInvalidPath{
					Reason: messages.ErrFieldDoesNotExist(ctx, typename(s.Type()), name),
					Path:   p.Path(),
				}
			}
			return f, nil
		case reflect.Interface, reflect.Ptr:
			s = s.Elem()
		default:
			return reflect.Value{}, &service.ErrInvalidPath{
				Reason: messages.ErrFieldDoesNotExist(ctx, typename(s.Type()), name),
				Path:   p.Path(),
			}
		}
	}
}

// ArrayIndex resolves and returns the array or slice element from the path p.
func ArrayIndex(ctx context.Context, p *path.ArrayIndex) (interface{}, context.Context, error) {
	obj, ctx, err := ResolveInternal(ctx, p.Parent())
	if err != nil {
		return nil, ctx, err
	}

	a := reflect.ValueOf(obj)
	switch {
	case box.IsMemorySlice(a.Type()):
		slice := box.AsMemorySlice(a)
		if count := slice.Count(); p.Index >= count {
			return nil, ctx, errPathOOB(ctx, p.Index, "Index", 0, count-1, p)
		}
		return slice.ISlice(p.Index, p.Index+1), ctx, nil

	default:
		switch a.Kind() {
		case reflect.Array, reflect.Slice, reflect.String:
			if count := uint64(a.Len()); p.Index >= count {
				return nil, ctx, errPathOOB(ctx, p.Index, "Index", 0, count-1, p)
			}
			return a.Index(int(p.Index)).Interface(), ctx, nil

		default:
			return nil, ctx, &service.ErrInvalidPath{
				Reason: messages.ErrTypeNotArrayIndexable(ctx, typename(a.Type())),
				Path:   p.Path(),
			}
		}
	}
}

// Slice resolves and returns the subslice from the path p.
func Slice(ctx context.Context, p *path.Slice) (interface{}, context.Context, error) {
	obj, ctx, err := ResolveInternal(ctx, p.Parent())
	if err != nil {
		return nil, ctx, err
	}
	a := reflect.ValueOf(obj)
	switch {
	case box.IsMemorySlice(a.Type()):
		slice := box.AsMemorySlice(a)
		if count := slice.Count(); p.Start >= count || p.End > count {
			return nil, ctx, errPathSliceOOB(ctx, p.Start, p.End, count, p)
		}
		return slice.ISlice(p.Start, p.End), ctx, nil

	default:
		switch a.Kind() {
		case reflect.Array, reflect.Slice, reflect.String:
			if int(p.Start) >= a.Len() || int(p.End) > a.Len() {
				return nil, ctx, errPathSliceOOB(ctx, p.Start, p.End, uint64(a.Len()), p)
			}
			return a.Slice(int(p.Start), int(p.End)).Interface(), ctx, nil

		default:
			return nil, ctx, &service.ErrInvalidPath{
				Reason: messages.ErrTypeNotSliceable(ctx, typename(a.Type())),
				Path:   p.Path(),
			}
		}
	}
}

// MapIndex resolves and returns the map value from the path p.
func MapIndex(ctx context.Context, p *path.MapIndex) (interface{}, context.Context, error) {
	obj, ctx, err := ResolveInternal(ctx, p.Parent())
	if err != nil {
		return nil, ctx, err
	}

	d := dictionary.From(obj)
	if d == nil {
		return nil, ctx, &service.ErrInvalidPath{
			Reason: messages.ErrTypeNotMapIndexable(ctx, typename(reflect.TypeOf(obj))),
			Path:   p.Path(),
		}
	}

	key, ok := convert(reflect.ValueOf(p.KeyValue(ctx)), d.KeyTy())
	if !ok {
		return nil, ctx, &service.ErrInvalidPath{
			Reason: messages.ErrIncorrectMapKeyType(ctx,
				typename(reflect.TypeOf(p.KeyValue(ctx))), // got
				typename(d.KeyTy())),                      // expected
			Path: p.Path(),
		}
	}

	val, ok := d.Lookup(ctx, key.Interface())
	if !ok {
		return nil, ctx, &service.ErrInvalidPath{
			Reason: messages.ErrMapKeyDoesNotExist(ctx, key.Interface()),
			Path:   p.Path(),
		}
	}
	return val, ctx, nil
}

// memoryLayout resolves the memory layout for the capture of the given path.
func memoryLayout(ctx context.Context, p path.Node) (*device.MemoryLayout, context.Context, error) {
	cp := path.FindCapture(p)
	if cp == nil {
		return nil, ctx, errPathNoCapture(ctx, p)
	}

	c, err := capture.ResolveFromPath(ctx, cp)
	if err != nil {
		return nil, ctx, err
	}

	return c.Header.ABI.MemoryLayout, ctx, nil
}

// ResolveService resolves and returns the object, value or memory at the path p,
// converting the final result to the service representation.
func ResolveService(ctx context.Context, p path.Node) (interface{}, context.Context, error) {
	v, ctx, err := ResolveInternal(ctx, p)
	if err != nil {
		return nil, ctx, err
	}
	return internalToService(ctx, v)
}

// ResolveInternal resolves and returns the object, value or memory at the path
// p without converting the potentially internal result to a service
// representation.
func ResolveInternal(ctx context.Context, p path.Node) (interface{}, context.Context, error) {
	switch p := p.(type) {
	case *path.ArrayIndex:
		return ArrayIndex(ctx, p)
	case *path.As:
		return As(ctx, p)
	case *path.Blob:
		return Blob(ctx, p)
	case *path.Capture:
		return Capture(ctx, p)
	case *path.Command:
		return Cmd(ctx, p)
	case *path.Commands:
		return Commands(ctx, p)
	case *path.CommandTree:
		return CommandTree(ctx, p)
	case *path.CommandTreeNode:
		return CommandTreeNode(ctx, p)
	case *path.CommandTreeNodeForCommand:
		return CommandTreeNodeForCommand(ctx, p)
	case *path.ConstantSet:
		return ConstantSet(ctx, p)
	case *path.Context:
		return Context(ctx, p)
	case *path.Contexts:
		return Contexts(ctx, p)
	case *path.Device:
		return Device(ctx, p)
	case *path.DeviceTraceConfiguration:
		return DeviceTraceConfiguration(ctx, p)
	case *path.Events:
		return Events(ctx, p)
	case *path.FramebufferObservation:
		return FramebufferObservation(ctx, p)
	case *path.Field:
		return Field(ctx, p)
	case *path.GlobalState:
		return GlobalState(ctx, p)
	case *path.ImageInfo:
		return ImageInfo(ctx, p)
	case *path.MapIndex:
		return MapIndex(ctx, p)
	case *path.Memory:
		return Memory(ctx, p)
	case *path.Metrics:
		return Metrics(ctx, p)
	case *path.Mesh:
		return Mesh(ctx, p)
	case *path.Parameter:
		return Parameter(ctx, p)
	case *path.Report:
		return Report(ctx, p)
	case *path.ResourceData:
		return ResourceData(ctx, p)
	case *path.Resources:
		return Resources(ctx, p.Capture)
	case *path.Result:
		return Result(ctx, p)
	case *path.Slice:
		return Slice(ctx, p)
	case *path.State:
		return State(ctx, p)
	case *path.StateTree:
		return StateTree(ctx, p)
	case *path.StateTreeNode:
		return StateTreeNode(ctx, p)
	case *path.StateTreeNodeForPath:
		return StateTreeNodeForPath(ctx, p)
	case *path.Thumbnail:
		return Thumbnail(ctx, p)
	case *path.Stats:
		return Stats(ctx, p)
	default:
		return nil, ctx, fmt.Errorf("Unknown path type %T", p)
	}
}

func typename(t reflect.Type) string {
	if s := t.Name(); len(s) > 0 {
		return s
	}
	switch t.Kind() {
	case reflect.Ptr:
		return "ptr<" + typename(t.Elem()) + ">"
		// TODO: Format other composite types?
	default:
		return t.String()
	}
}

func convert(val reflect.Value, ty reflect.Type) (reflect.Value, bool) {
	if !val.IsValid() {
		return reflect.Zero(ty), true
	}
	valTy := val.Type()
	if valTy == ty {
		return val, true
	}
	if valTy.ConvertibleTo(ty) {
		return val.Convert(ty), true
	}
	// slice -> array
	if valTy.Kind() == reflect.Slice && ty.Kind() == reflect.Array {
		if valTy.Elem().ConvertibleTo(ty.Elem()) {
			c := sint.Min(val.Len(), ty.Len())
			out := reflect.New(ty).Elem()
			for i := 0; i < c; i++ {
				v, ok := convert(val.Index(i), ty.Elem())
				if !ok {
					return val, false
				}
				out.Index(i).Set(v)
			}
			return out, true
		}
	}
	return val, false
}
