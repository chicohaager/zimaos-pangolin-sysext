#!/bin/sh
# Run on a ZimaOS host as root after `zpkg install pangolin.raw`. Reads only, changes nothing:
# the site tunnel needs credentials from a Pangolin server (the tile or site.env).
#   sudo sh host-test.sh           check CLI, wrapper, units, tile; with a running site tunnel also its log
set -u
ok=0; bad=0
pass() { ok=$((ok + 1)); echo "OK   $1"; }
fail() { bad=$((bad + 1)); echo "FAIL $1"; }

v=$(pangolin version 2>&1 | head -n 1); [ "$v" = 0.19.0 ] && pass "pangolin version $v" || fail "pangolin version: $v"
pangolin update >/dev/null 2>&1 && fail "pangolin update ran" || pass "pangolin update refused"
pangolin service install site >/dev/null 2>&1 && fail "pangolin service install ran" || pass "pangolin service install refused"
[ ! -e /etc/systemd/system/pangolin-site.service ] && pass "no upstream unit in /etc" || fail "/etc/systemd/system/pangolin-site.service exists (it would replace the module's unit)"
h=$(pangolin --help 2>&1); case "$h" in *"Login to Pangolin"*) pass "pangolin --help" ;; *) fail "pangolin --help: $(echo "$h" | head -n 1)" ;; esac
l=$(systemctl show -p LoadState --value pangolin-site.service); [ "$l" = loaded ] && pass "pangolin-site.service loaded" || fail "pangolin-site.service LoadState=$l"
e=$(systemctl show -p Environment --value pangolin-site.service)
case "$e" in *NEWT_SYSTEM_SUBSTRATE=CONTAINER*) pass "self-update off (NEWT_SYSTEM_SUBSTRATE)" ;; *) fail "unit environment: $e" ;; esac
a=$(systemctl is-active pangolin-web.service); [ "$a" = active ] && pass "pangolin-web active" || fail "pangolin-web: $a"
h=$(curl -fs -m 5 http://127.0.0.1/pangolin/health); case "$h" in *'"version":"1.0.0"'*) pass "tile health $h" ;; *) fail "tile health: $h" ;; esac
s=$(curl -s -o /dev/null -w '%{http_code}' -m 5 http://127.0.0.1/pangolin/status); [ "$s" = 401 ] && pass "status without login: 401" || fail "status without login: $s"
if [ -f /DATA/AppData/pangolin/site.env ]; then
  m=$(stat -c %a /DATA/AppData/pangolin/site.env); [ "$m" = 600 ] && pass "site.env mode 600" || fail "site.env mode $m"
fi
if [ "$(systemctl is-active pangolin-site.service)" = active ]; then
  pass "site tunnel active since $(systemctl show -p ActiveEnterTimestamp --value pangolin-site.service)"
  [ -f /etc/systemd/system/pangolin-site-watchdog.timer ] && pass "boot watchdog" || fail "site tunnel without boot watchdog"
fi

echo "pangolin host-test: $ok OK, $bad FAIL"
[ "$bad" -eq 0 ]
