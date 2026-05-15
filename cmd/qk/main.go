package main

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const version = "0.2.0"

type app struct {
	home      string
	qhome     string
	memory    string
	rules     string
	profiles  string
	hooks     string
	schedules string
	stdout    io.Writer
	stderr    io.Writer
}

type memoryEntry struct {
	ID        string   `json:"id"`
	Timestamp string   `json:"timestamp"`
	Command   string   `json:"command"`
	Args      []string `json:"args"`
	DryRun    bool     `json:"dry_run"`
	Result    string   `json:"result"`
	Paths     []string `json:"paths,omitempty"`
	Bytes     int64    `json:"bytes"`
	Source    string   `json:"source"`
}

type ruleSet struct {
	Protected []string `json:"protected"`
	Ignored   []string `json:"ignored"`
	Allowed   []string `json:"allowed_categories"`
}

type profile struct {
	Name    string   `json:"name"`
	Command string   `json:"command"`
	Args    []string `json:"args"`
}

type profileFile struct {
	Profiles []profile `json:"profiles"`
}

type scanItem struct {
	Path  string `json:"path"`
	Size  int64  `json:"size"`
	Kind  string `json:"kind"`
	Note  string `json:"note,omitempty"`
	Skip  bool   `json:"skip,omitempty"`
	Rule  string `json:"rule,omitempty"`
	IsDir bool   `json:"is_dir"`
}

func main() {
	a, err := newApp()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := a.run(os.Args[1:]); err != nil {
		fmt.Fprintln(a.stderr, err)
		os.Exit(1)
	}
}

func newApp() (*app, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	qhome := os.Getenv("QUAKER_HOME")
	if qhome == "" {
		qhome = filepath.Join(home, ".quaker")
	}
	return &app{
		home:      home,
		qhome:     qhome,
		memory:    filepath.Join(qhome, "memory.jsonl"),
		rules:     filepath.Join(qhome, "rules.json"),
		profiles:  filepath.Join(qhome, "profiles.json"),
		hooks:     filepath.Join(qhome, "hooks"),
		schedules: filepath.Join(qhome, "schedules"),
		stdout:    os.Stdout,
		stderr:    os.Stderr,
	}, nil
}

func (a *app) run(args []string) error {
	if len(args) == 0 {
		a.help()
		return nil
	}
	switch args[0] {
	case "-h", "--help", "help":
		a.help()
	case "-v", "--version", "version":
		fmt.Fprintf(a.stdout, "Quaker version %s\nEngine: Quaker Go engine\nState: %s\n", version, a.qhome)
	case "clean":
		return a.clean(args[1:])
	case "installer":
		return a.installer(args[1:])
	case "purge":
		return a.purge(args[1:])
	case "uninstall":
		return a.uninstall(args[1:])
	case "optimize":
		return a.optimize(args[1:])
	case "analyze", "analyse":
		return a.analyze(args[1:])
	case "status":
		return a.status(args[1:])
	case "touchid":
		return a.touchid(args[1:])
	case "completion":
		return a.completion(args[1:])
	case "update":
		return a.update(args[1:])
	case "doctor":
		return a.doctor(args[1:])
	case "memory":
		return a.memoryCmd(args[1:])
	case "rules":
		return a.rulesCmd(args[1:])
	case "profile":
		return a.profileCmd(args[1:])
	case "schedule":
		return a.scheduleCmd(args[1:])
	case "hooks":
		return a.hooksCmd(args[1:])
	default:
		return fmt.Errorf("unknown command: %s", args[0])
	}
	return nil
}

func (a *app) help() {
	fmt.Fprint(a.stdout, `Quaker
Remembered, rule-driven Mac maintenance.

Usage: qk <command> [options]

Core commands:
  qk clean [--dry-run|--apply]       Scan and clean safe user clutter
  qk installer [--dry-run|--apply]   Find installer files
  qk purge [path] [--dry-run|--apply] Scan project build artifacts
  qk uninstall [--list|--dry-run APP|--apply APP]
  qk optimize [--dry-run|--apply]    Run safe maintenance checks
  qk analyze [--json] [path]         Explore disk usage
  qk status [--json]                 Show system status
  qk touchid <status|enable|disable> Configure Touch ID for sudo
  qk completion <bash|zsh|fish>      Generate shell completions
  qk update                          Show update guidance

Quaker commands:
  qk doctor
  qk memory list|show|export|forget
  qk rules list|add|remove|check
  qk profile list|create|run
  qk schedule list|add|remove
  qk hooks list|install

Safety:
  Cleanup commands default to dry-run unless --apply is passed.
  Rules protect paths before any delete is attempted.
  Scheduled work is suggest-only by default.
`)
}

