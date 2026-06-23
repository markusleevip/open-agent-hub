// openagent is the project onboarding CLI for Open Agent Hub.
//
//	openagent init --server http://localhost:8085 --token pat_xxx [--name myproj]
//	openagent sync [--force]
//	openagent status [--local]
//
// init binds the current directory to a project, writes .openagent/ snapshots,
// injects the managed block into CLAUDE.md/AGENTS.md, and generates .mcp.json.
// sync incrementally refreshes snapshots by ETag; status shows sync state.
//
// .openagent/ has two internal layers:
//   - config.json: project identity (server_url/project_id), generated locally, no credentials;
//   - local/state.json: per-machine sync state (etag/file hash), excluded by .gitignore.
//
// Credentials are stored in ~/.openagent/credentials.json (indexed by server URL),
// never written into the project directory.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	managedBlockBegin = "<!-- openagenthub:begin -->"
	managedBlockEnd   = "<!-- openagenthub:end -->"
	configDir         = ".openagent"
	configFile        = ".openagent/config.json"
	stateFile         = ".openagent/local/state.json"
	formatVersion     = 1

	legacyConfigDir  = ".openagentconfig"
	legacyConfigFile = ".openagentconfig/config.json"
)

// Config is the structure of .openagent/config.json: project identity, no credentials or machine state.
type Config struct {
	FormatVersion int    `json:"format_version"`
	ServerURL     string `json:"server_url"`
	ProjectID     string `json:"project_id,omitempty"`
	// GitRemote is the repository remote URL, the primary identifier for the same project across machines/clones.
	GitRemote string `json:"git_remote,omitempty"`
}

// State is the structure of .openagent/local/state.json: per-machine sync state, must not be committed.
type State struct {
	ETag     string            `json:"etag"`
	Files    map[string]string `json:"files"` // path -> content hash (at last sync time)
	SyncedAt string            `json:"synced_at"`
}

// legacyConfig is the old .openagentconfig/config.json (identity and state mixed), used only for migration.
type legacyConfig struct {
	ServerURL string            `json:"server_url"`
	ProjectID string            `json:"project_id"`
	ETag      string            `json:"etag"`
	Files     map[string]string `json:"files"`
}

type syncFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	Hash    string `json:"hash"`
}

