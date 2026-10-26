# Pangolin for ZimaOS

**Version 1.0.0** · ZimaOS 1.8.0 (amd64) · sysext module `pangolin`

The [Pangolin CLI](https://github.com/fosrl/cli) as a native ZimaOS module — no container:

- **this host as a Pangolin site**: the site tunnel of the CLI (`pangolin up site`, the Newt code inside the CLI) connects the host to your [Pangolin](https://github.com/fosrl/pangolin) server, so the resources you define there reach the apps on this host — instead of a `fosrl/newt` or `fosrl/pangolin-cli` container;
- **the CLI in the ZimaOS terminal**: `pangolin login`, `pangolin list`, `pangolin ssh`, `pangolin scp`, `pangolin status`;
- the static binary of the upstream release **0.19.0**, checked against pinned sha256;
- `pangolin-zima` — starts the site tunnel now and at every boot;
- the **Pangolin** tile on the dashboard (`pangolin-web`).

## The tile

Enter the **endpoint**, **site ID** and **secret** of a site (Pangolin dashboard: *Sites* → *Add Site* → type *Newt*) and **Save**, then **Turn on**: the tunnel starts, also at every boot. The tile shows the state, the server, the site ID and the last 30 log lines. The secret is written to `/DATA/AppData/pangolin/site.env` (mode 600, root only) and never shown again — the tile only says that one is stored; leave the field empty to keep it. **Forget this site** turns the tunnel off and deletes `site.env` and the copy the tunnel saves of it (`/DATA/AppData/pangolin/.config/pangolin/site.json`; values in `site.env` win over that file).

`site.env` may name the site `NEWT_ID`/`NEWT_SECRET` (what the tile writes) or `SITE_ID`/`SITE_SECRET` (what the Pangolin dashboard offers); newt reads both, the `NEWT_` names win. Other settings of `pangolin up site`, for example `DISABLE_SSH=true` or `DISABLE_CLIENTS=true`, can be added to `site.env` by hand; the tile keeps such lines when it saves.

## Install (on the ZimaOS host)

With the [Module Manager](https://github.com/chicohaager/modmanager) (0.3.0 or later): tab **Catalog**, **Pangolin**, **Install**. By hand:

```sh
cd /tmp
curl -fsSLO https://github.com/chicohaager/zimaos-pangolin-sysext/releases/latest/download/pangolin-amd64.raw
curl -fsSLO https://github.com/chicohaager/zimaos-pangolin-sysext/releases/latest/download/pangolin-amd64.raw.sha256
sha256sum -c pangolin-amd64.raw.sha256
mv pangolin-amd64.raw pangolin.raw      # zpkg takes the module name from the file name
sudo zpkg install /tmp/pangolin.raw
```

## Command line

```sh
sudo pangolin-zima enable      # site tunnel on, also at boot (needs /DATA/AppData/pangolin/site.env)
sudo pangolin-zima disable
pangolin-zima status
pangolin service status site   # upstream's status and log commands work on the module's unit
pangolin login                 # the CLI as a client of your Pangolin server
```

Two upstream commands are refused on ZimaOS, with a hint: `pangolin update` (the CLI lives in the read-only system extension; new versions come with the module) and `pangolin service install|uninstall` (a unit the CLI writes would not start at boot — ZimaOS plans the units before the system extensions are merged; the module's own unit is started by a boot watchdog). The site tunnel's own self-update is switched off for the same reason (`NEWT_SYSTEM_SUBSTRATE=CONTAINER`, the switch newt reads).

## Build and test

```sh
sh build.sh amd64              # downloads the CLI and its source, checks sha256 and the license; needs Go, node, ../lintux-modkit
sudo sh tools/host-test.sh     # on the host after installing; reads only
```

`tools/check-license.sh` stops the build when the CLI or one of its fosrl modules (newt, olm) contains a file under the Fossorial Commercial License: fosrl/cli is mixed-licensed, only files without a license header are AGPL-3.

## License

MIT for this repository. The tile icon is the Pangolin logo (`public/logo/pangolin_orange.svg` of [fosrl/pangolin](https://github.com/fosrl/pangolin), commit 04bf1e1, AGPL-3 by its license notice: no license header) on the tile background. The Pangolin CLI is AGPL-3 (Fossorial, Inc.; `/usr/share/doc/pangolin/LICENSE.pangolin-cli`); its source for the bundled version: <https://github.com/fosrl/cli/tree/0.19.0> (`/usr/share/doc/pangolin/packages.txt`).
