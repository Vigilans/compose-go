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
