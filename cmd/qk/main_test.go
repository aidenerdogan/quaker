package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testApp(t *testing.T) *app {
	t.Helper()
	home := t.TempDir()
	qhome := filepath.Join(home, ".quaker")
	return &app{
		home:      home,
		qhome:     qhome,
		memory:    filepath.Join(qhome, "memory.jsonl"),
		rules:     filepath.Join(qhome, "rules.json"),
		profiles:  filepath.Join(qhome, "profiles.json"),
		hooks:     filepath.Join(qhome, "hooks"),
		schedules: filepath.Join(qhome, "schedules"),
		stdout:    &bytes.Buffer{},
		stderr:    &bytes.Buffer{},
	}
}

func TestDoctorRecordsMemory(t *testing.T) {
	a := testApp(t)
	if err := a.run([]string{"doctor"}); err != nil {
		t.Fatal(err)
	}
	entries := a.readMemory()
	if len(entries) != 1 || entries[0].Command != "doctor" {
		t.Fatalf("doctor memory = %#v", entries)
	}
}

func TestRulesProtectCleanup(t *testing.T) {
	a := testApp(t)
	protected := filepath.Join(a.home, "Library", "Caches", "keep")
	if err := os.MkdirAll(protected, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(protected, "file"), []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := a.run([]string{"rules", "add", "protect", protected}); err != nil {
		t.Fatal(err)
	}
	if err := a.run([]string{"clean", "--apply"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(protected); err != nil {
		t.Fatalf("protected path was removed: %v", err)
	}
}

func TestRulesProtectUninstall(t *testing.T) {
	a := testApp(t)
	appPath := filepath.Join(a.home, "Applications", "Demo.app")
	if err := os.MkdirAll(appPath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(appPath, "Info.plist"), []byte("demo"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := a.run([]string{"rules", "add", "protect", appPath}); err != nil {
		t.Fatal(err)
	}
	if err := a.run([]string{"uninstall", "--apply", "Demo"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(appPath); err != nil {
		t.Fatalf("protected app was removed: %v", err)
	}
}

func TestRulesRemoveByID(t *testing.T) {
	a := testApp(t)
	protected := filepath.Join(a.home, "keep")
	if err := a.run([]string{"rules", "add", "protect", protected}); err != nil {
		t.Fatal(err)
	}
	if err := a.run([]string{"rules", "add", "ignore", "caches"}); err != nil {
		t.Fatal(err)
	}
	if err := a.run([]string{"rules", "remove", "rule-0001"}); err != nil {
		t.Fatal(err)
	}
	if err := a.run([]string{"rules", "remove", "rule-0001"}); err != nil {
		t.Fatal(err)
	}
	rules, err := a.loadRules()
	if err != nil {
		t.Fatal(err)
	}
	if len(rules.Protected) != 0 || len(rules.Ignored) != 0 {
		t.Fatalf("rules were not removed by id: %#v", rules)
	}
}

func TestScheduleRejectsUnsafeProfileName(t *testing.T) {
	a := testApp(t)
	if err := a.run([]string{"schedule", "add", "weekly-scan", "--profile", "../escape"}); err == nil {
		t.Fatal("expected unsafe profile name to fail")
	}
}

func TestLaunchdPlistEscapesXML(t *testing.T) {
	plist := launchdPlist("weekly-scan-safe", `/tmp/qk&tool`, `safe"profile`)
	if strings.Contains(plist, `/tmp/qk&tool`) || strings.Contains(plist, `safe"profile`) {
		t.Fatalf("plist did not escape XML: %s", plist)
	}
	if !strings.Contains(plist, `/tmp/qk&amp;tool`) || !strings.Contains(plist, `safe&quot;profile`) {
		t.Fatalf("plist missing escaped values: %s", plist)
	}
}

func TestHooksRejectUnsupportedEventAndNonExecutable(t *testing.T) {
	a := testApp(t)
	script := filepath.Join(a.home, "hook.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := a.run([]string{"hooks", "install", "../../escape", script}); err == nil {
		t.Fatal("expected unsupported event to fail")
	}
	if err := a.run([]string{"hooks", "install", "after-doctor", script}); err == nil {
		t.Fatal("expected non-executable script to fail")
	}
	if err := os.Chmod(script, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := a.run([]string{"hooks", "install", "after-doctor", script}); err != nil {
		t.Fatal(err)
	}
}

func TestPurgeDryRunFindsProjectArtifacts(t *testing.T) {
	a := testApp(t)
	nodeModules := filepath.Join(a.home, "project", "node_modules")
	if err := os.MkdirAll(nodeModules, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nodeModules, "file"), []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	a.stdout = &out
	if err := a.run([]string{"purge", a.home, "--dry-run"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "node_modules") {
		t.Fatalf("purge output did not include node_modules: %s", out.String())
	}
}
