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
	"path/filepath"
	"strconv"

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
			consts.ServicesMapping:   template.ToVariadicMapping(func(keys ...string) (string, bool) { return r.crossRefMapping(scope, consts.ServiceMapping, keys...) }),
			consts.ContainersMapping: template.ToVariadicMapping(func(keys ...string) (string, bool) { return r.crossRefMapping(scope, consts.ContainerMapping, keys...) }),
			consts.NetworksMapping:   template.ToVariadicMapping(func(keys ...string) (string, bool) { return r.crossRefMapping(scope, consts.NetworkMapping, keys...) }),
			consts.VolumesMapping:    template.ToVariadicMapping(func(keys ...string) (string, bool) { return r.crossRefMapping(scope, consts.VolumeMapping, keys...) }),
			consts.ConfigsMapping:    template.ToVariadicMapping(func(keys ...string) (string, bool) { return r.crossRefMapping(scope, consts.ConfigMapping, keys...) }),
			consts.SecretsMapping:    template.ToVariadicMapping(func(keys ...string) (string, bool) { return r.crossRefMapping(scope, consts.SecretMapping, keys...) }),
		}, nil
	case path.Matches(tree.NewPath("services", tree.PathMatchAll)):
		return template.NamedMappings{
			consts.ServiceMapping:   template.ToVariadicMapping(func(keys ...string) (string, bool) { return r.serviceMapping(scope, keys...) }),
			consts.ImageMapping:     template.ToVariadicMapping(func(keys ...string) (string, bool) { return r.imageMapping(scope, keys...) }),
			consts.ContainerMapping: template.ToVariadicMapping(func(keys ...string) (string, bool) { return r.containerMapping(scope, keys...) }),
			consts.LabelsMapping:    template.ToVariadicMapping(func(keys ...string) (string, bool) { return r.labelsMapping(scope, consts.ServiceMapping, keys[0]) }),
		}, nil
	case path.Matches(tree.NewPath("networks", tree.PathMatchAll)):
		return template.NamedMappings{
			consts.NetworkMapping: template.ToVariadicMapping(func(keys ...string) (string, bool) { return r.networkMapping(scope, keys...) }),
			consts.LabelsMapping:  template.ToVariadicMapping(func(keys ...string) (string, bool) { return r.labelsMapping(scope, consts.NetworkMapping, keys[0]) }),
		}, nil
	case path.Matches(tree.NewPath("volumes", tree.PathMatchAll)):
		return template.NamedMappings{
			consts.VolumeMapping: template.ToVariadicMapping(func(keys ...string) (string, bool) { return r.volumeMapping(scope, keys...) }),
			consts.LabelsMapping: template.ToVariadicMapping(func(keys ...string) (string, bool) { return r.labelsMapping(scope, consts.VolumeMapping, keys[0]) }),
		}, nil
	case path.Matches(tree.NewPath("configs", tree.PathMatchAll)):
		return template.NamedMappings{
			consts.ConfigMapping: template.ToVariadicMapping(func(keys ...string) (string, bool) { return r.configMapping(scope, keys...) }),
			consts.LabelsMapping: template.ToVariadicMapping(func(keys ...string) (string, bool) { return r.labelsMapping(scope, consts.ConfigMapping, keys[0]) }),
		}, nil
	case path.Matches(tree.NewPath("secrets", tree.PathMatchAll)):
		return template.NamedMappings{
			consts.SecretMapping: template.ToVariadicMapping(func(keys ...string) (string, bool) { return r.secretMapping(scope, keys...) }),
			consts.LabelsMapping: template.ToVariadicMapping(func(keys ...string) (string, bool) { return r.labelsMapping(scope, consts.SecretMapping, keys[0]) }),
		}, nil
	default:
		return nil, fmt.Errorf("unsupported path for modelNamedMappingsResolver: %s", path)
	}
}

