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

	"github.com/compose-spec/compose-go/v2/consts"
	interp "github.com/compose-spec/compose-go/v2/interpolation"
	"github.com/compose-spec/compose-go/v2/paths"
	"github.com/compose-spec/compose-go/v2/template"
	"github.com/compose-spec/compose-go/v2/transform"
	"github.com/compose-spec/compose-go/v2/tree"
	"github.com/compose-spec/compose-go/v2/types"
	"github.com/compose-spec/compose-go/v2/utils"
)

type modelNamedMappingsResolver struct {
	configDetails types.ConfigDetails
	opts          *Options
}

type modelNamedMappingsScope struct {
	ctx          context.Context
	value        interface{}
	path         tree.Path
	opts         interp.Options
	caches       map[string]map[string]*string // nil means key not present in this mapping
	cycleTracker map[string]map[string]bool
}

func NewModelNamedMappingsResolver(configDetails types.ConfigDetails, opts *Options) interp.NamedMappingsResolver {
	return &modelNamedMappingsResolver{
		configDetails: configDetails,
		opts:          opts,
	}
}

func (r *modelNamedMappingsResolver) Accept(path tree.Path) bool {
	if path == "" { // Global level
		return true
	}
	parts := path.Parts()
	if len(parts) == 2 {
		return parts[0] == "services" || parts[0] == "networks" || parts[0] == "volumes" || parts[0] == "configs" || parts[0] == "secrets"
	}
	return false
}

func (r *modelNamedMappingsResolver) Resolve(ctx context.Context, value interface{}, path tree.Path, opts interp.Options) (template.NamedMappings, error) {
	scope := &modelNamedMappingsScope{
		ctx:          ctx,
		value:        value,
		path:         path,
		opts:         opts,
		caches:       map[string]map[string]*string{},
		cycleTracker: map[string]map[string]bool{},
	}

	switch {
	case path.Matches(tree.NewPath()):
		return template.NamedMappings{
			consts.ProjectMapping: func(key string) (string, bool) { return r.projectMapping(scope, key) },
		}, nil
	case path.Matches(tree.NewPath("services", tree.PathMatchAll)):
		return template.NamedMappings{
			consts.ServiceMapping:      func(key string) (string, bool) { return r.serviceMapping(scope, key) },
			consts.ImageMapping:        func(key string) (string, bool) { return r.imageMapping(scope, key) },
			consts.ContainerMapping:    func(key string) (string, bool) { return r.containerMapping(scope, key) },
			consts.ContainerEnvMapping: func(key string) (string, bool) { return r.containerEnvMapping(scope, key) },
			consts.LabelsMapping:       func(key string) (string, bool) { return r.labelsMapping(scope, consts.ServiceMapping, key) },
		}, nil
	case path.Matches(tree.NewPath("networks", tree.PathMatchAll)):
		return template.NamedMappings{
			consts.NetworkMapping: func(key string) (string, bool) { return r.networkMapping(scope, key) },
			consts.LabelsMapping:  func(key string) (string, bool) { return r.labelsMapping(scope, consts.NetworkMapping, key) },
		}, nil
	case path.Matches(tree.NewPath("volumes", tree.PathMatchAll)):
		return template.NamedMappings{
			consts.VolumeMapping: func(key string) (string, bool) { return r.volumeMapping(scope, key) },
			consts.LabelsMapping: func(key string) (string, bool) { return r.labelsMapping(scope, consts.VolumeMapping, key) },
		}, nil
	case path.Matches(tree.NewPath("configs", tree.PathMatchAll)):
		return template.NamedMappings{
			consts.ConfigMapping: func(key string) (string, bool) { return r.configMapping(scope, key) },
			consts.LabelsMapping: func(key string) (string, bool) { return r.labelsMapping(scope, consts.ConfigMapping, key) },
		}, nil
	case path.Matches(tree.NewPath("secrets", tree.PathMatchAll)):
		return template.NamedMappings{
			consts.SecretMapping: func(key string) (string, bool) { return r.secretMapping(scope, key) },
			consts.LabelsMapping: func(key string) (string, bool) { return r.labelsMapping(scope, consts.SecretMapping, key) },
		}, nil
	default:
		return nil, fmt.Errorf("unsupported path for modelNamedMappingsResolver: %s", path)
	}
}