type syncResult struct {
	Changed      bool       `json:"changed"`
	ETag         string     `json:"etag"`
	Files        []syncFile `json:"files"`
	ManagedBlock string     `json:"managed_block"`
	ManagedFiles []string   `json:"managed_files"`
	Project      *struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"project"`
	Hint string `json:"hint"`
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "init":
		err = cmdInit(os.Args[2:])
	case "sync":
		err = cmdSync(os.Args[2:])
	case "status":
		err = cmdStatus(os.Args[2:])
	case "-h", "--help", "help":
		usage()
	default:
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Println(`openagent - Open Agent Hub project CLI

Usage:
  openagent init --server <mcp-base-url> --token <pat_xxx> [--name <project-name>]
  openagent sync [--force]
  openagent status [--local]

init   Bind the current directory to a project, write .openagent/,
       inject the managed block into CLAUDE.md/AGENTS.md, and generate .mcp.json.
sync   Refresh local snapshots (skips when the server ETag is unchanged).
       Also migrates a legacy .openagentconfig/ directory if present.
status Show binding and per-file sync state; also checks the server ETag unless --local is set.

The token is stored in ~/.openagent/credentials.json, never inside the repo.
It can also be supplied via the OPENAGENT_TOKEN environment variable.`)
}

// ---------- commands ----------

func cmdInit(args []string) error {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	server := fs.String("server", "http://localhost:8085", "MCP gateway base URL")
	token := fs.String("token", "", "personal access token (pat_xxx)")
	name := fs.String("name", "", "project name when registering (defaults to directory name)")
	_ = fs.Parse(args)

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	*server = strings.TrimRight(*server, "/")

	if *token != "" {
		if err := saveCredential(*server, *token); err != nil {
			return fmt.Errorf("save credential: %w", err)
		}
	}
	tok, err := resolveToken(*server)
	if err != nil {
		return err
	}

	gitRemote := detectGitRemote(cwd)
	syncArgs := map[string]interface{}{
		"project_path":     cwd,
		"register_project": true,
		"project_name":     *name,
	}
	if gitRemote != "" {
		syncArgs["git_remote"] = gitRemote
	}
	res, err := callSyncProject(*server, tok, syncArgs)
	if err != nil {
		return err
	}

	cfg := &Config{FormatVersion: formatVersion, ServerURL: *server, GitRemote: gitRemote}
	if res.Project != nil {
		cfg.ProjectID = res.Project.ID
	}
	st := &State{Files: map[string]string{}}
	if err := applyBundle(res, st, true); err != nil {
		return err
	}
	st.ETag = res.ETag
	if err := ensureGitignoreEntry(".openagent/"); err != nil {
		fmt.Fprintln(os.Stderr, "warn: failed to update .gitignore:", err)
	}
	if err := writeMcpJSON(*server, cwd); err != nil {
		return err
	}
	if err := saveConfig(cfg); err != nil {
		return err
	}
	if err := saveState(st); err != nil {
		return err
	}

	if res.Project != nil {
		fmt.Printf("✓ bound to project %q (%s)\n", res.Project.Name, res.Project.ID)
	}
	fmt.Println("✓ wrote .openagent/ snapshots")
	fmt.Println("✓ added .openagent/ to .gitignore")
	fmt.Println("✓ injected managed block into CLAUDE.md and AGENTS.md")
	fmt.Println("✓ wrote .mcp.json (uses ${OPENAGENT_TOKEN}; export it before starting your agent)")
	fmt.Println("\nNext: export OPENAGENT_TOKEN=<your pat> and restart your agent client.")
	return nil
}

func cmdSync(args []string) error {
	fs := flag.NewFlagSet("sync", flag.ExitOnError)
	force := fs.Bool("force", false, "overwrite locally modified snapshot files")
	_ = fs.Parse(args)

	cfg, st, migrating, err := loadConfigAndState()
	if err != nil {
		return fmt.Errorf("not initialized (run `openagent init` first): %w", err)
	}
	tok, err := resolveToken(cfg.ServerURL)
	if err != nil {
		return err
	}
	cwd, _ := os.Getwd()

	params := map[string]interface{}{"project_path": cwd}
	if cfg.ProjectID != "" {
		params["project_id"] = cfg.ProjectID
	}
	// Prefer the git remote URL recorded in config; if missing, detect once and backfill config
	// so the server can identify the same repository.
	gitRemote := cfg.GitRemote
	if gitRemote == "" {
		if detected := detectGitRemote(cwd); detected != "" {
			gitRemote = detected
			cfg.GitRemote = detected
			_ = saveConfig(cfg)
		}
	}
	if gitRemote != "" {
		params["git_remote"] = gitRemote
	}
	if !*force {
		params["etag"] = st.ETag
	}
	res, err := callSyncProject(cfg.ServerURL, tok, params)
	if err != nil {
		return err
	}
	if !res.Changed {
		fmt.Println("✓ already up to date (etag", st.ETag+")")
		return nil
	}
	if err := applyBundle(res, st, *force); err != nil {
		return err
	}
	st.ETag = res.ETag
	if cfg.ProjectID == "" && res.Project != nil {
		cfg.ProjectID = res.Project.ID
		if err := saveConfig(cfg); err != nil {
			return err
		}
	}
	if migrating {
		// The legacy directory is entirely generated artifacts; remove it after migration;
		// config.json lands in the new location.
		if err := os.RemoveAll(legacyConfigDir); err != nil {
			fmt.Fprintln(os.Stderr, "warn: failed to remove legacy", legacyConfigDir+":", err)
		} else {
			fmt.Println("✓ migrated", legacyConfigDir, "->", configDir)
		}
		if err := saveConfig(cfg); err != nil {
			return err
		}
	}
	if err := saveState(st); err != nil {
		return err
	}
	fmt.Println("✓ synced (etag", res.ETag+")")
	return nil
}

func cmdStatus(args []string) error {
	fs := flag.NewFlagSet("status", flag.ExitOnError)
	localOnly := fs.Bool("local", false, "only inspect local state; do not call the MCP server")
	_ = fs.Parse(args)

	cfg, st, migrating, err := loadConfigAndState()
	if err != nil {
		return fmt.Errorf("not initialized (run `openagent init` first): %w", err)
	}
	fmt.Println("server:    ", cfg.ServerURL)
	fmt.Println("project_id:", orDash(cfg.ProjectID))
	fmt.Println("git_remote:", orDash(cfg.GitRemote))
	fmt.Println("etag:      ", orDash(st.ETag))
	fmt.Println("synced_at: ", orDash(st.SyncedAt))
	if migrating {
		fmt.Println("note:       legacy " + legacyConfigDir + "/ detected; run `openagent sync` to migrate to " + configDir + "/")
	}
	if !*localOnly {
		if err := printRemoteStatus(cfg, st); err != nil {
			fmt.Println("remote:    check failed (" + err.Error() + ")")
		}
	}
	fmt.Println("files:")
	for path, hash := range st.Files {
		data, err := os.ReadFile(path)
		switch {
		case err != nil:
			fmt.Println("  [missing] ", path)
		case contentHash(data) != hash:
			fmt.Println("  [modified]", path, "(local edits; `openagent sync --force` overwrites)")
		default:
			fmt.Println("  [ok]      ", path)
		}
	}
	return nil
}

func printRemoteStatus(cfg *Config, st *State) error {
	tok, err := resolveToken(cfg.ServerURL)
	if err != nil {
		return err
	}
	cwd, _ := os.Getwd()
	params := map[string]interface{}{"project_path": cwd}
	if cfg.ProjectID != "" {
		params["project_id"] = cfg.ProjectID
	}
	gitRemote := cfg.GitRemote
	if gitRemote == "" {
		gitRemote = detectGitRemote(cwd)
	}
	if gitRemote != "" {
		params["git_remote"] = gitRemote
	}
	if st.ETag != "" {
		params["etag"] = st.ETag
	}

	res, err := callSyncProject(cfg.ServerURL, tok, params)
	if err != nil {
		return err
	}
	if !res.Changed {
		fmt.Println("remote:    up to date")
		return nil
	}
	fmt.Println("remote:    newer snapshot available (run `openagent sync`)")
	fmt.Println("remote_etag:", orDash(res.ETag))
	return nil
}

// ---------- bundle application ----------

// applyBundle writes snapshot files, deletes snapshots no longer present on the server,
// and updates the managed block.
// When force is false, locally modified files are skipped (to avoid overwriting user edits).
func applyBundle(res *syncResult, st *State, force bool) error {
	if res.Hint != "" {
		fmt.Fprintln(os.Stderr, "hint:", res.Hint)
	}
	current := map[string]bool{}
	for _, f := range res.Files {
		current[f.Path] = true
		if !force {
			if prev, tracked := st.Files[f.Path]; tracked {
				if data, err := os.ReadFile(f.Path); err == nil && contentHash(data) != prev {
					fmt.Fprintf(os.Stderr, "skip %s: locally modified (use --force to overwrite)\n", f.Path)
					continue
				}
			}
		}
		if err := os.MkdirAll(filepath.Dir(f.Path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(f.Path, []byte(f.Content), 0o644); err != nil {
			return err
		}
		st.Files[f.Path] = f.Hash
	}

	// Deletion detection: snapshot files written in the previous sync but no longer
	// included in the current bundle.
	// Only touches files inside the snapshot directory whose content has not been
	// manually edited (modified files are kept with a warning; force deletes them).
	for path, prev := range st.Files {
		if current[path] {
			continue
		}
		if !strings.HasPrefix(path, configDir+"/") && !strings.HasPrefix(path, legacyConfigDir+"/") {
			delete(st.Files, path)
			continue
		}
		data, err := os.ReadFile(path)
		if err == nil && contentHash(data) != prev && !force {
			fmt.Fprintf(os.Stderr, "keep %s: removed on server but locally modified (use --force to delete)\n", path)
			continue
		}
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove stale %s: %w", path, err)
		}
		delete(st.Files, path)
		pruneEmptyDirs(filepath.Dir(path))
	}

	for _, doc := range res.ManagedFiles {
		if err := upsertManagedBlock(doc, res.ManagedBlock); err != nil {
			return fmt.Errorf("update %s: %w", doc, err)
		}
	}
	st.SyncedAt = time.Now().UTC().Format(time.RFC3339)
	return nil
}

// pruneEmptyDirs removes empty directories inside the snapshot directory bottom-up,
// up to the configDir/legacyConfigDir root.
func pruneEmptyDirs(dir string) {
	for dir != "." && dir != configDir && dir != legacyConfigDir {
		if os.Remove(dir) != nil { // non-empty or other error: stop
			return
		}
		dir = filepath.Dir(dir)
	}
}

func upsertManagedBlock(path, block string) error {
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	doc := string(data)
	begin := strings.Index(doc, managedBlockBegin)
	end := strings.Index(doc, managedBlockEnd)
	var out string
	switch {
	case begin >= 0 && end > begin:
		out = doc[:begin] + block + doc[end+len(managedBlockEnd):]
	case strings.TrimSpace(doc) == "":
		out = block + "\n"
	default:
		out = strings.TrimRight(doc, "\n") + "\n\n" + block + "\n"
	}
	return os.WriteFile(path, []byte(out), 0o644)
}

// ensureGitignoreEntry ensures the given entry exists in .gitignore.
// Only operates when a .git/ directory exists (git repo); skipped for non-git repos.
// Skips if the entry already exists; appends otherwise.
func ensureGitignoreEntry(entry string) error {
	if _, err := os.Stat(".git"); err != nil {
		return nil // not a git repo, skip
	}
	data, err := os.ReadFile(".gitignore")
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	content := string(data)
	for _, line := range strings.Split(content, "\n") {
		if strings.TrimSpace(line) == entry {
			return nil // already exists
		}
	}
	if content != "" && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	content += "\n# Open Agent Hub local snapshots\n" + entry + "\n"
	return os.WriteFile(".gitignore", []byte(content), 0o644)
}

// writeMcpJSON merges and writes .mcp.json: preserves existing entries, only adds/updates openagenthub.
// The token uses ${OPENAGENT_TOKEN} environment variable expansion to avoid plaintext in the repo.
func writeMcpJSON(serverURL, cwd string) error {
	root := map[string]interface{}{}
	if data, err := os.ReadFile(".mcp.json"); err == nil {
		if err := json.Unmarshal(data, &root); err != nil {
			return fmt.Errorf(".mcp.json exists but is not valid JSON: %w", err)
		}
	}
	servers, _ := root["mcpServers"].(map[string]interface{})
	if servers == nil {
		servers = map[string]interface{}{}
	}
	servers["openagenthub"] = map[string]interface{}{
		"type": "http",
		"url":  serverURL + "/mcp",
		"headers": map[string]interface{}{
			"Authorization":  "Bearer ${OPENAGENT_TOKEN}",
			"X-Project-Path": cwd,
		},
	}
	root["mcpServers"] = servers
	out, _ := json.MarshalIndent(root, "", "  ")
	return os.WriteFile(".mcp.json", append(out, '\n'), 0o644)
}

// ---------- config & credentials ----------

// loadConfigAndState reads identity and per-machine state. When the new layout is missing
// but a legacy .openagentconfig/config.json exists, constructs both from the old file
// and returns migrating=true (actual migration is completed by sync).
func loadConfigAndState() (*Config, *State, bool, error) {
	if data, err := os.ReadFile(configFile); err == nil {
		var cfg Config
		if err := json.Unmarshal(data, &cfg); err != nil {
			return nil, nil, false, fmt.Errorf("parse %s: %w", configFile, err)
		}
		if cfg.FormatVersion > formatVersion {
			return nil, nil, false, fmt.Errorf("%s has format_version %d; this openagent build supports up to %d (please upgrade)", configFile, cfg.FormatVersion, formatVersion)
		}
		st := &State{Files: map[string]string{}}
		if data, err := os.ReadFile(stateFile); err == nil {
			_ = json.Unmarshal(data, st)
			if st.Files == nil {
				st.Files = map[string]string{}
			}
		}
		return &cfg, st, false, nil
	}

	data, err := os.ReadFile(legacyConfigFile)
	if err != nil {
		return nil, nil, false, err
	}
	var legacy legacyConfig
	if err := json.Unmarshal(data, &legacy); err != nil {
		return nil, nil, false, fmt.Errorf("parse %s: %w", legacyConfigFile, err)
	}
	cfg := &Config{FormatVersion: formatVersion, ServerURL: legacy.ServerURL, ProjectID: legacy.ProjectID}
	st := &State{Files: legacy.Files}
	if st.Files == nil {
		st.Files = map[string]string{}
	}
	// Intentionally omit the old ETag: the path is fully migrated, a full pull is required.
	return cfg, st, true, nil
}

func saveConfig(cfg *Config) error {
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return err
	}
	out, _ := json.MarshalIndent(cfg, "", "  ")
	return os.WriteFile(configFile, append(out, '\n'), 0o644)
}

func saveState(st *State) error {
	if err := os.MkdirAll(filepath.Dir(stateFile), 0o755); err != nil {
		return err
	}
	out, _ := json.MarshalIndent(st, "", "  ")
	return os.WriteFile(stateFile, append(out, '\n'), 0o644)
}

func credentialsPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".openagent", "credentials.json"), nil
}

func saveCredential(serverURL, token string) error {
	path, err := credentialsPath()
	if err != nil {
		return err
	}
	creds := map[string]string{}
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &creds)
	}
	creds[serverURL] = token
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	out, _ := json.MarshalIndent(creds, "", "  ")
	return os.WriteFile(path, append(out, '\n'), 0o600)
}

func resolveToken(serverURL string) (string, error) {
	if tok := os.Getenv("OPENAGENT_TOKEN"); tok != "" {
		return tok, nil
	}
	path, err := credentialsPath()
	if err == nil {
		if data, err := os.ReadFile(path); err == nil {
			creds := map[string]string{}
			if json.Unmarshal(data, &creds) == nil && creds[serverURL] != "" {
				return creds[serverURL], nil
			}
		}
	}
	return "", fmt.Errorf("no token for %s: pass --token, set OPENAGENT_TOKEN, or check ~/.openagent/credentials.json", serverURL)
}

// ---------- MCP transport ----------

// callSyncProject calls hub.sync_project via JSON-RPC tools/call.
// The gateway wraps the tool result as MCP content (JSON embedded in text),
// so we need to unwrap two layers here.
func callSyncProject(serverURL, token string, args map[string]interface{}) (*syncResult, error) {
	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      time.Now().UnixMilli(),
		"method":  "tools/call",
		"params":  map[string]interface{}{"name": "hub.sync_project", "arguments": args},
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequest(http.MethodPost, serverURL+"/mcp", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cannot reach %s: %w", serverURL, err)
	}
	defer resp.Body.Close()

	var rpc struct {
		Error *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
		Result struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
			IsError bool `json:"isError"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rpc); err != nil {
		return nil, fmt.Errorf("invalid gateway response: %w", err)
	}
	if rpc.Error != nil {
		return nil, fmt.Errorf("gateway error %d: %s", rpc.Error.Code, rpc.Error.Message)
	}
	if len(rpc.Result.Content) == 0 {
		return nil, fmt.Errorf("empty tool result")
	}
	if rpc.Result.IsError {
		return nil, fmt.Errorf("tool error: %s", rpc.Result.Content[0].Text)
	}
	var res syncResult
	if err := json.Unmarshal([]byte(rpc.Result.Content[0].Text), &res); err != nil {
		return nil, fmt.Errorf("invalid tool payload: %w", err)
	}
	return &res, nil
}

// ---------- helpers ----------

// contentHash keeps the same algorithm as the server's newSyncFile: first 32 chars of sha256 hex.
func contentHash(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])[:32]
}

// detectGitRemote returns the origin remote URL of the repository (raw URL,
// normalization is left to the server).
// Returns empty string for non-git repos or when no origin exists; this is not treated as an error.
func detectGitRemote(dir string) string {
	cmd := exec.Command("git", "-C", dir, "remote", "get-url", "origin")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
