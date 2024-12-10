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

// Tests ConfigMapping/SecretMapping
func TestModelNamedMappingsResolverWithConfigAndSecretMapping(t *testing.T) {
	env := map[string]string{
		"USER": "test-user",
		"PWD":  os.Getenv("PWD"),
	}
	model := map[string]interface{}{
		"name": "test-project",
		"configs": map[string]interface{}{
			"config_1": map[string]interface{}{
				"name":         "${config[content]}-config",
				"content":      "test",
				"x-test-field": "${config[name]} ${config[content]} ${config[data]}",
			},
			"config_2": map[string]interface{}{
				"environment":  "USER",
				"x-test-field": "${config[name]} ${config[environment]} ${config[data]}",
			},
			"config_3": map[string]interface{}{
				"external":     true,
				"file":         "testdata/file/user.txt",
				"x-test-field": "${config[name]} ${config[file]} ${config[data]}",
			},
		},
		"secrets": map[string]interface{}{
			"secret_1": map[string]interface{}{
				"content":      "test",
				"x-test-field": "${secret[data]}",
			},
			"secret_2": map[string]interface{}{
				"environment":  "PWD",
				"x-test-field": "${secret[data]}",
			},
			"secret_3": map[string]interface{}{
				"file":         "testdata/file/access_key.txt",
				"x-test-field": "${secret[data]}",
			},
		},
	}
	expected := map[string]interface{}{
		"name": "test-project",
		"configs": map[string]interface{}{
			"config_1": map[string]interface{}{
				"name":         "test-config",
				"content":      "test",
				"x-test-field": "test-config test test",
			},
			"config_2": map[string]interface{}{
				"environment":  "USER",
				"x-test-field": "test-project_config_2 USER test-user",
			},
			"config_3": map[string]interface{}{
				"external":     true,
				"file":         "testdata/file/user.txt",
				"x-test-field": fmt.Sprintf("config_3 %s/testdata/file/user.txt test-user", os.Getenv("PWD")),
			},
		},
		"secrets": map[string]interface{}{
			"secret_1": map[string]interface{}{
				"content":      "test",
				"x-test-field": "test",
			},
			"secret_2": map[string]interface{}{
				"environment":  "PWD",
				"x-test-field": os.Getenv("PWD"),
			},
			"secret_3": map[string]interface{}{
				"file":         "testdata/file/access_key.txt",
				"x-test-field": "12345678-abcd-11ef-a236-d7497f4e9904",
			},
		},
	}
	assertInterpolateModel(t, env, model, expected)
}

