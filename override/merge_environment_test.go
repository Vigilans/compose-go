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

package override

import (
	"testing"
)

func Test_mergeYamlEnvironmentSequence(t *testing.T) {
	assertMergeYaml(t, `
services:
  test:
    image: foo
    environment:
      - FOO=BAR
`, `
services:
  test:
    environment:
      - QIX=ZOT
      - EMPTY=
      - NIL
`, `
services:
  test:
    image: !right foo
    environment:
      - !right FOO=BAR
      - !left  QIX=ZOT
      - !left  EMPTY=
      - !left  NIL
`)
}

func Test_mergeYamlEnvironmentMapping(t *testing.T) {
	assertMergeYaml(t, `
services:
  test:
    image: foo
    environment:
      FOO: BAR
`, `
services:
  test:
    environment:
      EMPTY: ""
      NIL: null
      QIX: ZOT
`, `
services:
  test:
    image: !right foo
    environment:
      - !right FOO=BAR
      - !left  EMPTY=
      - !left  NIL
      - !left  QIX=ZOT
`)
}

func Test_mergeYamlEnvironmentMixed(t *testing.T) {
	assertMergeYaml(t, `
services:
  test:
    image: foo
    environment:
      FOO: BAR
`, `
services:
  test:
    environment:
      - QIX=ZOT
`, `
services:
  test:
    image: !right foo
    environment:
      - !right FOO=BAR
      - !left  QIX=ZOT
`)
}

func Test_mergeYamlEnvironmentNumber(t *testing.T) {
	assertMergeYaml(t, `
services:
  test:
    environment:
      FOO: 1
`, `
services:
  test:
    environment:
      FOO: 3
`, `
services:
  test:
    environment:
      - !left FOO=3
`)
}