func (r *modelNamedMappingsResolver) projectMapping(_ *modelNamedMappingsScope, key string) (string, bool) {
	switch key {
	case "name":
		return r.opts.projectName, true
	case "working-dir": // TODO: working_dir or working-dir?
		return r.configDetails.WorkingDir, true
	}
	return "", false
}

func (r *modelNamedMappingsResolver) serviceMapping(scope *modelNamedMappingsScope, key string) (string, bool) {
	switch service := scope.value.(map[string]interface{}); key {
	case "name":
		return scope.path.Last(), true
	case "scale":
		return scope.cachedMapping(consts.ServiceMapping, key, func() (string, bool) {
			model := extractValueSubset(service, tree.Path("scale"), tree.NewPath("deploy", "replicas"))
			model = utils.Must(interpolateWithPath(scope.path, model, scope.opts)).(map[string]interface{})
			config := &types.ServiceConfig{}
			utils.Must(config, Transform(model, config))
			return fmt.Sprintf("%v", config.GetScale()), true
		})
	}

	return "", false
}

func (r *modelNamedMappingsResolver) imageMapping(scope *modelNamedMappingsScope, key string) (string, bool) {
	switch service := scope.value.(map[string]interface{}); key {
	case "name":
		return scope.cachedMapping(consts.ImageMapping, key, func() (string, bool) {
			if value := service["image"]; value != nil {
				return utils.Must(interpolateWithPath(scope.path.Next("image"), value, scope.opts)).(string), true
			}
			return "", false
		})
	}
	return "", false
}

func (r *modelNamedMappingsResolver) containerMapping(scope *modelNamedMappingsScope, key string) (string, bool) {
	switch service := scope.value.(map[string]interface{}); key {
	case "name":
		return scope.cachedMapping(consts.ContainerMapping, key, func() (string, bool) {
			if value := service["container_name"]; value != nil {
				return utils.Must(interpolateWithPath(scope.path.Next("container_name"), value, scope.opts)).(string), true
			}
			return "", false
		})
	case "user":
		return scope.cachedMapping(consts.ContainerMapping, key, func() (string, bool) {
			if value := service["user"]; value != nil {
				return utils.Must(interpolateWithPath(scope.path.Next("user"), value, scope.opts)).(string), true
			}
			return "", false
		})
	case "working-dir": // TODO: working_dir or working-dir?
		return scope.cachedMapping(consts.ContainerMapping, key, func() (string, bool) {
			if value := service["working_dir"]; value != nil {
				return utils.Must(interpolateWithPath(scope.path.Next("working_dir"), value, scope.opts)).(string), true
			}
			return "", false
		})
	}
	return "", false
}

func (r *modelNamedMappingsResolver) networkMapping(scope *modelNamedMappingsScope, key string) (string, bool) {
	if value, ok := r.resourceMapping(scope, consts.NetworkMapping, key); ok {
		return value, ok
	}
	switch network := scope.value.(map[string]interface{}); key {
	case "driver":
		return scope.cachedMapping(consts.VolumeMapping, key, func() (string, bool) {
			if value := network["driver"]; value != nil {
				return utils.Must(interpolateWithPath(scope.path.Next("driver"), value, scope.opts)).(string), true
			}
			return "", false
		})
	}
	return "", false
}

func (r *modelNamedMappingsResolver) volumeMapping(scope *modelNamedMappingsScope, key string) (string, bool) {
	if value, ok := r.resourceMapping(scope, consts.VolumeMapping, key); ok {
		return value, ok
	}
	switch volume := scope.value.(map[string]interface{}); key {
	case "driver":
		return scope.cachedMapping(consts.VolumeMapping, key, func() (string, bool) {
			if value := volume["driver"]; value != nil {
				return utils.Must(interpolateWithPath(scope.path.Next("driver"), value, scope.opts)).(string), true
			}
			return "", false
		})
	}
	return "", false
}

func (r *modelNamedMappingsResolver) configMapping(scope *modelNamedMappingsScope, key string) (string, bool) {
	if value, ok := r.resourceMapping(scope, consts.ConfigMapping, key); ok {
		return value, ok
	}
	return r.fileObjectMapping(scope, consts.ConfigMapping, key)
}

