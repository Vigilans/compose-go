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

package transform

import (
	"strconv"

	"github.com/compose-spec/compose-go/v2/tree"
	"github.com/compose-spec/compose-go/v2/utils"
)

type transformFunc func(data any, p tree.Path, ignoreParseError bool) (any, error)

var transformers = map[tree.Path]transformFunc{}

func init() {
	transformers["services.*"] = transformService
	transformers["services.*.build.secrets.*"] = transformFileMount
	transformers["services.*.build.additional_contexts"] = transformKeyValue
	transformers["services.*.depends_on"] = transformDependsOn
	transformers["services.*.env_file"] = transformEnvFile
	transformers["services.*.extends"] = transformExtends
	transformers["services.*.networks"] = transformServiceNetworks
	transformers["services.*.volumes.*"] = transformVolumeMount
	transformers["services.*.dns"] = transformStringOrList
	transformers["services.*.devices.*"] = transformDeviceMapping
	transformers["services.*.secrets.*"] = transformFileMount
	transformers["services.*.configs.*"] = transformFileMount
	transformers["services.*.ports"] = transformPorts
	transformers["services.*.build"] = transformBuild
	transformers["services.*.build.ssh"] = transformSSH
	transformers["services.*.ulimits.*"] = transformUlimits
	transformers["services.*.build.ulimits.*"] = transformUlimits
	transformers["volumes.*"] = transformMaybeExternal
	transformers["networks.*"] = transformMaybeExternal
	transformers["secrets.*"] = transformMaybeExternal
	transformers["configs.*"] = transformMaybeExternal
	transformers["include.*"] = transformInclude
}

func transformStringOrList(data any, _ tree.Path, _ bool) (any, error) {
	switch t := data.(type) {
	case string:
		return []any{t}, nil
	default:
		return data, nil
	}
}

// Canonical transforms a compose model into canonical syntax
func Canonical(yaml map[string]any, ignoreParseError bool) (map[string]any, error) {
	canonical, err := transform(yaml, tree.NewPath(), ignoreParseError)
	if err != nil {
		return nil, err
	}
	return canonical.(map[string]any), nil
}

func transform(data any, p tree.Path, ignoreParseError bool) (any, error) {
	for pattern, transformer := range transformers {
		if p.Matches(pattern) {
			return utils.TransformPairWithError(data, func(data any) (any, error) {
				return transformer(data, p, ignoreParseError)
			})
		}
	}
	switch v := data.(type) {
	case map[string]any:
		a, err := transformMapping(v, p, ignoreParseError)
		if err != nil {
			return a, err
		}
		return v, nil
	case []any:
		a, err := transformSequence(v, p, ignoreParseError)
		if err != nil {
			return a, err
		}
		return v, nil
	default:
		return data, nil
	}
}

func transformSequence(v []any, p tree.Path, ignoreParseError bool) ([]any, error) {
	for i, e := range v {
		t, err := transform(e, p.Next("[]"), ignoreParseError)
		if err != nil {
			return nil, err
		}
		v[i] = t
	}
	return v, nil
}

func transformMapping(v map[string]any, p tree.Path, ignoreParseError bool) (map[string]any, error) {
	for k, e := range v {
		t, err := transform(e, p.Next(k), ignoreParseError)
		if err != nil {
			return nil, err
		}
		v[k] = t
	}
	return v, nil
}

func convertIntoSequence(value any, transformValue func(int, any) (any, error)) ([]any, error) {
	var values []any
	switch value := value.(type) {
	case []any:
		values = value
	case nil:
		values = nil
	default:
		values = []any{value}
	}

	for i, value := range values {
		var err error
		values[i], err = utils.TransformPairWithError(value, func(a any) (any, error) {
			return transformValue(i, a)
		})
		if err != nil {
			return nil, err
		}
	}
	return values, nil
}

func convertIntoMapping(value any, parseKeyValue func(any) (string, any, error)) (map[string]any, error) {
	switch value := value.(type) {
	case []any:
		converted := map[string]any{}
		for _, entry := range value {
			var key string
			var value any
			var err error
			value, err = utils.TransformPairWithError(entry, func(val any) (any, error) {
				key, val, err = parseKeyValue(val)
				return val, err
			})
			if err != nil {
				return nil, err
			}
			converted[key] = value
		}
		return converted, nil
	case map[string]any:
		return value, nil
	default:
		return nil, nil
	}
}

func setMappingValue(mapping map[string]any, key string, value any) {
	// Attach sibiling's context to new value if exists
	if _, pairValues := utils.UnwrapPairWithValues[any](mapping); len(pairValues) > 0 {
		for _, pairValue := range pairValues {
			mapping[key] = utils.WrapPairWithValue(value, pairValue)
			return
		}
	} else {
		mapping[key] = value
	}
}
