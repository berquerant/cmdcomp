#!/bin/bash

set -e
set -o pipefail

readonly target="${1:-README.md}"
readonly bin="${2:-bin/cmdcomp}"

cat << 'HEADER' > "$target"
# cmdcomp

HEADER

"./$bin" --help >> "$target" 2>&1

cat << 'FOOTER' >> "$target"

## Install

``` shell
go install github.com/berquerant/cmdcomp/cmd/cmdcomp@latest
```
FOOTER
