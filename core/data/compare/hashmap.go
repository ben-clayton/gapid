// Copyright (C) 2018 Google Inc.
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

package compare

import (
	"fmt"
	"math"
	"reflect"
	"sort"
)

type keyVal struct {
	key, val reflect.Value
	valHash  uint64
}

type hashmap map[uint64][]keyVal

func newHashmap(m reflect.Value) hashmap {
	out := make(hashmap, m.Len())
	for _, k := range m.MapKeys() {
		out.add(k, m.MapIndex(k))
	}
	return out
}

func (m hashmap) entries() []keyVal {
	out := make([]keyVal, 0, len(m))
	for _, bin := range m {
		for _, entry := range bin {
			out = append(out, entry)
		}
	}
	return out
}

func (m hashmap) add(key, val reflect.Value) {
	h := hash(key)
	m[h] = append(m[h], keyVal{key, val, hash(val)})
}

type candidate struct {
	val  reflect.Value
	hash uint64
}

type candidates []candidate

func (c candidates) contains(v reflect.Value) bool {
	h := hash(v)
	s := sort.Search(len(c), func(i int) bool { return c[i].hash >= h })
	for _, t := range c[s:] {
		if t.hash != h {
			break
		}
		if DeepEqual(t.val.Interface(), v.Interface()) {
			return true
		}
	}
	return false
}

// Len is the number of elements in the collection.
func (c candidates) Len() int { return len(c) }

// Less reports whether the element with
// index i should sort before the element with index j.
func (c candidates) Less(i, j int) bool { return c[i].hash < c[j].hash }

// Swap swaps the elements with indexes i and j.
func (c candidates) Swap(i, j int) {
	c[i], c[j] = c[j], c[i]
}

func (m hashmap) get(key reflect.Value) candidates {
	k := key.Interface()
	bin := m[hash(key)]
	out := candidates{}
	for _, v := range bin {
		if DeepEqual(v.key.Interface(), k) {
			out = append(out, candidate{v.val, v.valHash})
		}
	}
	sort.Sort(out)
	return out
}

func hash(v reflect.Value) uint64 {
	h := hasher{}
	return h.hash(v)
}

type hasher struct {
	count int
}

var hashes = map[interface{}]uint64{}

func (h *hasher) hash(v reflect.Value) uint64 {
	switch v.Kind() {
	case reflect.Bool:
		if v.Bool() {
			return 1
		}
		return 0
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return uint64(v.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return v.Uint()
	case reflect.Uintptr:
		return uint64(v.Pointer())
	case reflect.Float32, reflect.Float64:
		return math.Float64bits(v.Float())
	case reflect.Complex64, reflect.Complex128:
		v := v.Complex()
		return math.Float64bits(real(v)) ^ math.Float64bits(imag(v))
	case reflect.Array, reflect.Slice:
		n := h.str(v.Type().Name())
		for i, c := 0, v.Len(); i < c; i++ {
			n = n*31 + h.hash(v.Index(i))
		}
		return n
	case reflect.Interface, reflect.Ptr:
		if v.IsNil() {
			return h.str(v.Type().Name())
		}
		if h, ok := hashes[v.Interface()]; ok {
			return h
		}
		hashes[v.Interface()] = 0
		n := h.hash(v.Elem())
		hashes[v.Interface()] = n
		return n
	case reflect.String:
		return h.str(v.String())
	case reflect.Struct:
		n := h.str(v.Type().Name())
		for i, c := 0, v.Type().NumField(); i < c; i++ {
			n = n*31 + h.hash(v.Field(i))
		}
		return n
	default:
		panic(fmt.Errorf("Cannot hash type '%v'", v.Type()))
	}
}

func (h hasher) str(s string) uint64 {
	if h, ok := hashes[s]; ok {
		return h
	}
	n := uint64(0)
	c := len(s)
	if c > 100 {
		c = 100
	}
	for _, r := range s {
		n = n*31 + uint64(r)
	}
	hashes[s] = n
	return n
}
