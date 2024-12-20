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

func TestMergeCAPSequence(t *testing.T) {
	assertMergeYaml(t, `
services:
  test:
    image: foo
    cap_add:
      - CAP_BPF
      - CAP_CHOWN
    cap_drop:
      - NET_ADMIN
      - SYS_ADMIN
`, `
services:
  test:
    cap_add:
      - CAP_KILL
      - CAP_CHOWN
    cap_drop:
      - NET_ADMIN
      - CAP_FOWNER
`, `
services:
  test:
    image: !right foo
    cap_add:
      - !right CAP_BPF
      - !left  CAP_CHOWN
      - !left  CAP_KILL
    cap_drop:
      - !left  NET_ADMIN
      - !right SYS_ADMIN
      - !left  CAP_FOWNER
`)
}
