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

package loader

import (
	"testing"

	"github.com/compose-spec/compose-go/v2/tree"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestWrapAndUnwrapValue(t *testing.T) {
	path := tree.NewPath("services", "service_1")
	testCases := []struct {
		value interface{}
		model map[string]interface{}
	}{
		{
			value: "test",
			model: map[string]interface{}{"services": map[string]interface{}{"service_1": "test"}},
		},
		{
			value: map[string]interface{}{"key1": "value1", "key2": "value2"},
			model: map[string]interface{}{"services": map[string]interface{}{"service_1": map[string]interface{}{"key1": "value1", "key2": "value2"}}},
		},
		{
			value: nil,
			model: map[string]interface{}{"services": map[string]interface{}{"service_1": nil}},
		},
	}

	for _, tc := range testCases {
		model := wrapValueWithPath(path, tc.value)
		assert.Check(t, is.DeepEqual(model, tc.model))

		value := unwrapValueWithPath(path, model)
		assert.Check(t, is.DeepEqual(value, tc.value))
	}
}

func TestExtractValueSubset(t *testing.T) {
	testCases := []struct {
		value     map[string]interface{}
		subpathes []tree.Path
		expected  map[string]interface{}
	}{
		{
			value: map[string]interface{}{
				"labels": map[string]interface{}{
					"test-label": "test",
				},
				"x-test-field": "test",
			},
			subpathes: []tree.Path{tree.Path("labels")},
			expected: map[string]interface{}{
				"labels": map[string]interface{}{
					"test-label": "test",
				},
			},
		},
		{
			value: map[string]interface{}{
				"environment": map[string]interface{}{
					"TESTVAR": "test",
				},
				"env_file": []any{
					"example1.env",
				},
				"x-test-field": "test",
			},
			subpathes: []tree.Path{tree.Path("environment"), tree.Path("env_file")},
			expected: map[string]interface{}{
				"environment": map[string]interface{}{
					"TESTVAR": "test",
				},
				"env_file": []any{
					"example1.env",
				},
			},
		},
		{
			value: map[string]interface{}{
				"scale": 3,
				"deploy": map[string]interface{}{
					"replicas":     3,
					"x-test-field": "test",
				},
				"x-test-field": "test",
			},
			subpathes: []tree.Path{tree.Path("scale"), tree.NewPath("deploy", "replicas")},
			expected: map[string]interface{}{
				"scale": 3,
				"deploy": map[string]interface{}{
					"replicas": 3,
				},
			},
		},
		{
			value: map[string]interface{}{
				"environment":  nil,
				"x-test-field": "test",
			},
			subpathes: []tree.Path{tree.Path("environment"), tree.Path("env_file")},
			expected: map[string]interface{}{
				"environment": nil,
			},
		},
		{
			value: map[string]interface{}{
				"scale":        3,
				"deploy":       nil,
				"x-test-field": "test",
			},
			subpathes: []tree.Path{tree.Path("scale"), tree.NewPath("deploy", "replicas")},
			expected: map[string]interface{}{
				"scale": 3,
			},
		},
	}

	for _, tc := range testCases {
		subset := extractValueSubset(tc.value, tc.subpathes...)
		assert.Check(t, is.DeepEqual(subset, tc.expected))
	}
}