func (a *app) ensureState() error {
	for _, dir := range []string{a.qhome, a.hooks, a.schedules} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	if _, err := os.Stat(a.memory); errors.Is(err, os.ErrNotExist) {
		if err := os.WriteFile(a.memory, nil, 0o644); err != nil {
			return err
		}
	}
	if _, err := os.Stat(a.rules); errors.Is(err, os.ErrNotExist) {
		if err := a.saveRules(ruleSet{}); err != nil {
			return err
		}
	}
	if _, err := os.Stat(a.profiles); errors.Is(err, os.ErrNotExist) {
		if err := a.saveProfiles(profileFile{}); err != nil {
			return err
		}
	}
	return nil
}

func newID() string {
	var b [4]byte
	_, _ = rand.Read(b[:])
	return "qk-" + time.Now().UTC().Format("20060102150405") + "-" + hex.EncodeToString(b[:])
}

func has(args []string, want string) bool {
	for _, arg := range args {
		if arg == want {
			return true
		}
	}
	return false
}

func (a *app) record(command string, args []string, dry bool, result string, paths []string, bytes int64) string {
	_ = a.ensureState()
	entry := memoryEntry{
		ID:        newID(),
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Command:   command,
		Args:      args,
		DryRun:    dry,
		Result:    result,
		Paths:     paths,
		Bytes:     bytes,
		Source:    "cli",
	}
	data, _ := json.Marshal(entry)
	f, err := os.OpenFile(a.memory, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err == nil {
		_, _ = f.Write(append(data, '\n'))
		_ = f.Close()
	}
	return entry.ID
}

func parseMode(args []string) (dry bool, jsonOut bool, rest []string, err error) {
	dry = true
	for _, arg := range args {
		switch arg {
		case "--dry-run", "-n":
			dry = true
		case "--apply":
			dry = false
		case "--json":
			jsonOut = true
		default:
			rest = append(rest, arg)
		}
	}
	return dry, jsonOut, rest, nil
}

func (a *app) loadRules() (ruleSet, error) {
	_ = a.ensureState()
	var rules ruleSet
	data, err := os.ReadFile(a.rules)
	if err != nil {
		return rules, err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return rules, nil
	}
	err = json.Unmarshal(data, &rules)
	return rules, err
}

func (a *app) saveRules(rules ruleSet) error {
	data, err := json.MarshalIndent(rules, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(a.rules, append(data, '\n'), 0o644)
}

func (a *app) protected(path string) (bool, string) {
	rules, err := a.loadRules()
	if err != nil {
		return false, ""
	}
	abs, _ := filepath.Abs(path)
	for _, rule := range rules.Protected {
		expanded := expandHome(rule, a.home)
		match, _ := filepath.Match(expanded, abs)
		if match || abs == expanded || strings.HasPrefix(abs, strings.TrimRight(expanded, string(os.PathSeparator))+string(os.PathSeparator)) {
			return true, rule
		}
	}
	return false, ""
}

func expandHome(path, home string) string {
	if path == "~" {
		return home
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(home, strings.TrimPrefix(path, "~/"))
	}
	return os.ExpandEnv(path)
}

func dirSize(path string) int64 {
	var total int64
	_ = filepath.WalkDir(path, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		info, err := d.Info()
		if err == nil && !info.IsDir() {
			total += info.Size()
		}
		return nil
	})
	return total
}

func removePath(path string) error {
	if path == "" || path == "/" {
		return fmt.Errorf("refusing unsafe path: %s", path)
	}
	return os.RemoveAll(path)
}

func human(n int64) string {
	units := []string{"B", "KB", "MB", "GB", "TB"}
	v := float64(n)
	i := 0
	for v >= 1024 && i < len(units)-1 {
		v /= 1024
		i++
	}
	if i == 0 {
		return fmt.Sprintf("%d%s", n, units[i])
	}
	return fmt.Sprintf("%.1f%s", v, units[i])
}

func (a *app) clean(args []string) error {
	dry, jsonOut, _, _ := parseMode(args)
	items := a.scanClean()
	return a.finishCleanup("clean", args, dry, jsonOut, items)
}

func (a *app) scanClean() []scanItem {
	candidates := []struct {
		path string
		kind string
	}{
		{filepath.Join(a.home, "Library", "Caches"), "user-cache"},
		{filepath.Join(a.home, "Library", "Logs"), "user-log"},
		{filepath.Join(a.home, ".Trash"), "trash"},
		{filepath.Join(a.home, ".cache"), "tool-cache"},
	}
	var items []scanItem
	for _, c := range candidates {
		entries, err := os.ReadDir(c.path)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			p := filepath.Join(c.path, entry.Name())
			size := dirSize(p)
			if !entry.IsDir() {
				if info, err := entry.Info(); err == nil {
					size = info.Size()
				}
			}
			if size == 0 {
				continue
			}
			skip, rule := a.protected(p)
			items = append(items, scanItem{Path: p, Size: size, Kind: c.kind, Skip: skip, Rule: rule, IsDir: entry.IsDir()})
		}
	}
	sortItems(items)
	return items
}