// Tests LabelsMapping/ContainerEnvMapping
func TestModelNamedMappingsResolverWithLabelsAndContainerEnvMapping(t *testing.T) {
	env := map[string]string{
		"USER":  "jenny",
		"FOO":   "bar",
		"count": "5",
	}
	model := map[string]interface{}{
		"services": map[string]interface{}{
			"service_0": map[string]interface{}{ // Nil environment and label field should not cause error
				"container_name": "service-${containerEnv[NUMBER]:-0}${labels[com.docker.compose.container-number]}",
			},
			"service_1": map[string]interface{}{ // Container name number <- containerEnv
				"container_name": "service-${containerEnv[NUMBER]}",
				"environment": map[string]interface{}{
					"TESTVAR": "{{{ ${env[USER]} ${FOO} ${containerEnv[NUMBER]} ${containerEnv[NONEXIST]} }}}",
					"NUMBER":  "1",
				},
			},
			"service_2": map[string]interface{}{ // Container name number <- containerEnv <- label
				"container_name": "service-${containerEnv[NUMBER]}",
				"environment": map[string]interface{}{
					"TESTVAR": "{{{ ${env[${labels[user]}]} ${FOO} ${containerEnv[NUMBER]} ${containerEnv[NONEXIST]} }}}",
					"NUMBER":  "${labels[com.docker.compose.container-number]}",
				},
				"labels": map[string]interface{}{
					"com.docker.compose.container-number": "2",
					"user":                                "USER",
				},
			},
			"service_3": map[string]interface{}{ // Container name number <- containerEnv <- label (key from env_file)
				"container_name": "service-${containerEnv[NUMBER]}",
				"environment": map[string]interface{}{
					"TESTVAR": "{{{ ${env[${labels[user]}]} ${FOO} ${containerEnv[NUMBER]} ${containerEnv[NONEXIST]} }}}",
					"NUMBER":  "${labels[${containerEnv[FOO]}]}",
				},
				"labels": map[string]interface{}{
					"com.docker.compose.container-number": "2",
					"user":                                "USER",
					"foo_from_env_file":                   "3",
				},
				"env_file": []any{
					"example1.env",
				},
			},
		},
		"networks": map[string]interface{}{
			"network_1": map[string]interface{}{
				"name": "network-${labels[com.docker.compose.network-number]}",
				"labels": map[string]interface{}{
					"com.docker.compose.network-number": "1",
				},
			},
		},
		"volumes": map[string]interface{}{
			"volume_1": map[string]interface{}{
				"name": "volume-${labels[com.docker.compose.volume-number]}",
				"labels": map[string]interface{}{
					"com.docker.compose.volume-number": "1",
				},
			},
		},
		"configs": map[string]interface{}{
			"config_1": map[string]interface{}{
				"name": "config-${labels[com.docker.compose.config-number]}",
				"labels": map[string]interface{}{
					"com.docker.compose.config-number": "1",
				},
			},
		},
		"secrets": map[string]interface{}{
			"secret_1": map[string]interface{}{
				"name": "secret-${labels[com.docker.compose.secret-number]}",
				"labels": map[string]interface{}{
					"com.docker.compose.secret-number": "1",
				},
			},
		},
	}
	expected := map[string]interface{}{
		"services": map[string]interface{}{
			"service_0": map[string]interface{}{ // Nil environment and label field should not cause error
				"container_name": "service-0",
			},
			"service_1": map[string]interface{}{
				"container_name": "service-1",
				"environment": map[string]interface{}{
					"TESTVAR": "{{{ jenny bar 1  }}}",
					"NUMBER":  "1",
				},
			},
			"service_2": map[string]interface{}{
				"container_name": "service-2",
				"environment": map[string]interface{}{
					"TESTVAR": "{{{ jenny bar 2  }}}",
					"NUMBER":  "2",
				},
				"labels": map[string]interface{}{
					"com.docker.compose.container-number": "2",
					"user":                                "USER",
				},
			},
			"service_3": map[string]interface{}{
				"container_name": "service-3",
				"environment": map[string]interface{}{
					"TESTVAR": "{{{ jenny bar 3  }}}",
					"NUMBER":  "3",
				},
				"labels": map[string]interface{}{
					"com.docker.compose.container-number": "2",
					"user":                                "USER",
					"foo_from_env_file":                   "3",
				},
				"env_file": []any{
					"example1.env",
				},
			},
		},
		"networks": map[string]interface{}{
			"network_1": map[string]interface{}{
				"name": "network-1",
				"labels": map[string]interface{}{
					"com.docker.compose.network-number": "1",
				},
			},
		},
		"volumes": map[string]interface{}{
			"volume_1": map[string]interface{}{
				"name": "volume-1",
				"labels": map[string]interface{}{
					"com.docker.compose.volume-number": "1",
				},
			},
		},
		"configs": map[string]interface{}{
			"config_1": map[string]interface{}{
				"name": "config-1",
				"labels": map[string]interface{}{
					"com.docker.compose.config-number": "1",
				},
			},
		},
		"secrets": map[string]interface{}{
			"secret_1": map[string]interface{}{
				"name": "secret-1",
				"labels": map[string]interface{}{
					"com.docker.compose.secret-number": "1",
				},
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
		{ // Test var references itself
			model: map[string]interface{}{
				"services": map[string]interface{}{
					"service_1": map[string]interface{}{
						"environment": map[string]interface{}{
							"TESTVAR": "{{{ ${containerEnv[TESTVAR]} }}}",
						},
					},
				},
			},
			errMsgs: []string{
				`error while interpolating services.service_1.environment.TESTVAR: failed to interpolate model: ` +
					`error while interpolating services.service_1.environment.TESTVAR: lookup cycle detected: containerEnv[TESTVAR]`,
			},
		},
		{ // Test var references other var that references itself
			model: map[string]interface{}{
				"services": map[string]interface{}{
					"service_1": map[string]interface{}{
						"environment": map[string]interface{}{
							"TESTVAR":  "{{{ ${containerEnv[OTHERVAR]} }}}",
							"OTHERVAR": "{{{ ${containerEnv[TESTVAR]} }}}",
						},
					},
				},
			},
			errMsgs: []string{
				`error while interpolating services.service_1.environment.TESTVAR: failed to interpolate model: ` +
					`error while interpolating services.service_1.environment.OTHERVAR: failed to interpolate model: ` +
					`error while interpolating services.service_1.environment.TESTVAR: lookup cycle detected: containerEnv[OTHERVAR]`,
				`error while interpolating services.service_1.environment.OTHERVAR: failed to interpolate model: ` +
					`error while interpolating services.service_1.environment.TESTVAR: failed to interpolate model: ` +
					`error while interpolating services.service_1.environment.OTHERVAR: lookup cycle detected: containerEnv[TESTVAR]`,
			},
		},
		{ // Test var references other var that references itself with label
			model: map[string]interface{}{
				"services": map[string]interface{}{
					"service_1": map[string]interface{}{
						"environment": map[string]interface{}{
							"TESTVAR": "{{{ ${labels[OTHERLABEL]} }}}",
						},
						"labels": map[string]interface{}{
							"OTHERLABEL": "{{{ ${containerEnv[TESTVAR]} }}}",
						},
					},
				},
			},
			errMsgs: []string{
				`error while interpolating services.service_1.environment.TESTVAR: failed to interpolate model: ` +
					`error while interpolating services.service_1.labels.OTHERLABEL: failed to interpolate model: ` +
					`error while interpolating services.service_1.environment.TESTVAR: lookup cycle detected: labels[OTHERLABEL]`,
				`error while interpolating services.service_1.labels.OTHERLABEL: failed to interpolate model: ` +
					`error while interpolating services.service_1.environment.TESTVAR: failed to interpolate model: ` +
					`error while interpolating services.service_1.labels.OTHERLABEL: lookup cycle detected: containerEnv[TESTVAR]`,
			},
		},
		{ // Test var (env var) references test var (label), same key should not result in error
			model: map[string]interface{}{
				"services": map[string]interface{}{
					"service_1": map[string]interface{}{
						"environment": map[string]interface{}{
							"TESTVAR": "{{{ ${labels[TESTVAR]} }}}",
						},
						"labels": map[string]interface{}{
							"TESTVAR": "test",
						},
					},
				},
			},
			errMsgs: []string{},
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
