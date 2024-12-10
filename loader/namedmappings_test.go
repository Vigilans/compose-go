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
	"context"
	"fmt"
	"os"
	"testing"

	interp "github.com/compose-spec/compose-go/v2/interpolation"
	"gopkg.in/yaml.v3"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

// Tests ProjectMapping
func TestModelNamedMappingsResolverWithProjectMapping(t *testing.T) {
	env := map[string]string{}
	model := map[string]interface{}{
		"name":         "test-project",
		"x-test-field": "${project[name]} ${project[working-dir]}",
	}
	expected := map[string]interface{}{
		"name":         "test-project",
		"x-test-field": fmt.Sprintf("test-project %s", os.Getenv("PWD")),
	}
	assertInterpolateModel(t, env, model, expected)
}

// Tests ServiceMapping/ImageMapping/ContainerMapping
func TestModelNamedMappingsResolverWithServiceMapping(t *testing.T) {
	env := map[string]string{
		"USER": "test-user",
		"PWD":  os.Getenv("PWD"),
	}
	model := map[string]interface{}{
		"services": map[string]interface{}{
			"service_1": map[string]interface{}{
				"image":          "image:${service[name]}",
				"container_name": "container.${service[name]}",
				"user":           "${env[USER]}",
				"working_dir":    "${env[PWD]}",
				"x-test-field-1": "${image[name]} ${container[name]} ${service[scale]}",
				"x-test-field-2": "${container[user]} ${container[working-dir]}",
			},
			"service_2": map[string]interface{}{
				"scale":        2,
				"x-test-field": "${service[scale]}",
			},
			"service_3": map[string]interface{}{
				"deploy":       map[string]interface{}{"replicas": 3},
				"x-test-field": "${service[scale]}",
			},
		},
	}
	expected := map[string]interface{}{
		"services": map[string]interface{}{
			"service_1": map[string]interface{}{
				"image":          "image:service_1",
				"container_name": "container.service_1",
				"user":           "test-user",
				"working_dir":    os.Getenv("PWD"),
				"x-test-field-1": "image:service_1 container.service_1 1",
				"x-test-field-2": fmt.Sprintf("test-user %s", os.Getenv("PWD")),
			},
			"service_2": map[string]interface{}{
				"scale":        2,
				"x-test-field": "2",
			},
			"service_3": map[string]interface{}{
				"deploy":       map[string]interface{}{"replicas": 3},
				"x-test-field": "3",
			},
		},
	}
	assertInterpolateModel(t, env, model, expected)
}

// Tests NetworkMapping
func TestModelNamedMappingsResolverWithNetworkMapping(t *testing.T) {
	env := map[string]string{
		"USER": "test-user",
		"PWD":  os.Getenv("PWD"),
		"TRUE": "true",
	}
	model := map[string]interface{}{
		"name": "test-project",
		"networks": map[string]interface{}{
			"network_1": map[string]interface{}{
				"name":         "${network[driver]}-network",
				"external":     "${env[TRUE]}",
				"driver":       "bridge",
				"x-test-field": "${network[name]} ${network[external]}",
			},
			"network_2": map[string]interface{}{
				"external":     "${env[TRUE]}",
				"x-test-field": "${network[name]} ${network[external]}",
			},
			"network_3": map[string]interface{}{
				"x-test-field": "${network[name]} ${network[external]}",
			},
		},
	}
	expected := map[string]interface{}{
		"name": "test-project",
		"networks": map[string]interface{}{
			"network_1": map[string]interface{}{
				"name":         "bridge-network",
				"external":     true,
				"driver":       "bridge",
				"x-test-field": "bridge-network true",
			},
			"network_2": map[string]interface{}{
				"external":     true,
				"x-test-field": "network_2 true",
			},
			"network_3": map[string]interface{}{
				"x-test-field": "test-project_network_3 false",
			},
		},
	}
	assertInterpolateModel(t, env, model, expected)
}