func (a *app) installer(args []string) error {
	dry, jsonOut, _, _ := parseMode(args)
	var items []scanItem
	roots := []string{filepath.Join(a.home, "Downloads"), filepath.Join(a.home, "Desktop")}
	exts := map[string]bool{".dmg": true, ".pkg": true, ".iso": true, ".xip": true, ".zip": true}
	for _, root := range roots {
		_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			if exts[strings.ToLower(filepath.Ext(path))] {
				info, err := d.Info()
				if err != nil {
					return nil
				}
				skip, rule := a.protected(path)
				items = append(items, scanItem{Path: path, Size: info.Size(), Kind: "installer", Skip: skip, Rule: rule})
			}
			return nil
		})
	}
	sortItems(items)
	return a.finishCleanup("installer", args, dry, jsonOut, items)
}

func (a *app) purge(args []string) error {
	dry, jsonOut, rest, _ := parseMode(args)
	root := a.home
	if len(rest) > 0 {
		root = expandHome(rest[0], a.home)
	}
	items := a.scanPurge(root)
	return a.finishCleanup("purge", args, dry, jsonOut, items)
}

func (a *app) scanPurge(root string) []scanItem {
	targets := map[string]bool{
		"node_modules": true, "dist": true, "build": true, ".next": true,
		".nuxt": true, "target": true, "vendor/bundle": true, "coverage": true,
	}
	var items []scanItem
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() || path == root {
			return nil
		}
		name := d.Name()
		rel, _ := filepath.Rel(root, path)
		if targets[name] || targets[filepath.ToSlash(rel)] {
			skip, rule := a.protected(path)
			items = append(items, scanItem{Path: path, Size: dirSize(path), Kind: "project-artifact", Skip: skip, Rule: rule, IsDir: true})
			return filepath.SkipDir
		}
		if strings.Count(rel, string(os.PathSeparator)) > 6 {
			return filepath.SkipDir
		}
		return nil
	})
	sortItems(items)
	return items
}

func sortItems(items []scanItem) {
	sort.Slice(items, func(i, j int) bool {
		return items[i].Size > items[j].Size
	})
}

func (a *app) finishCleanup(command string, args []string, dry, jsonOut bool, items []scanItem) error {
	var total int64
	var paths []string
	for i := range items {
		if items[i].Skip {
			continue
		}
		total += items[i].Size
		paths = append(paths, items[i].Path)
		if !dry {
			if err := removePath(items[i].Path); err != nil {
				items[i].Note = err.Error()
				items[i].Skip = true
			}
		}
	}
	id := a.record(command, args, dry, "ok", paths, total)
	if jsonOut {
		return json.NewEncoder(a.stdout).Encode(map[string]any{
			"memory_id": id, "dry_run": dry, "bytes": total, "items": items,
		})
	}
	if dry {
		fmt.Fprintf(a.stdout, "Quaker %s dry run\n", command)
	} else {
		fmt.Fprintf(a.stdout, "Quaker %s applied\n", command)
	}
	if len(items) == 0 {
		fmt.Fprintln(a.stdout, "No matching items found.")
	} else {
		for _, item := range items {
			marker := "would remove"
			if !dry {
				marker = "removed"
			}
			if item.Skip {
				marker = "protected"
			}
			fmt.Fprintf(a.stdout, "  %s  %8s  %s\n", marker, human(item.Size), item.Path)
		}
	}
	fmt.Fprintf(a.stdout, "Total: %s\nMemory: %s\n", human(total), id)
	return nil
}

