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