func (r *modelNamedMappingsResolver) ResolveGlobal(ctx context.Context, opts interp.Options) (template.NamedMappings, error) {
	scope := &modelNamedMappingsScope{
		ctx:          ctx,
		opts:         opts,
		caches:       map[string]map[string]*string{},
		cycleTracker: map[string]map[string]bool{},
	}
	return template.NamedMappings{
		consts.ProjectMapping: template.ToVariadicMapping(func(keys ...string) (string, bool) { return r.projectMapping(scope, keys...) }),
		consts.ComposeMapping: template.ToVariadicMapping(func(keys ...string) (string, bool) { return r.composeMapping(scope, keys...) }),
	}, nil
}

func (r *modelNamedMappingsResolver) projectMapping(scope *modelNamedMappingsScope, keys ...string) (string, bool) {
	switch keys[0] {
	case "name":
		if name, ok := scope.ctx.Value(consts.ProjectNameKey{}).(string); ok {
			return name, true
		}
		return r.opts.projectName, true
	case "working-dir": // TODO: working_dir or working-dir?
		if path, ok := scope.ctx.Value(consts.ProjectDirKey{}).(string); ok {
			return path, true
		}
		return r.configDetails.WorkingDir, true
	}
	return "", false
}

func (r *modelNamedMappingsResolver) composeMapping(scope *modelNamedMappingsScope, keys ...string) (string, bool) {
	switch keys[0] {
	case "root-dir":
		return r.configDetails.WorkingDir, true
	case "working-dir":
		if path, ok := scope.ctx.Value(consts.WorkingDirKey{}).(string); ok {
			return path, true
		}
		return r.configDetails.WorkingDir, true
	case "config-dir":
		return scope.cachedMapping(consts.ComposeMapping, keys[0], func() (string, bool) {
			if path, ok := scope.ctx.Value(consts.ComposeFileKey{}).(string); ok {
				for _, loader := range r.opts.ResourceLoaders {
					if loader.Accept(path) {
						path = utils.Must(loader.Load(scope.ctx, path))
						path = utils.Must(filepath.Abs(path))
						return filepath.Dir(path), true
					}
				}
			}
			return "", false
		})
	}
	return "", false
}

func (r *modelNamedMappingsResolver) serviceMapping(scope *modelNamedMappingsScope, keys ...string) (string, bool) {
	switch service := scope.value.(map[string]interface{}); keys[0] {
	case "name":
		return scope.path.Last(), true
	case "scale":
		return scope.cachedMapping(consts.ServiceMapping, keys[0], func() (string, bool) {
			model := extractValueSubset(service, tree.Path("scale"), tree.NewPath("deploy", "replicas"))
			model = utils.Must(interpolateWithPath(scope.path, model, scope.opts)).(map[string]interface{})
			config := &types.ServiceConfig{}
			utils.Must(config, Transform(model, config))
			return fmt.Sprintf("%v", config.GetScale()), true
		})
	case "containers":
		if len(keys) > 2 && keys[1] == "0" { // Return self as the only container, overriden by replicas logics
			return r.containerMapping(scope, keys[2:]...)
		}
	}
	if scale, ok := r.serviceMapping(scope, "scale"); ok {
		if utils.Must(strconv.Atoi(scale)) == 1 { // If scale is 1, `service` mapping can retrieve its only container's fields
			return r.containerMapping(scope, keys...)
		}
	}
	return "", false
}

func (r *modelNamedMappingsResolver) imageMapping(scope *modelNamedMappingsScope, keys ...string) (string, bool) {
	switch service := scope.value.(map[string]interface{}); keys[0] {
	case "name":
		return scope.cachedMapping(consts.ImageMapping, keys[0], func() (string, bool) {
			if value := service["image"]; value != nil {
				return utils.Must(interpolateWithPath(scope.path.Next("image"), value, scope.opts)).(string), true
			}
			return "", false
		})
	}
	return "", false
}

