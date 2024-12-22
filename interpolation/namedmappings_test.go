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

package interpolation

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/compose-spec/compose-go/v2/template"
	"github.com/compose-spec/compose-go/v2/tree"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

var labels = map[string]string{
	"com.docker.compose.container-number": "2",
	"org.opencontainers.image.ref.name":   "ubuntu",
	"org.opencontainers.image.version":    "22.04",
}

var secrets = map[string]string{
	"root_password": "testP@ssw0rd",
	"access_key":    "12345678-abcd-11ef-a236-d7497f4e9904",
}

func envMapping(name string) (string, bool) {
	if name == "FOO_2" {
		return "bar_2", true
	}
	return defaultMapping(name)
}

func labelsMapping(name string) (string, bool) {
	val, ok := labels[name]
	return val, ok
}

func secretMapping(name string) (string, bool) {
	val, ok := secrets[name]
	return val, ok
}

type numberNamedMappingsResolver struct{}

func (numberNamedMappingsResolver) Accept(path tree.Path) bool {
	return path.Matches(tree.NewPath("services", tree.PathMatchAll))
}

func (numberNamedMappingsResolver) Resolve(ctx context.Context, value interface{}, path tree.Path, opts Options) (template.NamedMappings, error) {
	return template.NamedMappings{
		"labels": template.ToVariadicMapping(func(key string) (string, bool) {
			switch key {
			case "com.docker.compose.container-number":
				if parts := strings.Split(path.Last(), "_"); len(parts) == 2 { // service_1 -> service, 1
					return parts[1], true
				}
			}
			return "", false
		}),
	}, nil
}

func TestInterpolateWithNamedMappings(t *testing.T) {
	namedMappings := map[tree.Path]template.NamedMappings{
		tree.NewPath(): { // global level
			"env":    template.ToVariadicMapping(envMapping),
			"labels": template.ToVariadicMapping(labelsMapping),
			"secret": template.ToVariadicMapping(secretMapping),
		},
	}
	testcases := []struct {
		test     string
		expected string
		errMsg   string
	}{
		{test: "{{{ ${env[USER]} ${env[FOO]} ${env[count]} }}}", expected: "{{{ jenny bar 5 }}}"},
		{test: "{{{ ${labels[com.docker.compose.container-number]} ${secret[root_password]} }}}", expected: "{{{ 2 testP@ssw0rd }}}"},

		{test: "{{{ ${env[FOO]:-foo_} }}}", expected: "{{{ bar }}}"},
		{test: "{{{ ${env[FOO]:-foo} ${env[BAR]:-DEFAULT_VALUE} }}}", expected: "{{{ bar DEFAULT_VALUE }}}"},
		{test: "{{{ ${env[BAR]} }}}", expected: "{{{  }}}"},
		{test: "${env[FOO]:-baz} }}}", expected: "bar }}}"},
		{test: "${env[FOO]-baz} }}}", expected: "bar }}}"},

		{test: "{{{ ${env[FOO_${labels[com.docker.compose.container-number]}]:-foo_} }}}", expected: "{{{ bar_2 }}}"},
		{test: "{{{ ${env[FOO_${labels[unset]}]:-foo_} }}}", expected: "{{{ foo_ }}}"},
		{test: "{{{ ${env[FOO_${labels[unset]:-2}]:-foo_} }}}", expected: "{{{ bar_2 }}}"},

		{test: "{{{ ${unset[FOO]:-foo_} }}}", errMsg: `named mapping not found: "unset"`},
		{test: "{{{ ${env[${unset[FOO]}]} }}}", errMsg: `named mapping not found: "unset"`},
		{test: "{{{ ${env[~invalid~key~]} }}}", errMsg: `invalid key in named mapping: "~invalid~key~"`},
		{test: "{{{ ${env[${secret[root_password]}]} }}}", errMsg: `invalid key in named mapping: "${secret[root_password]}" (resolved to "testP@ssw0rd")`},
		{test: "{{{ ${env[${secret[access_key]}]} }}}", expected: "{{{  }}}"},
	}

	getServiceConfig := func(val string) map[string]interface{} {
		if val == "" {
			return map[string]interface{}{}
		}
		return map[string]interface{}{
			"myservice": map[string]interface{}{
				"environment": map[string]interface{}{
					"TESTVAR": val,
				},
			},
		}
	}

	getFullErrorMsg := func(msg string) string {
		return fmt.Sprintf("error while interpolating myservice.environment.TESTVAR: %s", msg)
	}

	for _, testcase := range testcases {
		result, err := Interpolate(getServiceConfig(testcase.test), Options{NamedMappings: namedMappings})
		if testcase.errMsg != "" {
			assert.Assert(t, err != nil, fmt.Sprintf("This should result in an error %q", testcase.errMsg))
			assert.Equal(t, getFullErrorMsg(testcase.errMsg), err.Error())
		}
		assert.Check(t, is.DeepEqual(getServiceConfig(testcase.expected), result))
	}
}

