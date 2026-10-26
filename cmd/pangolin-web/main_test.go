package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Test values only (RFC 2606 names, made-up IDs and secrets).
const (
	endpoint = "https://pangolin.example.com"
	siteID   = "k3x9q2w7m1"
	secret   = "s3cr3t-Value_for.tests"
)

type fake struct {
	calls  [][]string
	active string
	log    string
}

func setup(t *testing.T) *fake {
	t.Helper()
	f := &fake{active: "inactive"}
	confDir = filepath.Join(t.TempDir(), "pangolin")
	unitDir = t.TempDir()
	run = func(_ context.Context, name string, args ...string) ([]byte, error) {
		f.calls = append(f.calls, append([]string{name}, args...))
		switch {
		case name == "systemctl" && args[0] == "is-active":
			return []byte(f.active + "\n"), nil
		case name == "journalctl":
			return []byte(f.log), nil
		case name == cli && args[0] == "version":
			return []byte("0.19.0\n"), nil
		}
		return nil, nil
	}
	return f
}

func call(t *testing.T, method, path string, body any) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	w := httptest.NewRecorder()
	api(func(h http.Handler) http.Handler { return h }).ServeHTTP(w, httptest.NewRequest(method, path, &buf))
	var out map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w, out
}

func envFile(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(envPath())
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestNewSiteIsWrittenPrivateAndTheSecretNeverComesBack(t *testing.T) {
	setup(t)
	w, _ := call(t, "POST", "/pangolin/config", map[string]string{"endpoint": endpoint, "site_id": siteID, "secret": secret})
	if w.Code != 200 {
		t.Fatalf("config: %d %s", w.Code, w.Body)
	}
	want := "PANGOLIN_ENDPOINT=" + endpoint + "\nNEWT_ID=" + siteID + "\nNEWT_SECRET=" + secret + "\n"
	if got := envFile(t); got != want {
		t.Fatalf("site.env:\n%s\nwant:\n%s", got, want)
	}
	st, _ := os.Stat(envPath())
	if st.Mode().Perm() != 0o600 {
		t.Fatalf("site.env mode %v, want 0600", st.Mode().Perm())
	}
	w, out := call(t, "GET", "/pangolin/status", nil)
	if strings.Contains(w.Body.String(), secret) {
		t.Fatalf("status shows the secret: %s", w.Body)
	}
	if out["secret_set"] != true || out["site_id"] != siteID || out["endpoint"] != endpoint || out["cli"] != "0.19.0" {
		t.Fatalf("status: %s", w.Body)
	}
}

func TestEmptySecretKeepsTheStoredOneAndOtherLinesStay(t *testing.T) {
	f := setup(t)
	_ = os.MkdirAll(confDir, 0o700)
	_ = os.WriteFile(envPath(), []byte("NEWT_SECRET="+secret+"\nDISABLE_SSH=true\nNEWT_ID=old\n"), 0o600)
	f.active = "active"
	w, _ := call(t, "POST", "/pangolin/config", map[string]string{"endpoint": endpoint, "site_id": siteID, "secret": ""})
	if w.Code != 200 {
		t.Fatalf("config: %d %s", w.Code, w.Body)
	}
	want := "NEWT_SECRET=" + secret + "\nDISABLE_SSH=true\nNEWT_ID=" + siteID + "\nPANGOLIN_ENDPOINT=" + endpoint + "\n"
	if got := envFile(t); got != want {
		t.Fatalf("site.env:\n%s\nwant:\n%s", got, want)
	}
	last := f.calls[len(f.calls)-1]
	if strings.Join(last, " ") != "systemctl restart "+unit {
		t.Fatalf("a running tunnel must restart with the new values, last call %v", last)
	}
}

func TestBadInputIsRefusedAndNothingIsWritten(t *testing.T) {
	cases := []map[string]string{
		{"endpoint": "https://x.example.com\nNEWT_SECRET=evil", "site_id": siteID, "secret": secret},
		{"endpoint": "pangolin.example.com", "site_id": siteID, "secret": secret},
		{"endpoint": "https://pangolin.example.com/$HOME", "site_id": siteID, "secret": secret},
		{"endpoint": "ftp://pangolin.example.com", "site_id": siteID, "secret": secret},
		{"endpoint": endpoint, "site_id": "a b", "secret": secret},
		{"endpoint": endpoint, "site_id": siteID, "secret": "short"},
		{"endpoint": endpoint, "site_id": siteID, "secret": "with space inside"},
		{"endpoint": endpoint, "site_id": siteID, "secret": ""}, // new site without a stored secret
	}
	for _, c := range cases {
		setup(t)
		w, _ := call(t, "POST", "/pangolin/config", c)
		if w.Code != 400 {
			t.Errorf("%v: %d, want 400", c, w.Code)
		}
		if _, err := os.Stat(envPath()); !os.IsNotExist(err) {
			t.Errorf("%v: site.env written", c)
		}
	}
}

func TestLogMasksTheSecret(t *testing.T) {
	f := setup(t)
	_ = os.MkdirAll(confDir, 0o700)
	_ = os.WriteFile(envPath(), []byte("NEWT_SECRET="+secret+"\n"), 0o600)
	f.log = "2026-10-26T08:00:00+0100 host pangolin[1]: connecting with secret " + secret + "\n"
	w, out := call(t, "GET", "/pangolin/status", nil)
	if strings.Contains(w.Body.String(), secret) || !strings.Contains(out["log"].(string), "connecting with secret •••") {
		t.Fatalf("log: %s", w.Body)
	}
}

func TestBootWatchdogIsReported(t *testing.T) {
	setup(t)
	_ = os.WriteFile(filepath.Join(unitDir, "pangolin-site-watchdog.timer"), nil, 0o644)
	_, out := call(t, "GET", "/pangolin/status", nil)
	if out["boot"] != true {
		t.Fatalf("boot: %v", out["boot"])
	}
}

func TestActions(t *testing.T) {
	f := setup(t)
	for _, c := range []struct{ action, cmd string }{
		{"enable", "pangolin-zima enable"},
		{"disable", "pangolin-zima disable"},
		{"restart", "systemctl restart " + unit},
	} {
		w, _ := call(t, "POST", "/pangolin/action", map[string]string{"action": c.action})
		last := f.calls[len(f.calls)-1]
		if w.Code != 200 || strings.Join(last, " ") != c.cmd {
			t.Errorf("%s: %d, ran %v, want %s", c.action, w.Code, last, c.cmd)
		}
	}
	if w, _ := call(t, "POST", "/pangolin/action", map[string]string{"action": "rm"}); w.Code != 400 {
		t.Errorf("unknown action: %d, want 400", w.Code)
	}
}

func TestForgetTurnsOffFirstThenRemovesTheCredentials(t *testing.T) {
	f := setup(t)
	_ = os.MkdirAll(confDir, 0o700)
	_ = os.WriteFile(envPath(), []byte("NEWT_SECRET="+secret+"\n"), 0o600)
	w, _ := call(t, "POST", "/pangolin/action", map[string]string{"action": "forget"})
	if w.Code != 200 {
		t.Fatalf("forget: %d %s", w.Code, w.Body)
	}
	if strings.Join(f.calls[len(f.calls)-1], " ") != "pangolin-zima disable" {
		t.Fatalf("forget ran %v", f.calls)
	}
	if _, err := os.Stat(envPath()); !os.IsNotExist(err) {
		t.Fatal("site.env still there")
	}
}

// 'pangolin up site' saves the credentials it ran with to $HOME/.config/pangolin/site.json
// (measured on ZimaOS 1.8.0, 2026-10-26: "Saving config to: …/site.json"): forget removes that copy too.
func TestForgetRemovesTheCopyTheTunnelSaved(t *testing.T) {
	setup(t)
	saved := filepath.Join(confDir, ".config", "pangolin", "site.json")
	_ = os.MkdirAll(filepath.Dir(saved), 0o700)
	_ = os.WriteFile(envPath(), []byte("NEWT_SECRET="+secret+"\n"), 0o600)
	_ = os.WriteFile(saved, []byte(`{"secret":"`+secret+`"}`), 0o600)
	if w, _ := call(t, "POST", "/pangolin/action", map[string]string{"action": "forget"}); w.Code != 200 {
		t.Fatalf("forget: %d %s", w.Code, w.Body)
	}
	if _, err := os.Stat(saved); !os.IsNotExist(err) {
		t.Fatal("site.json with the credentials still there")
	}
}

// The Pangolin dashboard offers SITE_ID/SITE_SECRET (pangolin-cli), newt reads them as aliases
// of NEWT_ID/NEWT_SECRET (newtconfig.go 1.19.0): a site.env written that way counts too.
func TestSiteAliasesCount(t *testing.T) {
	f := setup(t)
	_ = os.MkdirAll(confDir, 0o700)
	_ = os.WriteFile(envPath(), []byte("PANGOLIN_ENDPOINT="+endpoint+"\nSITE_ID="+siteID+"\nSITE_SECRET="+secret+"\n"), 0o600)
	f.log = "using " + secret + "\n"
	w, out := call(t, "GET", "/pangolin/status", nil)
	if out["site_id"] != siteID || out["secret_set"] != true || strings.Contains(w.Body.String(), secret) {
		t.Fatalf("status: %s", w.Body)
	}
	w, _ = call(t, "POST", "/pangolin/config", map[string]string{"endpoint": endpoint, "site_id": siteID, "secret": ""})
	if w.Code != 200 {
		t.Fatalf("config with the stored SITE_SECRET: %d %s", w.Code, w.Body)
	}
	if got := envFile(t); !strings.Contains(got, "NEWT_SECRET="+secret+"\n") {
		t.Fatalf("site.env:\n%s", got)
	}
}