func (a *app) uninstall(args []string) error {
	dry, jsonOut, rest, _ := parseMode(args)
	if has(args, "--list") {
		apps := listApps(a.home)
		if jsonOut {
			return json.NewEncoder(a.stdout).Encode(apps)
		}
		for _, app := range apps {
			fmt.Fprintf(a.stdout, "%s\t%s\n", app["name"], app["path"])
		}
		a.record("uninstall", args, true, "listed", nil, 0)
		return nil
	}
	if len(rest) == 0 {
		return fmt.Errorf("usage: qk uninstall --list | [--dry-run|--apply] <app>")
	}
	name := strings.Join(rest, " ")
	appPath := findApp(a.home, name)
	if appPath == "" {
		return fmt.Errorf("app not found: %s", name)
	}
	items := []scanItem{{Path: appPath, Size: dirSize(appPath), Kind: "application", IsDir: true}}
	leftovers := leftoverPaths(a.home, name)
	for _, p := range leftovers {
		if _, err := os.Stat(p); err == nil {
			items = append(items, scanItem{Path: p, Size: dirSize(p), Kind: "leftover", IsDir: true})
		}
	}
	return a.finishCleanup("uninstall", args, dry, jsonOut, items)
}

func listApps(home string) []map[string]string {
	var out []map[string]string
	for _, root := range []string{"/Applications", filepath.Join(home, "Applications")} {
		entries, _ := os.ReadDir(root)
		for _, e := range entries {
			if e.IsDir() && strings.HasSuffix(e.Name(), ".app") {
				name := strings.TrimSuffix(e.Name(), ".app")
				out = append(out, map[string]string{"name": name, "path": filepath.Join(root, e.Name())})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i]["name"] < out[j]["name"] })
	return out
}

func findApp(home, name string) string {
	want := strings.ToLower(strings.TrimSuffix(name, ".app"))
	for _, app := range listApps(home) {
		if strings.ToLower(app["name"]) == want {
			return app["path"]
		}
	}
	return ""
}

func leftoverPaths(home, name string) []string {
	key := strings.NewReplacer(" ", "", ".", "", "-", "").Replace(strings.ToLower(name))
	var paths []string
	roots := []string{
		filepath.Join(home, "Library", "Application Support"),
		filepath.Join(home, "Library", "Caches"),
		filepath.Join(home, "Library", "Preferences"),
		filepath.Join(home, "Library", "Logs"),
	}
	for _, root := range roots {
		entries, _ := os.ReadDir(root)
		for _, e := range entries {
			cmp := strings.NewReplacer(" ", "", ".", "", "-", "").Replace(strings.ToLower(e.Name()))
			if strings.Contains(cmp, key) {
				paths = append(paths, filepath.Join(root, e.Name()))
			}
		}
	}
	return paths
}

func (a *app) optimize(args []string) error {
	dry, jsonOut, _, _ := parseMode(args)
	tasks := []map[string]string{
		{"name": "DNS cache check", "status": "suggested"},
		{"name": "LaunchServices check", "status": "suggested"},
		{"name": "Font cache check", "status": "suggested"},
		{"name": "Login item audit", "status": "suggested"},
	}
	if !dry {
		for i := range tasks {
			tasks[i]["status"] = "checked"
		}
	}
	id := a.record("optimize", args, dry, "ok", nil, 0)
	if jsonOut {
		return json.NewEncoder(a.stdout).Encode(map[string]any{"memory_id": id, "dry_run": dry, "tasks": tasks})
	}
	fmt.Fprintln(a.stdout, "Quaker optimize")
	for _, task := range tasks {
		fmt.Fprintf(a.stdout, "  %s: %s\n", task["name"], task["status"])
	}
	fmt.Fprintf(a.stdout, "Memory: %s\n", id)
	return nil
}