func TestInterpolateWithScopedNamedMappings(t *testing.T) {
	namedMappings := map[tree.Path]template.NamedMappings{
		tree.NewPath(): { // Global level
			"env":    template.ToVariadicMapping(envMapping),
			"secret": template.ToVariadicMapping(secretMapping),
		},
		tree.NewPath("services", "service_1"): { // Per-service level
			"labels": template.ToVariadicMapping(func(key string) (string, bool) {
				if key == "com.docker.compose.container-number" {
					return "1", true
				}
				return labelsMapping(key)
			}),
		},
		tree.NewPath("services", "service_2"): {
			"labels": template.ToVariadicMapping(func(key string) (string, bool) {
				if key == "com.docker.compose.container-number" {
					return "2", true
				}
				return labelsMapping(key)
			}),
		},
	}
	model := map[string]interface{}{
		"services": map[string]interface{}{
			"service_1": map[string]interface{}{
				"environment": map[string]interface{}{
					"TESTVAR": "{{{ ${env[USER]} ${secret[root_password]} ${labels[org.opencontainers.image.version]} ${labels[com.docker.compose.container-number]} }}}",
				},
			},
			"service_2": map[string]interface{}{
				"environment": map[string]interface{}{
					"TESTVAR": "{{{ ${env[BAR]} ${secret[access_key]} ${labels[org.opencontainers.image.version]} ${labels[com.docker.compose.container-number]} }}}",
				},
			},
		},
	}
	expected := map[string]interface{}{
		"services": map[string]interface{}{
			"service_1": map[string]interface{}{
				"environment": map[string]interface{}{
					"TESTVAR": "{{{ jenny testP@ssw0rd 22.04 1 }}}",
				},
			},
			"service_2": map[string]interface{}{
				"environment": map[string]interface{}{
					"TESTVAR": "{{{  12345678-abcd-11ef-a236-d7497f4e9904 22.04 2 }}}",
				},
			},
		},
	}

	result, err := Interpolate(model, Options{NamedMappings: namedMappings})
	assert.NilError(t, err)
	assert.Check(t, is.DeepEqual(result, expected))
}

func TestInterpolateWithEnvNamedMappingsResolver(t *testing.T) {
	resolvers := []NamedMappingsResolver{
		EnvNamedMappingsResolver{},
	}
	model := map[string]interface{}{
		"services": map[string]interface{}{
			"service_1": map[string]interface{}{
				"environment": map[string]interface{}{
					"TESTVAR": "{{{ ${env[USER]} ${FOO} }}}",
				},
			},
			"service_2": map[string]interface{}{
				"environment": map[string]interface{}{
					"TESTVAR": "{{{ ${env[FOO]} ${USER} }}}",
				},
			},
		},
	}
	expected := map[string]interface{}{
		"services": map[string]interface{}{
			"service_1": map[string]interface{}{
				"environment": map[string]interface{}{
					"TESTVAR": "{{{ jenny bar }}}",
				},
			},
			"service_2": map[string]interface{}{
				"environment": map[string]interface{}{
					"TESTVAR": "{{{ bar jenny }}}",
				},
			},
		},
	}

	namedMappings, err := ResolveNamedMappings(context.Background(), model, Options{LookupValue: defaultMapping}, resolvers)
	assert.NilError(t, err)

	// Check envMapping is identical with defaultMapping
	envMapping := namedMappings[tree.NewPath()]["env"]
	for key, value := range defaults {
		result, ok := envMapping(key)
		assert.Check(t, ok)
		assert.Equal(t, value, result)
	}

	// Check interpolated result
	result, err := Interpolate(model, Options{LookupValue: defaultMapping, NamedMappings: namedMappings})
	assert.NilError(t, err)
	assert.Check(t, is.DeepEqual(result, expected))
}

func TestInterpolateWithMixedNamedMappingsAndResolvers(t *testing.T) {
	namedMappings := map[tree.Path]template.NamedMappings{
		tree.NewPath(): { // Pre-filled named mappings
			"secret": template.ToVariadicMapping(secretMapping),
			"labels": template.ToVariadicMapping(labelsMapping), // Serve as fallback for non container number labels
		},
	}
	resolvers := []NamedMappingsResolver{
		EnvNamedMappingsResolver{},
		numberNamedMappingsResolver{},
	}
	model := map[string]interface{}{
		"services": map[string]interface{}{
			"service_1": map[string]interface{}{
				"environment": map[string]interface{}{
					"TESTVAR": "{{{ ${env[USER]} ${secret[root_password]} ${labels[org.opencontainers.image.version]} ${labels[com.docker.compose.container-number]} }}}",
				},
			},
			"service_2": map[string]interface{}{
				"environment": map[string]interface{}{
					"TESTVAR": "{{{ ${env[BAR]} ${secret[access_key]} ${labels[org.opencontainers.image.version]} ${labels[com.docker.compose.container-number]} }}}",
				},
			},
		},
	}
	expected := map[string]interface{}{
		"services": map[string]interface{}{
			"service_1": map[string]interface{}{
				"environment": map[string]interface{}{
					"TESTVAR": "{{{ jenny testP@ssw0rd 22.04 1 }}}",
				},
			},
			"service_2": map[string]interface{}{
				"environment": map[string]interface{}{
					"TESTVAR": "{{{  12345678-abcd-11ef-a236-d7497f4e9904 22.04 2 }}}",
				},
			},
		},
	}

	namedMappings, err := ResolveNamedMappings(context.Background(), model, Options{LookupValue: defaultMapping, NamedMappings: namedMappings}, resolvers)
	assert.NilError(t, err)
	result, err := Interpolate(model, Options{LookupValue: defaultMapping, NamedMappings: namedMappings})
	assert.NilError(t, err)
	assert.Check(t, is.DeepEqual(result, expected))
}