func (r *modelNamedMappingsResolver) containerMapping(scope *modelNamedMappingsScope, keys ...string) (string, bool) {
	switch service := scope.value.(map[string]interface{}); keys[0] {
	case "name":
		return scope.cachedMapping(consts.ContainerMapping, keys[0], func() (string, bool) {
			if value := service["container_name"]; value != nil {
				return utils.Must(interpolateWithPath(scope.path.Next("container_name"), value, scope.opts)).(string), true
			}
			return "", false
		})
	case "user":
		return scope.cachedMapping(consts.ContainerMapping, keys[0], func() (string, bool) {
			if value := service["user"]; value != nil {
				return utils.Must(interpolateWithPath(scope.path.Next("user"), value, scope.opts)).(string), true
			}
			return "", false
		})
	case "working-dir": // TODO: working_dir or working-dir?
		return scope.cachedMapping(consts.ContainerMapping, keys[0], func() (string, bool) {
			if value := service["working_dir"]; value != nil {
				return utils.Must(interpolateWithPath(scope.path.Next("working_dir"), value, scope.opts)).(string), true
			}
			return "", false
		})
	case "image":
		switch len(keys) {
		case 1:
			return r.imageMapping(scope, "name")
		default:
			return r.imageMapping(scope, keys[1:]...)
		}
	case "env":
		return r.containerEnvMapping(scope, keys[1])
	case "labels":
		return r.labelsMapping(scope, consts.ServiceMapping, keys[1])
	}
	return "", false
}

func (r *modelNamedMappingsResolver) networkMapping(scope *modelNamedMappingsScope, keys ...string) (string, bool) {
	if value, ok := r.resourceMapping(scope, consts.NetworkMapping, keys...); ok {
		return value, ok
	}
	switch network := scope.value.(map[string]interface{}); keys[0] {
	case "driver":
		return scope.cachedMapping(consts.VolumeMapping, keys[0], func() (string, bool) {
			if value := network["driver"]; value != nil {
				return utils.Must(interpolateWithPath(scope.path.Next("driver"), value, scope.opts)).(string), true
			}
			return "", false
		})
	}
	return "", false
}

func (r *modelNamedMappingsResolver) volumeMapping(scope *modelNamedMappingsScope, keys ...string) (string, bool) {
	if value, ok := r.resourceMapping(scope, consts.VolumeMapping, keys...); ok {
		return value, ok
	}
	switch volume := scope.value.(map[string]interface{}); keys[0] {
	case "driver":
		return scope.cachedMapping(consts.VolumeMapping, keys[0], func() (string, bool) {
			if value := volume["driver"]; value != nil {
				return utils.Must(interpolateWithPath(scope.path.Next("driver"), value, scope.opts)).(string), true
			}
			return "", false
		})
	}
	return "", false
}

func (r *modelNamedMappingsResolver) configMapping(scope *modelNamedMappingsScope, keys ...string) (string, bool) {
	if value, ok := r.resourceMapping(scope, consts.ConfigMapping, keys...); ok {
		return value, ok
	}
	return r.fileObjectMapping(scope, consts.ConfigMapping, keys[0])
}

func (r *modelNamedMappingsResolver) secretMapping(scope *modelNamedMappingsScope, keys ...string) (string, bool) {
	if value, ok := r.resourceMapping(scope, consts.ConfigMapping, keys...); ok {
		return value, ok
	}
	return r.fileObjectMapping(scope, consts.ConfigMapping, keys[0])
}

