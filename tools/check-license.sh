#!/bin/sh
# May the pinned Pangolin CLI be redistributed in this module? Stops the build when not.
#   sh tools/check-license.sh <cli-source-dir>
#
# fosrl/cli is mixed-licensed (its LICENSE, 2026-10-26): files with a "Fossorial Commercial
# License" header must not be copied without a contract, files without a header are AGPL-3.
# So for the CLI source and every fosrl module its go.mod pulls in (newt, olm — their code is
# in the binary):
#   - no Go file may carry the commercial header,
#   - the CLI's LICENSE must still say that files without a header are AGPL-3,
#   - each fosrl module's LICENSE must be the AGPL-3.
# Measured for 0.19.0 (newt v1.19.0, olm v1.10.1): 0 of 131 CLI, 0 of 118 newt and 0 of 74 olm
# Go files with the commercial header.
set -eu
SRC="${1:?cli-source-dir}"
[ -f "$SRC/go.mod" ] || { echo "check-license: no go.mod in $SRC" >&2; exit 2; }
bad=0
count() { grep -rl 'Fossorial Commercial License' --include='*.go' "$1" | wc -l; }

n=$(count "$SRC"); total=$(find "$SRC" -name '*.go' | wc -l)
echo "check-license: fosrl/cli: $n of $total Go files with the commercial header"
[ "$n" -eq 0 ] || { grep -rl 'Fossorial Commercial License' --include='*.go' "$SRC" >&2; bad=1; }
if ! tr '\n' ' ' < "$SRC/LICENSE" | tr -s ' ' | grep -q 'All files without a license header are, by default, licensed under the GNU Affero General Public License'; then
  echo "check-license: fosrl/cli LICENSE no longer says that files without a header are AGPL-3" >&2; bad=1
fi

# The fosrl modules of the CLI's go.mod, from the module cache.
mods=$(cd "$SRC" && GOFLAGS=-mod=mod go list -m -f '{{if not .Main}}{{.Path}}@{{.Version}}{{end}}' all | grep '^github.com/fosrl/' || true)
[ -n "$mods" ] || { echo "check-license: no fosrl modules found in $SRC/go.mod" >&2; bad=1; }
for m in $mods; do
  dir=$(cd "$SRC" && go mod download -json "$m" | sed -n 's/^[[:space:]]*"Dir": "\(.*\)",$/\1/p')
  [ -d "$dir" ] || { echo "check-license: $m not downloaded" >&2; bad=1; continue; }
  n=$(count "$dir"); total=$(find "$dir" -name '*.go' | wc -l)
  echo "check-license: $m: $n of $total Go files with the commercial header"
  [ "$n" -eq 0 ] || bad=1
  head -n 3 "$dir/LICENSE" | grep -q 'GNU AFFERO GENERAL PUBLIC LICENSE' || { echo "check-license: $m LICENSE is not the AGPL-3" >&2; bad=1; }
done
[ "$bad" -eq 0 ] && echo "check-license: ok, AGPL-3 throughout"
exit "$bad"
