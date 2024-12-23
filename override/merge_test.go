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

package override

import (
	"testing"

	"github.com/compose-spec/compose-go/v2/utils"
	"gopkg.in/yaml.v3"
	"gotest.tools/v3/assert"
)

// override using the same logging driver will override driver options
func Test_mergeOverrides(t *testing.T) {
	right := `
services:
  test:
    image: foo
    scale: 1
`
	left := `
services:
  test:
    image: bar
    scale: 2
`
	expected := `
services:
  test:
    image: bar
    scale: 2
`

	got, err := Merge(unmarshal(t, right), unmarshal(t, left))
	assert.NilError(t, err)
	assert.DeepEqual(t, got, unmarshal(t, expected))
}

func Test_mergeOverridesWithSourceInfo(t *testing.T) {
	assertMergeYaml(t, `
services:
  test:
    image: foo
    scale: 1
`, `
services:
  test:
    image: bar
    scale: 2
`, `
services:
  test:
    image: !left bar
    scale: !left 2
`)
}

func assertMergeYamlOriginal(t *testing.T, right string, left string, want string) {
	t.Helper()
	got, err := Merge(unmarshal(t, right), unmarshal(t, left))
	assert.NilError(t, err)
	got, err = EnforceUnicity(got)
	assert.NilError(t, err)
	assert.DeepEqual(t, got, unmarshal(t, want))
}

func assertMergeYaml(t *testing.T, right string, left string, want string) {
	t.Helper()
	got, err := Merge(attachSourceInfo(t, "right", unmarshal(t, right)), attachSourceInfo(t, "left", unmarshal(t, left)))
	assert.NilError(t, err)
	got, err = EnforceUnicity(got)
	assert.NilError(t, err)
	assert.DeepEqual(t, got, unmarshal(t, want))
}

type sourceTagProcessor struct {
	target map[string]any
}

// UnmarshalYAML implement yaml.Unmarshaler
func (p *sourceTagProcessor) UnmarshalYAML(node *yaml.Node) error {
	target, err := p.resolveModelWithSourceTag(node)
	for k, v := range target.(map[string]any) {
		p.target[k] = v
	}
	return err
}

func (p *sourceTagProcessor) resolveModelWithSourceTag(node *yaml.Node) (any, error) {
	switch node.Tag {
	case "!right", "!left":
		source := node.Tag[1:]
		node.Tag = "" // Unset node tag so it does not interfere with decoding
		var value any
		if err := node.Decode(&value); err != nil {
			return nil, err
		}
		return utils.Pair{First: value, Second: source}, nil
	}
	switch node.Kind {
	case yaml.SequenceNode:
		values := []any{}
		for _, v := range node.Content {
			value, err := p.resolveModelWithSourceTag(v)
			if err != nil {
				return nil, err
			}
			values = append(values, value)
		}
		return values, nil
	case yaml.MappingNode:
		var key string
		values := make(map[string]any)
		for idx, v := range node.Content {
			if idx%2 == 0 {
				key = v.Value
			} else {
				resolved, err := p.resolveModelWithSourceTag(v)
				if err != nil {
					return nil, err
				}
				values[key] = resolved
			}
		}
		return values, nil
	default:
		var value any
		if err := node.Decode(&value); err != nil {
			return nil, err
		}
		return value, nil
	}
}

func unmarshal(t *testing.T, s string) map[string]any {
	t.Helper()
	val := map[string]any{}
	err := yaml.Unmarshal([]byte(s), &sourceTagProcessor{val})
	assert.NilError(t, err, s)
	return val
}

func attachSourceInfo(t *testing.T, sourceName string, source map[string]any) map[string]any {
	t.Helper()
	for k, v := range source {
		switch v := v.(type) {
		case map[string]any:
			source[k] = attachSourceInfo(t, sourceName, v)
		case []any:
			for i, item := range v {
				switch item := item.(type) {
				case map[string]any:
					v[i] = attachSourceInfo(t, sourceName, item)
				default:
					v[i] = utils.Pair{First: item, Second: sourceName}
				}
			}
		default:
			source[k] = utils.Pair{First: v, Second: sourceName}
		}
	}
	return source
}
