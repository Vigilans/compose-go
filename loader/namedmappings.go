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
	return false
}

func (r *modelNamedMappingsResolver) Resolve(ctx context.Context, value interface{}, path tree.Path, opts interp.Options) (template.NamedMappings, error) {
	scope := &modelNamedMappingsScope{
		ctx:          ctx,
		value:        value,
		path:         path,
		opts:         opts,
	}

	switch {
	case path.Matches(tree.NewPath()):
		return template.NamedMappings{
			consts.ProjectMapping: func(key string) (string, bool) { return r.projectMapping(scope, key) },
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
