// Command pangolin-web serves the Pangolin tile: the site credentials
// (endpoint, site ID, secret) in /DATA/AppData/pangolin/site.env, the site
// tunnel on and off (on = also at every boot, pangolin-zima) and its log.
// The secret is written, never shown: the tile only learns whether one is set.
package main

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/chicohaager/lintux-modkit/httpx"
	"github.com/chicohaager/lintux-modkit/webmod"
)

// version is set at build time from VERSION (-ldflags -X main.version=...).
var version = "dev"

const unit = "pangolin-site.service"

// Paths and commands; tests point them elsewhere.
var (
	confDir = "/DATA/AppData/pangolin"
	unitDir = "/etc/systemd/system"
	cli     = "/usr/libexec/pangolin/pangolin"
	run     = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return exec.CommandContext(ctx, name, args...).CombinedOutput()
	}
)

var mu sync.Mutex

func envPath() string { return filepath.Join(confDir, "site.env") }

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	restartStale(ctx)
	cancel()
	webmod.Run(webmod.Options{
		Name: "pangolin", Service: "pangolin-web", Binary: "/usr/bin/pangolin-web",
		Route: "/pangolin", Version: version, API: api,
	})
}

func api(verify func(http.Handler) http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/pangolin/status", verify(http.HandlerFunc(statusHandler)))
	mux.Handle("/pangolin/config", verify(http.HandlerFunc(configHandler)))
	mux.Handle("/pangolin/action", verify(http.HandlerFunc(actionHandler)))
	return mux
}

// readEnv returns site.env as ordered lines and a key → value map; a missing
// file is no error.
func readEnv() ([]string, map[string]string, error) {
	b, err := os.ReadFile(envPath())
	if os.IsNotExist(err) {
		return nil, map[string]string{}, nil
	}
	if err != nil {
		return nil, nil, err
	}
	lines := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
	kv := map[string]string{}
	for _, l := range lines {
		if k, v, ok := strings.Cut(l, "="); ok && !strings.HasPrefix(strings.TrimSpace(k), "#") {
			kv[strings.TrimSpace(k)] = v
		}
	}
	return lines, kv, nil
}

// storedSiteID and siteSecret read NEWT_ID/NEWT_SECRET, else the aliases SITE_ID/SITE_SECRET that the
// Pangolin dashboard offers — the order newt itself uses (newtconfig.go, newt 1.19.0).
func storedSiteID(kv map[string]string) string {
	if v := kv["NEWT_ID"]; v != "" {
		return v
	}
	return kv["SITE_ID"]
}

func siteSecret(kv map[string]string) string {
	if v := kv["NEWT_SECRET"]; v != "" {
		return v
	}
	return kv["SITE_SECRET"]
}

// Status is what the tile shows. The secret itself never leaves the host.
type Status struct {
	CLI       string `json:"cli"`
	Endpoint  string `json:"endpoint"`
	SiteID    string `json:"site_id"`
	SecretSet bool   `json:"secret_set"`
	Active    string `json:"active"`
	Boot      bool   `json:"boot"`
	Log       string `json:"log"`
	Error     string `json:"error,omitempty"`
}

func statusHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httpx.MethodNotAllowed(w)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	s := Status{}
	if v, err := run(ctx, cli, "version"); err == nil {
		s.CLI = strings.TrimSpace(string(v))
	}
	_, kv, err := readEnv()
	if err != nil {
		s.Error = err.Error()
	}
	s.Endpoint, s.SiteID, s.SecretSet = kv["PANGOLIN_ENDPOINT"], storedSiteID(kv), siteSecret(kv) != ""
	a, _ := run(ctx, "systemctl", "is-active", unit)
	s.Active = strings.TrimSpace(string(a))
	_, err = os.Stat(filepath.Join(unitDir, "pangolin-site-watchdog.timer"))
	s.Boot = err == nil
	l, _ := run(ctx, "journalctl", "-u", unit, "-n", "30", "--no-pager", "-o", "short-iso")
	s.Log = mask(mask(string(l), kv["NEWT_SECRET"]), kv["SITE_SECRET"])
	httpx.WriteJSON(w, http.StatusOK, s)
}

// mask removes the secret from text that goes to the browser.
func mask(text, secret string) string {
	if secret == "" {
		return text
	}
	return strings.ReplaceAll(text, secret, "•••")
}

var (
	siteIDRe = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)
	secretRe = regexp.MustCompile(`^[A-Za-z0-9._~+/=-]{8,256}$`)
)

// checkEndpoint accepts an http(s) URL with a host and nothing a systemd
// EnvironmentFile line could misread (quotes, spaces, backslashes, $).
func checkEndpoint(s string) error {
	if strings.ContainsAny(s, " \t\r\n\"'\\$`") {
		return httpx.BadRequest("bad_endpoint", "endpoint: no spaces, quotes or backslashes")
	}
	u, err := url.Parse(s)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return httpx.BadRequest("bad_endpoint", "endpoint: an address like https://pangolin.example.com")
	}
	return nil
}

func configHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpx.MethodNotAllowed(w)
		return
	}
	var in struct {
		Endpoint string `json:"endpoint"`
		SiteID   string `json:"site_id"`
		Secret   string `json:"secret"`
	}
	if !httpx.Decode(w, r, &in) {
		return
	}
	in.Endpoint, in.SiteID, in.Secret = strings.TrimSpace(in.Endpoint), strings.TrimSpace(in.SiteID), strings.TrimSpace(in.Secret)
	if err := checkEndpoint(in.Endpoint); err != nil {
		httpx.WriteErr(w, err)
		return
	}
	if !siteIDRe.MatchString(in.SiteID) {
		httpx.WriteError(w, http.StatusBadRequest, "bad_site_id", "site ID: letters, digits, - and _")
		return
	}
	mu.Lock()
	defer mu.Unlock()
	lines, kv, err := readEnv()
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "failed", err.Error())
		return
	}
	// An empty secret keeps the stored one: the tile never has it to send back.
	if in.Secret == "" {
		in.Secret = siteSecret(kv)
		if in.Secret == "" {
			httpx.WriteError(w, http.StatusBadRequest, "bad_secret", "secret: required for a new site")
			return
		}
	} else if !secretRe.MatchString(in.Secret) {
		httpx.WriteError(w, http.StatusBadRequest, "bad_secret", "secret: 8 to 256 characters, letters, digits and ._~+/=-")
		return
	}
	if err := writeEnv(lines, map[string]string{
		"PANGOLIN_ENDPOINT": in.Endpoint, "NEWT_ID": in.SiteID, "NEWT_SECRET": in.Secret,
	}); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "failed", err.Error())
		return
	}
	// A running tunnel takes the new credentials at once.
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	if a, _ := run(ctx, "systemctl", "is-active", unit); strings.TrimSpace(string(a)) == "active" {
		if out, err := run(ctx, "systemctl", "restart", unit); err != nil {
			httpx.WriteError(w, http.StatusBadGateway, "failed", strings.TrimSpace(string(out)))
			return
		}
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// writeEnv sets the given keys in site.env and keeps every other line (for
// example DISABLE_SSH=true added by hand). Written to a 0600 temporary file
// first and renamed, so a half-written secret never sits there.
func writeEnv(lines []string, set map[string]string) error {
	if err := os.MkdirAll(confDir, 0o700); err != nil {
		return err
	}
	done := map[string]bool{}
	var out []string
	for _, l := range lines {
		k, _, ok := strings.Cut(l, "=")
		k = strings.TrimSpace(k)
		if v, mine := set[k]; ok && mine {
			if !done[k] {
				out = append(out, k+"="+v)
				done[k] = true
			}
			continue
		}
		out = append(out, l)
	}
	for _, k := range []string{"PANGOLIN_ENDPOINT", "NEWT_ID", "NEWT_SECRET"} {
		if !done[k] {
			out = append(out, k+"="+set[k])
		}
	}
	tmp, err := os.CreateTemp(confDir, ".site.env-")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.WriteString(strings.Join(out, "\n") + "\n"); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), envPath())
}

func actionHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpx.MethodNotAllowed(w)
		return
	}
	var in struct {
		Action string `json:"action"`
	}
	if !httpx.Decode(w, r, &in) {
		return
	}
	var cmd []string
	switch in.Action {
	case "enable", "disable":
		cmd = []string{"pangolin-zima", in.Action}
	case "restart":
		cmd = []string{"systemctl", "restart", unit}
	case "forget":
		cmd = []string{"pangolin-zima", "disable"}
	default:
		httpx.WriteError(w, http.StatusBadRequest, "bad_action", "action: enable, disable, restart or forget")
		return
	}
	mu.Lock()
	defer mu.Unlock()
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	if out, err := run(ctx, cmd[0], cmd[1:]...); err != nil {
		httpx.WriteError(w, http.StatusBadGateway, "failed", strings.TrimSpace(string(out)))
		return
	}
	if in.Action == "forget" {
		// site.env and the copy 'pangolin up site' saves under HOME=confDir
		// ("Saving config to: …/.config/pangolin/site.json", measured 1.8.0).
		for _, p := range []string{envPath(), filepath.Join(confDir, ".config", "pangolin", "site.json")} {
			if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
				httpx.WriteError(w, http.StatusInternalServerError, "failed", fmt.Sprintf("tunnel off, but %s stays: %v", p, err))
				return
			}
		}
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Paths of the running tunnel; tests replace them.
var (
	mainPID = func(ctx context.Context) string {
		out, _ := run(ctx, "systemctl", "show", "-p", "MainPID", "--value", unit)
		return strings.TrimSpace(string(out))
	}
	readExe = func(pid string) (string, error) { return os.Readlink("/proc/" + pid + "/exe") }
)

// restartStale restarts a running tunnel whose binary is not the current
// /usr/libexec/pangolin/pangolin. zpkg restarts only the services in the module
// JSON (pangolin-web) when the module is replaced; the tunnel kept running from
// the old image, its /proc/<pid>/exe read /libexec/pangolin/pangolin (measured
// 1.8.0, 2026-10-26). The inode is no sign: it was 15 in both images.
func restartStale(ctx context.Context) {
	if a, _ := run(ctx, "systemctl", "is-active", unit); strings.TrimSpace(string(a)) != "active" {
		return
	}
	pid := mainPID(ctx)
	if pid == "" || pid == "0" {
		return
	}
	exe, err := readExe(pid)
	if err != nil || exe == cli {
		return
	}
	fmt.Fprintf(os.Stderr, "pangolin-web: %s runs %s, not %s: restarting it\n", unit, exe, cli)
	if out, err := run(ctx, "systemctl", "restart", unit); err != nil {
		fmt.Fprintf(os.Stderr, "pangolin-web: restart %s: %v %s\n", unit, err, out)
	}
}
