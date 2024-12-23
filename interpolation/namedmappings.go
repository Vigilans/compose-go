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

	"github.com/compose-spec/compose-go/v2/consts"
	"github.com/compose-spec/compose-go/v2/template"
	"github.com/compose-spec/compose-go/v2/tree"
)

type NamedMappingsResolver interface {
	// Accept returns true if the resolver can resolve named mappings for model's value at the given path
	Accept(path tree.Path) bool

	// Resolve resolves named mappings based on model's value at the given path
	Resolve(ctx context.Context, value interface{}, path tree.Path, opts Options) (template.NamedMappings, error)

	// ResolveGlobal resolves named mappings simply based on provided context, before any model value is available
	ResolveGlobal(ctx context.Context, opts Options) (template.NamedMappings, error)
}

type EnvNamedMappingsResolver struct{}

func (r EnvNamedMappingsResolver) Accept(path tree.Path) bool {
	return false // EnvNamedMappingsResolver does not resolve named mappings based on model's value
}

func (r EnvNamedMappingsResolver) Resolve(ctx context.Context, value interface{}, path tree.Path, opts Options) (template.NamedMappings, error) {
	return nil, nil
}

func (r EnvNamedMappingsResolver) ResolveGlobal(ctx context.Context, opts Options) (template.NamedMappings, error) {
	return template.NamedMappings{
		consts.HostEnvMapping: template.ToVariadicMapping(template.Mapping(opts.LookupValue)),
	}, nil
}

func ResolveNamedMappings(ctx context.Context, value interface{}, opts Options, resolvers []NamedMappingsResolver) (namedMappings map[tree.Path]template.NamedMappings, err error) {
	namedMappings, err = ResolveGlobalNamedMappings(ctx, opts, resolvers)
	if err != nil {
		return nil, err
	}
	opts.NamedMappings = namedMappings // All resolvers will use and update this same underlying named mappings

	for _, resolver := range resolvers {
		err := recursiveResolveNamedMappings(ctx, value, tree.NewPath(), opts, resolver, namedMappings)
		if err != nil {
			return nil, err
		}
	}

	return namedMappings, nil
}

func recursiveResolveNamedMappings(ctx context.Context, value interface{}, path tree.Path, opts Options, resolver NamedMappingsResolver, namedMappings map[tree.Path]template.NamedMappings) error {
	if resolver.Accept(path) {
		resolved, err := resolver.Resolve(ctx, value, path, opts)
		if err != nil {
			return err
		}
		if mappings, ok := namedMappings[path]; ok {
			namedMappings[path] = mappings.Merge(resolved) // The early resolved named mappings take priority
		} else {
			namedMappings[path] = resolved
		}
	}

	switch value := value.(type) {
	case map[string]interface{}:
		for key, elem := range value {
			err := recursiveResolveNamedMappings(ctx, elem, path.Next(key), opts, resolver, namedMappings)
			if err != nil {
				return err
			}
		}
	case []interface{}:
		for i, elem := range value {
			err := recursiveResolveNamedMappings(ctx, elem, path.NextIndex(i), opts, resolver, namedMappings)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func ResolveGlobalNamedMappings(ctx context.Context, opts Options, resolvers []NamedMappingsResolver) (namedMappings map[tree.Path]template.NamedMappings, err error) {
	namedMappings = map[tree.Path]template.NamedMappings{}

	// Deep copy namedMappings from opts as initial
	for path, mappings := range opts.NamedMappings {
		namedMappings[path] = template.NamedMappings{}
		for key, mapping := range mappings {
			namedMappings[path][key] = mapping
		}
	}
	opts.NamedMappings = namedMappings // All resolvers will use and update this same underlying named mappings

	globalPath := tree.NewPath()
	for _, resolver := range resolvers {
		resolved, err := resolver.ResolveGlobal(ctx, opts)
		if err != nil {
			return nil, err
		}
		namedMappings[globalPath] = namedMappings[globalPath].Merge(resolved) // The early resolved named mappings take priority
	}

	return namedMappings, nil
}

func MergeNamedMappings(namedMappings map[tree.Path]template.NamedMappings, other map[tree.Path]template.NamedMappings) map[tree.Path]template.NamedMappings {
	merged := map[tree.Path]template.NamedMappings{}

	// Deep copy base namedMappings as initial
	for path, mappings := range namedMappings {
		merged[path] = template.NamedMappings{}
		for key, mapping := range mappings {
			merged[path][key] = mapping
		}
	}

	for path, mappings := range other {
		if base, ok := merged[path]; ok {
			merged[path] = base.Merge(mappings)
		} else {
			merged[path] = mappings
		}
	}

	return merged
}

func LookupNamedMappings(namedMappings map[tree.Path]template.NamedMappings, path tree.Path) template.NamedMappings {
	prefixNamedMappings := template.NamedMappings{}
	currentPath := tree.NewPath()

	// Walk the named mappings tree to build the prefix named mappings
	if current, ok := namedMappings[currentPath]; ok {
		prefixNamedMappings = current // Initial global level mappings
	}
	for _, part := range path.Parts() {
		currentPath = currentPath.Next(part)
		if current, ok := namedMappings[currentPath]; ok {
			prefixNamedMappings = current.Merge(prefixNamedMappings) // Longer prefix match takes priority
		}
	}

	return prefixNamedMappings
}
