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

func TestMergeServiceDependsOn(t *testing.T) {
	assertMergeYaml(t, `
services:
  test:
    image: foo
    depends_on:
      - dependency1
      - dependency2
`, `
services:
  test:
    depends_on:
      dependency1:
        condition: service_healthy
      dependency3:
`, `
services:
  test:
    image: !right foo
    depends_on:
      dependency1:
        condition: !left service_healthy
        required: !right true
      dependency2:
        condition: !right service_started
        required: !right true
      dependency3: !left
`)
}
