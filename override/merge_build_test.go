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

func Test_mergeYamlBuildSequence(t *testing.T) {
	assertMergeYaml(t, `
services:
  test:
    build:
      context: .
      args:
        - FOO=BAR
      additional_contexts:
        - resources=/path/to/resources
        - app=docker-image://my-app:latest
      platform:
        - linux/amd64
      tags:
        - "myimage:mytag"
      extra_hosts:
        - "somehost=162.242.195.82"
        - "otherhost=50.31.209.229"
`, `
services:
  test:
    build:
      context: .
      args:
        - GIT_COMMIT=cdc3b19
        - EMPTY=
        - NIL
      additional_contexts:
        - source=https://github.com/myuser/project.git
      platform:
        - "linux/arm64"
        - "linux/amd64"
        - "unsupported/unsupported"
      tags:
        - "myimage:mytag"
        - "registry/username/myrepos:my-other-tag"
      extra_hosts:
        - "otherhost=50.31.209.230"
        - "myhostv6=::1"
`, `
services:
  test:
    build:
      context: !left .
      args:
        - !right FOO=BAR
        - !left GIT_COMMIT=cdc3b19
        - !left EMPTY=
        - !left NIL
      additional_contexts:
        - !right resources=/path/to/resources
        - !right app=docker-image://my-app:latest
        - !left  source=https://github.com/myuser/project.git
      platform:
        - !left "linux/amd64"
        - !left "linux/arm64"
        - !left "unsupported/unsupported"
      tags:
        - !left "myimage:mytag"
        - !left "registry/username/myrepos:my-other-tag"
      extra_hosts:
        - !right "somehost=162.242.195.82"
        - !right "otherhost=50.31.209.229"
        - !left  "otherhost=50.31.209.230"
        - !left  "myhostv6=::1"
`)
}

func Test_mergeYamlArgsMapping(t *testing.T) {
	assertMergeYaml(t, `
services:
  test:
    build:
      context: .
      args:
        FOO: BAR
      additional_contexts:
        source: https://github.com/myuser/project.git
`, `
services:
  test:
    build:
      context: .
      args:
        EMPTY: ""
        NIL: null
        QIX: ZOT
      additional_contexts:
        app: docker-image://my-app:latest
        resources: /path/to/resources
`, `
services:
  test:
    build:
      context: !left .
      args:
       - !right FOO=BAR
       - !left EMPTY=
       - !left NIL
       - !left QIX=ZOT
      additional_contexts:
        - !right source=https://github.com/myuser/project.git
        - !left  app=docker-image://my-app:latest
        - !left  resources=/path/to/resources
`)
}

func Test_mergeYamlArgsMixed(t *testing.T) {
	assertMergeYaml(t, `
services:
  test:
    build:
      context: .
      args:
        FOO: BAR
      additional_contexts:
        - resources=/path/to/resources
        - app=docker-image://my-app:latest
      platform: linux/amd64
      tags:
        - "myimage:mytag"
`, `
services:
  test:
    build:
      args:
        - QIX=ZOT
      additional_contexts:
        app: docker-image://new-app:latest
        source: https://github.com/myuser/project.git
      tags:
        - "registry/username/myrepos:my-other-tag"

`, `
services:
  test:
    build:
      context: !right .
      args:
        - !right FOO=BAR
        - !left  QIX=ZOT
      additional_contexts:
        - !right resources=/path/to/resources
        - !left  app=docker-image://new-app:latest
        - !left  source=https://github.com/myuser/project.git
      platform: !right linux/amd64
      tags:
        - !right "myimage:mytag"
        - !left  "registry/username/myrepos:my-other-tag"
`)
}

func Test_mergeYamlArgsNumber(t *testing.T) {
	assertMergeYaml(t, `
services:
  test:
    build:
      context: .
      args:
        FOO: 1
`, `
services:
  test:
    build:
      context: .
      args:
        FOO: 3
`, `
services:
  test:
    build:
      context: !left .
      args:
       - !left FOO=3
`)
}