func (r *modelNamedMappingsResolver) resourceMapping(scope *modelNamedMappingsScope, name string, keys ...string) (string, bool) {
	switch resource := scope.value.(map[string]interface{}); keys[0] {
	case "name", "external":
		return scope.cachedMapping(name, keys[0], func() (string, bool) {
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
			if keys[0] == "name" {
				scope.caches[name]["external"] = &external
				return resourceName, true
			} else {
				scope.caches[name]["name"] = &resourceName
				return external, true
			}
		})
	case "labels":
		return r.labelsMapping(scope, name, keys[1])
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
				model = utils.Must(r.resolveRelativePaths(scope, model))
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
	return scope.cachedMapping("container[env]", key, func() (string, bool) {
		// Cache for holding all uninterpolated env variables
		uninterpolatedEnv, ok := scope.caches["uninterpolatedEnv"]
		if !ok {
			// Only use .environment and .env_file fields to forge uninterpolated env variables
			model := wrapValueWithPath(scope.path, extractValueSubset(scope.value, tree.NewPath("environment"), tree.NewPath("env_file")))

			// Apply Canonical to reuse `transformEnvFile` logic
			model = utils.Must(transform.Canonical(model, false))

			// Resolve env_file path to absolute
			model = utils.Must(r.resolveRelativePaths(scope, model))

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
			model = utils.Must(r.resolveRelativePaths(scope, model))

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

func (r *modelNamedMappingsResolver) crossRefMapping(scope *modelNamedMappingsScope, elementType string, keys ...string) (string, bool) {
	target := keys[0]
	args := keys[1:]

	var path tree.Path
	switch elementType {
	case consts.ServiceMapping, consts.ContainerMapping:
		path = tree.NewPath("services")
	case consts.NetworkMapping:
		path = tree.NewPath("networks")
	case consts.VolumeMapping:
		path = tree.NewPath("volumes")
	case consts.ConfigMapping:
		path = tree.NewPath("configs")
		if len(args) == 0 { // ${configs[name]} will directly return its data string
			args = []string{"data"}
		}
	case consts.SecretMapping:
		path = tree.NewPath("secrets")
		if len(args) == 0 { // ${secrets[name]} will directly return its data string
			args = []string{"data"}
		}
	}

	switch elementType {
	case consts.ServiceMapping, consts.NetworkMapping, consts.VolumeMapping, consts.ConfigMapping, consts.SecretMapping:
		// Try to match element by key first
		if mapping, ok := interp.LookupNamedMappings(scope.opts.NamedMappings, path.Next(target))[elementType]; ok {
			return mapping(args...)
		}

		// If not found, unwrap to services/networks/volumes/configs/secrets and try to match element by name
		if elements, ok := unwrapValueWithPath(path, scope.value.(map[string]interface{})).(map[string]interface{}); ok {
			for elementKey := range elements {
				path := path.Next(elementKey)
				if mapping, ok := interp.LookupNamedMappings(scope.opts.NamedMappings, path)[elementType]; ok {
					if name, ok := mapping("name"); ok && name == target {
						return mapping(args...)
					}
				}
			}
		}
	case consts.ContainerMapping:
		// Enumerate all containers in all services and try to match container by name
		if services, ok := unwrapValueWithPath(path, scope.value.(map[string]interface{})).(map[string]interface{}); ok {
			for serviceKey := range services {
				path := path.Next(serviceKey)
				if mapping, ok := interp.LookupNamedMappings(scope.opts.NamedMappings, path)[consts.ServiceMapping]; ok {
					if scale, ok := mapping("scale"); ok {
						scale := utils.Must(strconv.Atoi(scale))
						for i := 0; i < scale; i++ {
							if name, ok := mapping("containers", strconv.Itoa(i), "name"); ok && name == target {
								return mapping(append([]string{"containers", strconv.Itoa(i)}, args...)...)
							}
						}
					}
				}
			}
		}
	}
	return "", false
}

func (r *modelNamedMappingsResolver) modelToProject(model map[string]interface{}) (*types.Project, error) {
	opts := r.opts.clone()
	opts.SkipConsistencyCheck = true
	return modelToProject(model, opts, r.configDetails)
}

func (r *modelNamedMappingsResolver) resolveRelativePaths(scope *modelNamedMappingsScope, model map[string]interface{}) (map[string]interface{}, error) {
	model = utils.WrapPair(model, func(path tree.Path, _ any) any {
		if composeMapping, ok := interp.LookupNamedMappings(scope.opts.NamedMappings, path)[consts.ComposeMapping]; ok {
			if workingDir, ok := composeMapping("working-dir"); ok {
				return workingDir
			}
		}
		return nil // Will be omitted by `utils.UnwrapPairWithValues[string]`
	})
	model, workingDirMapping := utils.UnwrapPairWithValues[string](model)
	return model, paths.ResolveRelativePathsWithBaseMapping(model, r.configDetails.WorkingDir, r.opts.RemoteResourceFilters(), workingDirMapping)
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