func TestMergeNamedMappings(t *testing.T) {
	servicePath := tree.NewPath("services", "service_1")
	namedMappings1 := map[tree.Path]template.NamedMappings{
		tree.NewPath(): {
			"env": template.ToVariadicMapping(func(key string) (string, bool) {
				if key == "FOO" {
					return "first", true
				}
				return "", false
			}),
			"secret": template.ToVariadicMapping(func(key string) (string, bool) {
				if key == "access_key" {
					return "access_key_value", true
				}
				return "", false
			}),
		},
	}
	namedMappings2 := map[tree.Path]template.NamedMappings{
		tree.NewPath(): {
			"env": template.ToVariadicMapping(func(key string) (string, bool) {
				if key == "FOO" {
					return "first_shadowed", true
				}
				if key == "BAR" {
					return "second", true
				}
				return "", false
			}),
		},
		servicePath: {
			"labels": template.ToVariadicMapping(func(key string) (string, bool) {
				if key == "label" {
					return "value", true
				}
				return "", false
			}),
		},
	}
	mergedNamedMappings := MergeNamedMappings(namedMappings1, namedMappings2)
	serviceScopeNamedMappings := LookupNamedMappings(mergedNamedMappings, servicePath)

	result, ok := serviceScopeNamedMappings["env"]("FOO")
	assert.Check(t, is.Equal(ok, true))
	assert.Check(t, is.Equal(result, "first")) // namedMappings1 should take precedence
	result, ok = serviceScopeNamedMappings["env"]("BAR")
	assert.Check(t, is.Equal(ok, true))
	assert.Check(t, is.Equal(result, "second"))
	result, ok = serviceScopeNamedMappings["secret"]("access_key")
	assert.Check(t, is.Equal(ok, true))
	assert.Check(t, is.Equal(result, "access_key_value"))
	result, ok = serviceScopeNamedMappings["labels"]("label")
	assert.Check(t, is.Equal(ok, true))
	assert.Check(t, is.Equal(result, "value"))
}

func TestLookupNamedMappings(t *testing.T) {
	servicePath := tree.NewPath("services", "service_1")
	namedMappings := map[tree.Path]template.NamedMappings{
		tree.NewPath(): {
			"env": template.ToVariadicMapping(func(key string) (string, bool) {
				if key == "FOO" {
					return "first", true
				}
				if key == "BAR" {
					return "second", true
				}
				return "", false
			}),
			"labels": template.ToVariadicMapping(func(key string) (string, bool) {
				if key == "global-label" {
					return "global-value", true
				}
				if key == "service-label" {
					return "service-value-shadowed", true
				}
				return "", false
			}),
		},
		servicePath: {
			"labels": template.ToVariadicMapping(func(key string) (string, bool) {
				if key == "service-label" {
					return "service-value", true
				}
				return "", false
			}),
			"secret": template.ToVariadicMapping(func(key string) (string, bool) {
				if key == "access_key" {
					return "access_key_value", true
				}
				return "", false
			}),
		},
	}
	serviceScopeNamedMappings := LookupNamedMappings(namedMappings, servicePath)
	result, ok := serviceScopeNamedMappings["env"]("FOO")
	assert.Check(t, is.Equal(ok, true))
	assert.Check(t, is.Equal(result, "first"))
	result, ok = serviceScopeNamedMappings["env"]("BAR")
	assert.Check(t, is.Equal(ok, true))
	assert.Check(t, is.Equal(result, "second"))
	result, ok = serviceScopeNamedMappings["labels"]("global-label")
	assert.Check(t, is.Equal(ok, true))
	assert.Check(t, is.Equal(result, "global-value"))
	result, ok = serviceScopeNamedMappings["labels"]("service-label")
	assert.Check(t, is.Equal(ok, true))
	assert.Check(t, is.Equal(result, "service-value")) // Deeper scope should take precedence
	result, ok = serviceScopeNamedMappings["secret"]("access_key")
	assert.Check(t, is.Equal(ok, true))
	assert.Check(t, is.Equal(result, "access_key_value"))
}