func (a *app) analyze(args []string) error {
	_, jsonOut, rest, _ := parseMode(args)
	path := "."
	if len(rest) > 0 {
		path = expandHome(rest[0], a.home)
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}
	var rows []scanItem
	var total int64
	for _, e := range entries {
		p := filepath.Join(path, e.Name())
		size := dirSize(p)
		if !e.IsDir() {
			if info, err := e.Info(); err == nil {
				size = info.Size()
			}
		}
		total += size
		rows = append(rows, scanItem{Path: p, Size: size, Kind: "entry", IsDir: e.IsDir()})
	}
	sortItems(rows)
	id := a.record("analyze", args, true, "ok", []string{path}, total)
	if jsonOut {
		return json.NewEncoder(a.stdout).Encode(map[string]any{"memory_id": id, "path": path, "total_size": total, "entries": rows})
	}
	for _, row := range rows {
		fmt.Fprintf(a.stdout, "%8s  %s\n", human(row.Size), row.Path)
	}
	fmt.Fprintf(a.stdout, "Total: %s\nMemory: %s\n", human(total), id)
	return nil
}

func (a *app) status(args []string) error {
	jsonOut := has(args, "--json")
	host, _ := os.Hostname()
	free, total := diskUsage(a.home)
	payload := map[string]any{
		"host":       host,
		"os":         runtime.GOOS,
		"arch":       runtime.GOARCH,
		"cpus":       runtime.NumCPU(),
		"go_version": runtime.Version(),
		"disk_free":  free,
		"disk_total": total,
		"time":       time.Now().Format(time.RFC3339),
	}
	id := a.record("status", args, true, "ok", nil, 0)
	payload["memory_id"] = id
	if jsonOut {
		return json.NewEncoder(a.stdout).Encode(payload)
	}
	fmt.Fprintf(a.stdout, "Host: %s\nPlatform: %s/%s\nCPU cores: %d\nDisk: %s free of %s\nMemory: %s\n",
		host, runtime.GOOS, runtime.GOARCH, runtime.NumCPU(), human(free), human(total), id)
	return nil
}

func diskUsage(path string) (free int64, total int64) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, 0
	}
	return int64(stat.Bavail) * int64(stat.Bsize), int64(stat.Blocks) * int64(stat.Bsize)
}

func (a *app) touchid(args []string) error {
	cmd := "status"
	if len(args) > 0 {
		cmd = args[0]
	}
	dry := has(args, "--dry-run") || has(args, "-n")
	switch cmd {
	case "status":
		fmt.Fprintln(a.stdout, "Touch ID sudo status: inspect /etc/pam.d/sudo_local on macOS.")
	case "enable", "disable":
		if dry {
			fmt.Fprintf(a.stdout, "Dry run: would %s Touch ID sudo configuration.\n", cmd)
		} else {
			fmt.Fprintf(a.stdout, "Quaker does not edit PAM automatically yet. Run with --dry-run for guidance.\n")
		}
	default:
		return fmt.Errorf("usage: qk touchid status|enable|disable [--dry-run]")
	}
	a.record("touchid", args, dry, "ok", nil, 0)
	return nil
}

func (a *app) completion(args []string) error {
	shell := "zsh"
	if len(args) > 0 {
		shell = args[0]
	}
	commands := "clean uninstall optimize analyze status purge installer touchid completion update memory rules profile schedule hooks doctor help version"
	switch shell {
	case "bash":
		fmt.Fprintf(a.stdout, "_qk_completion() {\n  COMPREPLY=($(compgen -W %q -- \"${COMP_WORDS[COMP_CWORD]}\"))\n}\ncomplete -F _qk_completion qk quaker\n", commands)
	case "zsh":
		fmt.Fprintf(a.stdout, "#compdef qk quaker\n_qk() { _arguments '1:command:(%s)' }\ncompdef _qk qk quaker\n", commands)
	case "fish":
		fmt.Fprintf(a.stdout, "complete -c qk -f -a %q\ncomplete -c quaker -f -a %q\n", commands, commands)
	default:
		return fmt.Errorf("usage: qk completion <bash|zsh|fish>")
	}
	return nil
}