func (r *modelNamedMappingsResolver) secretMapping(scope *modelNamedMappingsScope, key string) (string, bool) {
	if value, ok := r.resourceMapping(scope, consts.ConfigMapping, key); ok {
		return value, ok
	}
	return r.fileObjectMapping(scope, consts.ConfigMapping, key)
}

func (r *modelNamedMappingsResolver) resourceMapping(scope *modelNamedMappingsScope, name string, key string) (string, bool) {
	switch resource := scope.value.(map[string]interface{}); key {
	case "name", "external":
		return scope.cachedMapping(name, key, func() (string, bool) {
			// Resource name requires `external` field to join resolution
			model := wrapValueWithPath(scope.path, extractValueSubset(resource, tree.NewPath("name"), tree.NewPath("external")))
			model["name"] = r.opts.projectName // Project name used for non-named non-external resource

			// Apply Canonical to reuse `transformMaybeExternal` logic
			model = utils.Must(transform.Canonical(model, false))

			// Interpoloate model (ensure `external` field is resolved)
			model = utils.Must(interp.Interpolate(model, scope.opts))

			// Apply `setNameFromKey` logic to set default name
			setNameFromKey(model)

			// Unwrap and extract fields
			resource = unwrapValueWithPath(scope.path, model).(map[string]interface{})
			resourceName := resource["name"].(string)
			external := fmt.Sprintf("%v", resource["external"] != nil && resource["external"].(bool))

			// Cache values and return
			if key == "name" {
				scope.caches[name]["external"] = &external
				return resourceName, true
			} else {
				scope.caches[name]["name"] = &resourceName
				return external, true
			}
		})
	}
	return "", false
}

func (r *modelNamedMappingsResolver) fileObjectMapping(scope *modelNamedMappingsScope, name string, key string) (string, bool) {
	switch fileObject := scope.value.(map[string]interface{}); key {
	case "file":
		return scope.cachedMapping(name, key, func() (string, bool) {
			if value := fileObject["file"]; value != nil {
				model := wrapValueWithPath(scope.path.Next("file"), value)
				model = utils.Must(interp.Interpolate(model, scope.opts))
				model = utils.Must(model, paths.ResolveRelativePaths(model, r.configDetails.WorkingDir, r.opts.RemoteResourceFilters()))
				return unwrapValueWithPath(scope.path.Next("file"), model).(string), true
			}
			return "", false
		})
	case "environment":
		return scope.cachedMapping(name, key, func() (string, bool) {
			if value := fileObject["environment"]; value != nil {
				return utils.Must(interpolateWithPath(scope.path.Next("environment"), value, scope.opts)).(string), true
			}
			return "", false
		})
	case "content":
		return scope.cachedMapping(name, key, func() (string, bool) {
			if value := fileObject["content"]; value != nil {
				return utils.Must(interpolateWithPath(scope.path.Next("content"), value, scope.opts)).(string), true
			}
			return "", false
		})
	case "data":
		return scope.cachedMapping(name, key, func() (string, bool) {
			if env, ok := r.fileObjectMapping(scope, name, "environment"); ok {
				return r.configDetails.Environment[env], true
			}
			if file, ok := r.fileObjectMapping(scope, name, "file"); ok {
				return string(utils.Must(os.ReadFile(file))), true
			}
			if content, ok := r.fileObjectMapping(scope, name, "content"); ok {
				return content, true
			}
			return "", false
		})
	}
	return "", false
}

