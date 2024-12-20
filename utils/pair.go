/*
   Copyright 2020 The Compose Specification Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package utils

import (
	"strconv"

	"github.com/compose-spec/compose-go/v2/tree"
)

type Pair struct {
	First  any
	Second any
}

func WrapPair[T any | []any | map[string]any](target T, callback func(tree.Path, any) any) T {
	var nil T
	if target, ok := wrapPair(target, tree.NewPath(), callback).(T); ok {
		return target
	}
	return nil
}

func WrapPairWithValue[T any | []any | map[string]any](target T, value any) T {
	return WrapPair(target, func(_ tree.Path, _ any) any { return value })
}

func WrapPairWithValues[T any | []any | map[string]any, U any](target T, values map[tree.Path]U) T {
	return WrapPair(target, func(p tree.Path, _ any) any { return values[p] })
}

func wrapPair(value any, path tree.Path, callback func(tree.Path, any) any) any {
	switch value := value.(type) {
	case map[string]any:
		dict := make(map[string]any)
		for k, v := range value {
			dict[k] = wrapPair(v, path.Next(k), callback)
		}
		return dict
	case []any:
		list := make([]any, len(value))
		for i, v := range value {
			list[i] = wrapPair(v, path.Next(strconv.Itoa(i)), callback)
		}
		return list
	default:
		return Pair{First: value, Second: callback(path, value)}
	}
}

func UnwrapPair[T any | []any | map[string]any](target T, callbacks ...func(tree.Path, Pair)) T {
	var nil T
	if target, ok := unwrapPair(target, tree.NewPath(), callbacks...).(T); ok {
		return target
	}
	return nil
}

func UnwrapPairWithValues[V any, T any | []any | map[string]any](target T) (T, map[tree.Path]V) {
	values := map[tree.Path]V{}
	target = UnwrapPair(target, func(path tree.Path, pair Pair) {
		if value, ok := pair.Second.(V); ok {
			values[path] = value
		}
	})
	return target, values
}

func unwrapPair(value any, path tree.Path, callbacks ...func(tree.Path, Pair)) any {
	switch value := value.(type) {
	case map[string]any:
		dict := make(map[string]any)
		for k, v := range value {
			dict[k] = unwrapPair(v, path.Next(k), callbacks...)
		}
		return dict
	case []any:
		list := make([]any, len(value))
		for i, v := range value {
			list[i] = unwrapPair(v, path.Next(strconv.Itoa(i)), callbacks...)
		}
		return list
	case Pair:
		for _, callback := range callbacks {
			callback(path, value)
		}
		return value.First
	default:
		return value
	}
}

func TransformPair(target any, callback func(any) any) any {
	switch target := any(target).(type) {
	case Pair:
		newValue := callback(target.First)
		return WrapPairWithValue(newValue, target.Second) // Recursively wrap new value's fields with second value of the pair
	default:
		return callback(target)
	}
}

func TransformPairWithError(target any, callback func(any) (any, error)) (any, error) {
	switch target := any(target).(type) {
	case Pair:
		newValue, err := callback(target.First)
		if err != nil {
			return nil, err
		}
		return WrapPairWithValue(newValue, target.Second), nil
	default:
		return callback(target)
	}
}