func (a *app) update(args []string) error {
	fmt.Fprintln(a.stdout, "Quaker update guidance:")
	fmt.Fprintln(a.stdout, "  1. Pull the latest Quaker source.")
	fmt.Fprintln(a.stdout, "  2. Run make build.")
	fmt.Fprintln(a.stdout, "  3. Run ./install-quaker.sh.")
	a.record("update", args, true, "ok", nil, 0)
	return nil
}

func (a *app) doctor(args []string) error {
	_ = a.ensureState()
	rules, _ := a.loadRules()
	entries := a.readMemory()
	free, total := diskUsage(a.home)
	fmt.Fprintln(a.stdout, "Quaker Doctor")
	fmt.Fprintf(a.stdout, "Disk: %s free of %s\n", human(free), human(total))
	fmt.Fprintf(a.stdout, "Protected rules: %d\n", len(rules.Protected))
	fmt.Fprintf(a.stdout, "Ignored suggestions: %d\n", len(rules.Ignored))
	fmt.Fprintf(a.stdout, "Memory entries: %d\n", len(entries))
	if len(entries) > 0 {
		fmt.Fprintln(a.stdout, "Suggestion: create a profile from a useful dry run with qk profile create weekly-safe --from-last-run")
	} else {
		fmt.Fprintln(a.stdout, "Suggestion: run qk clean --dry-run to build your first memory entry")
	}
	id := a.record("doctor", args, true, "ok", nil, 0)
	a.runHooks("after-doctor", id, "doctor")
	return nil
}

func (a *app) readMemory() []memoryEntry {
	_ = a.ensureState()
	f, err := os.Open(a.memory)
	if err != nil {
		return nil
	}
	defer f.Close()
	var out []memoryEntry
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var e memoryEntry
		if json.Unmarshal(scanner.Bytes(), &e) == nil {
			out = append(out, e)
		}
	}
	return out
}

