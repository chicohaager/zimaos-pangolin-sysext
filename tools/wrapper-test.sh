#!/bin/sh
# Tests for files/usr/bin/pangolin: refused commands, everything else reaches the CLI unchanged.
#   sh tools/wrapper-test.sh
set -u
HERE=$(cd "$(dirname "$0")/.." && pwd)
W="$HERE/files/usr/bin/pangolin"
T=$(mktemp -d)
trap 'rm -rf "$T"' EXIT
# The stand-in CLI records each argument followed by "|" (a loop: printf with no argument would still print one "|").
printf '#!/bin/sh\nfor a; do printf "%%s|" "$a"; done > "%s/args"\n' "$T" > "$T/cli"
chmod 755 "$T/cli"
ok=0; bad=0
check() { # name, want-rc, want-args ("-" = the CLI must not run), args...
  name=$1; want=$2; wargs=$3; shift 3
  rm -f "$T/args"
  PANGOLIN_BIN="$T/cli" sh "$W" "$@" >/dev/null 2>&1; rc=$?
  got=$(cat "$T/args" 2>/dev/null || echo -)
  if [ "$rc" -eq "$want" ] && [ "$got" = "$wargs" ]; then ok=$((ok + 1)); else bad=$((bad + 1)); echo "FAIL $name: rc=$rc args=$got (want rc=$want args=$wargs)"; fi
}
check "update refused" 1 - update
check "service install site refused" 1 - service install site
check "service install client refused" 1 - service install client
check "service uninstall refused" 1 - service uninstall site
check "service status passes" 0 "service|status|site|" service status site
check "login passes" 0 "login|" login
check "ssh with spaces passes" 0 "ssh|my host|-p|22|" ssh "my host" -p 22
check "no args passes" 0 ""
echo "wrapper-test: $ok OK, $bad FAIL"
[ "$bad" -eq 0 ]
