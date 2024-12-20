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

package transform

import (
	"fmt"
	"strings"

	"github.com/compose-spec/compose-go/v2/tree"
)

func transformSSH(data any, p tree.Path, _ bool) (any, error) {
	switch v := data.(type) {
	case map[string]any:
		return v, nil
	case []any:
		return convertIntoMapping(v, func(e any) (string, any, error) {
			s, ok := e.(string)
			if !ok {
				return "", nil, fmt.Errorf("invalid ssh key type %T", e)
			}
			id, path, ok := strings.Cut(s, "=")
			if !ok {
				if id != "default" {
					return "", nil, fmt.Errorf("invalid ssh key %q", s)
				}
				return id, nil, nil
			}
			return id, path, nil
		})
	default:
		return data, fmt.Errorf("%s: invalid type %T for ssh", p, v)
	}
}