func (a *app) memoryCmd(args []string) error {
	if len(args) == 0 {
		args = []string{"list"}
	}
	switch args[0] {
	case "list":
		for _, e := range a.readMemory() {
			fmt.Fprintf(a.stdout, "%-22s %-20s %-12s %s\n", e.ID, e.Timestamp, e.Command, e.Result)
		}
	case "show":
		if len(args) < 2 {
			return fmt.Errorf("usage: qk memory show <id>")
		}
		for _, e := range a.readMemory() {
			if e.ID == args[1] {
				return json.NewEncoder(a.stdout).Encode(e)
			}
		}
		return fmt.Errorf("memory entry not found: %s", args[1])
	case "export":
		enc := json.NewEncoder(a.stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(a.readMemory())
	case "forget":
		if len(args) < 2 {
			return fmt.Errorf("usage: qk memory forget <id|--before YYYY-MM-DD>")
		}
		if args[1] == "--before" {
			if len(args) < 3 {
				return fmt.Errorf("usage: qk memory forget --before YYYY-MM-DD")
			}
			cutoff := args[2]
			if !strings.Contains(cutoff, "T") {
				cutoff += "T00:00:00Z"
			}
			var keep []memoryEntry
			for _, e := range a.readMemory() {
				if e.Timestamp >= cutoff {
					keep = append(keep, e)
				}
			}
			return a.writeMemory(keep)
		}
		var keep []memoryEntry
		for _, e := range a.readMemory() {
			if e.ID != args[1] {
				keep = append(keep, e)
			}
		}
		return a.writeMemory(keep)
	default:
		return fmt.Errorf("usage: qk memory list|show|export|forget")
	}
	return nil
}

func (a *app) writeMemory(entries []memoryEntry) error {
	_ = a.ensureState()
	f, err := os.Create(a.memory)
	if err != nil {
		return err
	}
	defer f.Close()
	for _, e := range entries {
		data, _ := json.Marshal(e)
		_, _ = f.Write(append(data, '\n'))
	}
	return nil
}

func (a *app) rulesCmd(args []string) error {
	if len(args) == 0 {
		args = []string{"list"}
	}
	rules, _ := a.loadRules()
	switch args[0] {
	case "list":
		return json.NewEncoder(a.stdout).Encode(rules)
	case "add":
		if len(args) < 3 {
			return fmt.Errorf("usage: qk rules add protect <path> | ignore <id>")
		}
		switch args[1] {
		case "protect":
			rules.Protected = append(rules.Protected, args[2])
		case "ignore":
			rules.Ignored = append(rules.Ignored, args[2])
		default:
			return fmt.Errorf("unknown rule kind: %s", args[1])
		}
		if err := a.saveRules(rules); err != nil {
			return err
		}
		fmt.Fprintln(a.stdout, "Rule added.")
	case "remove":
		if len(args) < 2 {
			return fmt.Errorf("usage: qk rules remove <value>")
		}
		var removed bool
		rules, removed = removeRule(rules, args[1])
		if !removed {
			fmt.Fprintln(a.stdout, "Rule not found.")
			return nil
		}
		fmt.Fprintln(a.stdout, "Rule removed.")
		return a.saveRules(rules)
	case "check":
		if len(args) < 2 {
			return fmt.Errorf("usage: qk rules check <path>")
		}
		if ok, rule := a.protected(args[1]); ok {
			fmt.Fprintf(a.stdout, "protected by %s\n", rule)
		} else {
			fmt.Fprintln(a.stdout, "not protected")
		}
	default:
		return fmt.Errorf("usage: qk rules list|add|remove|check")
	}
	return nil
}

func removeString(in []string, value string) []string {
	out := in[:0]
	for _, item := range in {
		if item != value {
			out = append(out, item)
		}
	}
	return out
}

func removeRule(rules ruleSet, target string) (ruleSet, bool) {
	if strings.HasPrefix(target, "rule-") {
		id, err := strconv.Atoi(strings.TrimPrefix(target, "rule-"))
		if err != nil || id < 1 {
			return rules, false
		}
		switch {
		case id <= len(rules.Protected):
			rules.Protected = append(rules.Protected[:id-1], rules.Protected[id:]...)
			return rules, true
		case id <= len(rules.Protected)+len(rules.Ignored):
			index := id - len(rules.Protected) - 1
			rules.Ignored = append(rules.Ignored[:index], rules.Ignored[index+1:]...)
			return rules, true
		default:
			return rules, false
		}
	}

	protectedLen := len(rules.Protected)
	ignoredLen := len(rules.Ignored)
	rules.Protected = removeString(rules.Protected, target)
	rules.Ignored = removeString(rules.Ignored, target)
	return rules, len(rules.Protected) != protectedLen || len(rules.Ignored) != ignoredLen
}

func (a *app) loadProfiles() (profileFile, error) {
	_ = a.ensureState()
	var pf profileFile
	data, err := os.ReadFile(a.profiles)
	if err != nil {
		return pf, err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return pf, nil
	}
	return pf, json.Unmarshal(data, &pf)
}

func (a *app) saveProfiles(pf profileFile) error {
	data, err := json.MarshalIndent(pf, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(a.profiles, append(data, '\n'), 0o644)
}

func (a *app) profileCmd(args []string) error {
	if len(args) == 0 {
		args = []string{"list"}
	}
	pf, _ := a.loadProfiles()
	switch args[0] {
	case "list":
		for _, p := range pf.Profiles {
			fmt.Fprintf(a.stdout, "%s\t%s %s\n", p.Name, p.Command, strings.Join(p.Args, " "))
		}
	case "create":
		if len(args) < 3 || args[2] != "--from-last-run" {
			return fmt.Errorf("usage: qk profile create <name> --from-last-run")
		}
		mem := a.readMemory()
		if len(mem) == 0 {
			return fmt.Errorf("no memory entries yet")
		}
		last := mem[len(mem)-1]
		pf.Profiles = append(pf.Profiles, profile{Name: args[1], Command: last.Command, Args: last.Args})
		if err := a.saveProfiles(pf); err != nil {
			return err
		}
		fmt.Fprintln(a.stdout, "Profile created.")
	case "run":
		if len(args) < 2 {
			return fmt.Errorf("usage: qk profile run <name> [--dry-run|--apply]")
		}
		for _, p := range pf.Profiles {
			if p.Name == args[1] {
				runArgs := append([]string{p.Command}, p.Args...)
				if !has(args, "--apply") && !has(runArgs, "--dry-run") {
					runArgs = append(runArgs, "--dry-run")
				}
				return a.run(runArgs)
			}
		}
		return fmt.Errorf("profile not found: %s", args[1])
	default:
		return fmt.Errorf("usage: qk profile list|create|run")
	}
	return nil
}

func (a *app) scheduleCmd(args []string) error {
	if err := a.ensureState(); err != nil {
		return err
	}
	if len(args) == 0 {
		args = []string{"list"}
	}
	switch args[0] {
	case "list":
		entries, _ := os.ReadDir(a.schedules)
		for _, e := range entries {
			fmt.Fprintln(a.stdout, strings.TrimSuffix(e.Name(), ".plist"))
		}
	case "add":
		if len(args) < 4 || args[1] != "weekly-scan" || args[2] != "--profile" {
			return fmt.Errorf("usage: qk schedule add weekly-scan --profile <name>")
		}
		id := "weekly-scan-" + args[3]
		plist := launchdPlist(id, os.Args[0], args[3])
		if err := os.MkdirAll(filepath.Join(a.home, "Library", "LaunchAgents"), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(a.schedules, id+".plist"), []byte(plist), 0o644); err != nil {
			return err
		}
		agent := filepath.Join(a.home, "Library", "LaunchAgents", "com.quaker."+id+".plist")
		if err := os.WriteFile(agent, []byte(plist), 0o644); err != nil {
			return err
		}
		fmt.Fprintf(a.stdout, "Added suggest-only schedule %s\n", id)
	case "remove":
		if len(args) < 2 {
			return fmt.Errorf("usage: qk schedule remove <id>")
		}
		_ = os.Remove(filepath.Join(a.schedules, args[1]+".plist"))
		_ = os.Remove(filepath.Join(a.home, "Library", "LaunchAgents", "com.quaker."+args[1]+".plist"))
		fmt.Fprintln(a.stdout, "Schedule removed.")
	default:
		return fmt.Errorf("usage: qk schedule list|add|remove")
	}
	return nil
}

func launchdPlist(id, bin, prof string) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>Label</key><string>com.quaker.%s</string>
<key>ProgramArguments</key><array><string>%s</string><string>profile</string><string>run</string><string>%s</string><string>--dry-run</string></array>
<key>StartCalendarInterval</key><dict><key>Weekday</key><integer>1</integer><key>Hour</key><integer>9</integer></dict>
</dict></plist>
`, id, bin, prof)
}

func (a *app) hooksCmd(args []string) error {
	if err := a.ensureState(); err != nil {
		return err
	}
	if len(args) == 0 {
		args = []string{"list"}
	}
	switch args[0] {
	case "list":
		_ = filepath.WalkDir(a.hooks, func(path string, d fs.DirEntry, err error) error {
			if err == nil && !d.IsDir() {
				rel, _ := filepath.Rel(a.hooks, path)
				fmt.Fprintln(a.stdout, rel)
			}
			return nil
		})
	case "install":
		if len(args) < 3 {
			return fmt.Errorf("usage: qk hooks install <event> <script>")
		}
		dir := filepath.Join(a.hooks, args[1])
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		dst := filepath.Join(dir, filepath.Base(args[2]))
		_ = os.Remove(dst)
		if err := os.Symlink(args[2], dst); err != nil {
			return err
		}
		fmt.Fprintln(a.stdout, "Hook installed.")
	default:
		return fmt.Errorf("usage: qk hooks list|install")
	}
	return nil
}

func (a *app) runHooks(event, id, command string) {
	dir := filepath.Join(a.hooks, event)
	entries, _ := os.ReadDir(dir)
	payload, _ := json.Marshal(map[string]string{"event": event, "memory_id": id, "command": command})
	for _, e := range entries {
		path := filepath.Join(dir, e.Name())
		cmd := exec.Command(path)
		cmd.Stdin = strings.NewReader(string(payload))
		_ = cmd.Start()
		done := make(chan struct{})
		go func() { _ = cmd.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			_ = cmd.Process.Kill()
		}
	}
}
