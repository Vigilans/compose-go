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
	"testing"

	"github.com/compose-spec/compose-go/v2/tree"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestWrapPairWithValue(t *testing.T) {
	dict := map[string]any{
		"a": map[string]any{
			"b": "2",
			"c": map[string]any{
				"d": 3,
			},
		},
		"e": []any{
			map[string]any{
				"f": 3,
				"g": nil,
			},
			"4",
			nil,
		},
	}

	ctx := struct{}{}
	result := WrapPairWithValue(dict, ctx)

	assert.Check(t, is.DeepEqual(result, map[string]any{
		"a": map[string]any{
			"b": Pair{First: "2", Second: ctx},
			"c": map[string]any{
				"d": Pair{First: 3, Second: ctx},
			},
		},
		"e": []any{
			map[string]any{
				"f": Pair{First: 3, Second: ctx},
				"g": Pair{First: nil, Second: ctx},
			},
			Pair{First: "4", Second: ctx},
			Pair{First: nil, Second: ctx},
		},
	}))
}

func TestWrapPairWithValues(t *testing.T) {
	dict := map[string]any{
		"a": map[string]any{
			"b": "2",
			"c": map[string]any{
				"d": 3,
			},
		},
		"e": []any{
			map[string]any{
				"f": 3,
				"g": nil,
			},
			"4",
			nil,
		},
	}

	contexts := map[tree.Path]any{
		tree.NewPath("a", "b"):      "ctx1",
		tree.NewPath("a", "c", "d"): "ctx1",
		tree.NewPath("e", "0", "f"): "ctx2",
		tree.NewPath("e", "0", "g"): "ctx2",
		tree.NewPath("e", "1"):      "ctx3",
		tree.NewPath("e", "2"):      "ctx4",
	}
	result := WrapPairWithValues(dict, contexts)

	assert.Check(t, is.DeepEqual(result, map[string]any{
		"a": map[string]any{
			"b": Pair{First: "2", Second: "ctx1"},
			"c": map[string]any{
				"d": Pair{First: 3, Second: "ctx1"},
			},
		},
		"e": []any{
			map[string]any{
				"f": Pair{First: 3, Second: "ctx2"},
				"g": Pair{First: nil, Second: "ctx2"},
			},
			Pair{First: "4", Second: "ctx3"},
			Pair{First: nil, Second: "ctx4"},
		},
	}))
}

func TestUnwrapPair(t *testing.T) {
	dict := map[string]any{
		"a": map[string]any{
			"b": Pair{First: "2", Second: "ctx1"},
			"c": map[string]any{
				"d": Pair{First: 3, Second: "ctx1"},
			},
		},
		"e": []any{
			map[string]any{
				"f": Pair{First: 3, Second: "ctx2"},
				"g": Pair{First: nil, Second: "ctx2"},
			},
			Pair{First: "4", Second: "ctx3"},
			Pair{First: nil, Second: "ctx4"},
		},
	}

	result := UnwrapPair(dict)
	assert.Check(t, is.DeepEqual(result, map[string]any{
		"a": map[string]any{
			"b": "2",
			"c": map[string]any{
				"d": 3,
			},
		},
		"e": []any{
			map[string]any{
				"f": 3,
				"g": nil,
			},
			"4",
			nil,
		},
	}))
}

func TestUnwrapPairWithValues(t *testing.T) {
	dict := map[string]any{
		"a": map[string]any{
			"b": Pair{First: "2", Second: "ctx1"},
			"c": map[string]any{
				"d": Pair{First: 3, Second: "ctx1"},
			},
		},
		"e": []any{
			map[string]any{
				"f": Pair{First: 3, Second: "ctx2"},
				"g": Pair{First: nil, Second: "ctx2"},
			},
			Pair{First: "4", Second: "ctx3"},
			Pair{First: nil, Second: "ctx4"},
		},
	}

	result, contexts := UnwrapPairWithValues[string](dict)
	assert.Check(t, is.DeepEqual(result, map[string]any{
		"a": map[string]any{
			"b": "2",
			"c": map[string]any{
				"d": 3,
			},
		},
		"e": []any{
			map[string]any{
				"f": 3,
				"g": nil,
			},
			"4",
			nil,
		},
	}))
	assert.Check(t, is.DeepEqual(contexts, map[tree.Path]string{
		tree.NewPath("a", "b"):      "ctx1",
		tree.NewPath("a", "c", "d"): "ctx1",
		tree.NewPath("e", "0", "f"): "ctx2",
		tree.NewPath("e", "0", "g"): "ctx2",
		tree.NewPath("e", "1"):      "ctx3",
		tree.NewPath("e", "2"):      "ctx4",
	}))
}

func TestTransformPair(t *testing.T) {
	transform := func(value any) any {
		switch value := value.(type) {
		case int:
			return value + 1
		case string:
			return map[string]any{
				"a": value,
				"b": value,
			}
		default:
			return value
		}
	}
	testCases := []struct {
		testCase any
		expected any
	}{
		{
			testCase: nil,
			expected: nil,
		},
		{
			testCase: 1,
			expected: 2,
		},
		{
			testCase: "value",
			expected: map[string]any{
				"a": "value",
				"b": "value",
			},
		},
		{
			testCase: Pair{First: 1, Second: "ctx1"},
			expected: Pair{First: 2, Second: "ctx1"},
		},
		{
			testCase: Pair{First: "value", Second: "ctx1"},
			expected: map[string]any{ // Wrapped recursively
				"a": Pair{First: "value", Second: "ctx1"},
				"b": Pair{First: "value", Second: "ctx1"},
			},
		},
		{
			testCase: map[string]any{"key": "value"},
			expected: map[string]any{"key": "value"},
		},
	}

	for _, tc := range testCases {
		result := TransformPair(tc.testCase, transform)
		assert.Check(t, is.DeepEqual(result, tc.expected))
	}
}