func (r *modelNamedMappingsResolver) containerEnvMapping(scope *modelNamedMappingsScope, key string) (string, bool) {
	return scope.cachedMapping(consts.ContainerEnvMapping, key, func() (string, bool) {
		// Cache for holding all uninterpolated env variables
		uninterpolatedEnv, ok := scope.caches["uninterpolatedEnv"]
		if !ok {
			// Only use .environment and .env_file fields to forge uninterpolated env variables
			model := wrapValueWithPath(scope.path, extractValueSubset(scope.value, tree.NewPath("environment"), tree.NewPath("env_file")))

			// Apply Canonical to reuse `transformEnvFile` logic
			model = utils.Must(transform.Canonical(model, false))

			// Resolve env_file path to absolute
			utils.Must(model, paths.ResolveRelativePaths(model, r.configDetails.WorkingDir, r.opts.RemoteResourceFilters()))

			// Apply modelToProject to reuse `WithServicesEnvironmentResolved` logic
			project := utils.Must(r.modelToProject(model))

			// Fetch resolved `Environment` field as cache
			uninterpolatedEnv = project.Services[scope.path.Last()].Environment
			scope.caches["uninterpolatedEnv"] = uninterpolatedEnv
		}

		// We can early return if the key is not present in the mapping
		if value, ok := uninterpolatedEnv[key]; !ok || value == nil {
			return "", false
		}

		// Interpolate only one key-value pair here
		// So uninterpolated values in other fields won't affect
		return utils.Must(interpolateWithPath(scope.path.Next("environment").Next(key), *uninterpolatedEnv[key], scope.opts)).(string), true
	})
}

func (r *modelNamedMappingsResolver) labelsMapping(scope *modelNamedMappingsScope, elementType string, key string) (string, bool) {
	return scope.cachedMapping(consts.LabelsMapping, key, func() (string, bool) {
		// Cache for holding all uninterpolated labels
		uninterpolatedLabels, ok := scope.caches["uninterpolatedLabels"]
		if !ok {
			model := wrapValueWithPath(scope.path, extractValueSubset(scope.value, tree.NewPath("labels"), tree.NewPath("label_file")))

			// Apply Canonical to reuse `transformStringOrList` logic for label_file
			model = utils.Must(transform.Canonical(model, false))

			// Resolve label_file path to absolute
			utils.Must(model, paths.ResolveRelativePaths(model, r.configDetails.WorkingDir, r.opts.RemoteResourceFilters()))

			// Apply modelToProject to reuse `Labels.DecodeMapstructure` logic
			project := utils.Must(r.modelToProject(model))

			var labels types.Labels
			switch elementName := scope.path.Last(); elementType {
			case consts.ServiceMapping:
				labels = project.Services[elementName].Labels
			case consts.NetworkMapping:
				labels = project.Networks[elementName].Labels
			case consts.VolumeMapping:
				labels = project.Volumes[elementName].Labels
			case consts.ConfigMapping:
				labels = project.Configs[elementName].Labels
			case consts.SecretMapping:
				labels = project.Secrets[elementName].Labels
			default:
				panic(fmt.Errorf("unsupported element type in labelNamedMapping: %s", elementType))
			}
			uninterpolatedLabels = types.Mapping(labels).ToMappingWithEquals()
			scope.caches["uninterpolatedLabels"] = uninterpolatedLabels
		}

		// We can early return if the key is not present in the mapping
		if value, ok := uninterpolatedLabels[key]; !ok || value == nil {
			return "", false
		}

		// Interpolate only one key-value pair here
		// So uninterpolated values in other fields won't affect
		return utils.Must(interpolateWithPath(scope.path.Next("labels").Next(key), *uninterpolatedLabels[key], scope.opts)).(string), true
	})
}

func (r *modelNamedMappingsResolver) modelToProject(model map[string]interface{}) (*types.Project, error) {
	opts := r.opts.clone()
	opts.SkipConsistencyCheck = true
	return modelToProject(model, opts, r.configDetails)
}

func (s *modelNamedMappingsScope) cachedMapping(name string, key string, onCacheMiss func() (string, bool)) (string, bool) {
	cache, ok := s.caches[name]
	if !ok {
		cache = map[string]*string{}
		s.caches[name] = cache
	}

	if value, ok := cache[key]; ok { // Cache hit
		if value != nil {
			return *value, true
		} else {
			return "", false
		}
	}

	// Check for lookup cycle
	cycleTracker, ok := s.cycleTracker[name]
	if !ok {
		cycleTracker = map[string]bool{}
		s.cycleTracker[name] = cycleTracker
	}
	if cycleTracker[key] {
		panic(fmt.Errorf("lookup cycle detected: %s[%s]", name, key))
	}
	cycleTracker[key] = true
	defer func() { delete(cycleTracker, key) }()

	value, ok := onCacheMiss()
	if !ok {
		cache[key] = nil
		return "", false
	}

	cache[key] = &value
	return value, true
}