// Tests VolumeMapping
func TestModelNamedMappingsResolverWithVolumeMapping(t *testing.T) {
	env := map[string]string{
		"USER": "test-user",
		"PWD":  os.Getenv("PWD"),
		"TRUE": "true",
	}
	model := map[string]interface{}{
		"name": "test-project",
		"volumes": map[string]interface{}{
			"volume_1": map[string]interface{}{
				"name":         "${volume[driver]}-volume",
				"external":     "${env[TRUE]}",
				"driver":       "overlay",
				"x-test-field": "${volume[name]} ${volume[external]}",
			},
			"volume_2": map[string]interface{}{
				"external":     "${env[TRUE]}",
				"x-test-field": "${volume[name]} ${volume[external]}",
			},
			"volume_3": map[string]interface{}{
				"x-test-field": "${volume[name]} ${volume[external]}",
			},
		},
	}
	expected := map[string]interface{}{
		"name": "test-project",
		"volumes": map[string]interface{}{
			"volume_1": map[string]interface{}{
				"name":         "overlay-volume",
				"external":     true,
				"driver":       "overlay",
				"x-test-field": "overlay-volume true",
			},
			"volume_2": map[string]interface{}{
				"external":     true,
				"x-test-field": "volume_2 true",
			},
			"volume_3": map[string]interface{}{
				"x-test-field": "test-project_volume_3 false",
			},
		},
	}
	assertInterpolateModel(t, env, model, expected)
}

func TestModelNamedMappingsResolverWithCycledLookup(t *testing.T) {
	var testcases = []struct {
		model   map[string]interface{}
		errMsgs []string
	}{
		{ // Test image name references itself
			model: map[string]interface{}{
				"services": map[string]interface{}{
					"service_1": map[string]interface{}{
						"image": "image:${image[name]}",
					},
				},
			},
			errMsgs: []string{
				`error while interpolating services.service_1.image: failed to interpolate model: ` +
					`error while interpolating services.service_1.image: lookup cycle detected: image[name]`,
			},
		},
	}

	for _, tc := range testcases {
		modelYAML, err := yaml.Marshal(tc.model)
		assert.NilError(t, err)

		env := map[string]string{
			"USER":  "jenny",
			"FOO":   "bar",
			"count": "5",
		}
		configDetails := buildConfigDetails(string(modelYAML), env)
		opts := toOptions(&configDetails, nil)

		resolvers := []interp.NamedMappingsResolver{
			interp.EnvNamedMappingsResolver{},
			NewModelNamedMappingsResolver(configDetails, opts),
		}
		interpOpts := *opts.Interpolate
		namedMappings, err := interp.ResolveNamedMappings(context.Background(), tc.model, interpOpts, resolvers)
		assert.NilError(t, err)

		// Check interpolated result
		interpOpts.NamedMappings = namedMappings
		_, err = interp.Interpolate(tc.model, interpOpts)
		if len(tc.errMsgs) > 0 {
			assert.Assert(t, err != nil, "This should result in an error")
			assert.Check(t, is.Contains(tc.errMsgs, err.Error()))
		} else {
			assert.NilError(t, err)
		}
	}
}

func assertInterpolateModel(t *testing.T, env map[string]string, model map[string]any, expected map[string]any) {
	t.Helper()

	modelYAML, err := yaml.Marshal(model)
	assert.NilError(t, err)

	configDetails := buildConfigDetails(string(modelYAML), env)
	opts := toOptions(&configDetails, nil)
	assert.NilError(t, projectName(&configDetails, opts))

	ctx := context.Background()

	resolvers := []interp.NamedMappingsResolver{
		interp.EnvNamedMappingsResolver{},
		NewModelNamedMappingsResolver(configDetails, opts),
	}
	interpOpts := *opts.Interpolate
	namedMappings, err := interp.ResolveNamedMappings(ctx, model, interpOpts, resolvers)
	assert.NilError(t, err)

	// Check interpolated result
	interpOpts.NamedMappings = namedMappings
	result, err := interp.Interpolate(model, interpOpts)
	assert.NilError(t, err)
	assert.Check(t, is.DeepEqual(result, expected))
}
