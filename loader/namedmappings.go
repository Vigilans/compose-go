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
		return parts[0] == "services"
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
