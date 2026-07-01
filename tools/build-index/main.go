// Command build-index generates index.json — the flat catalog a Meshploy install
// fetches to render the one-click template gallery.
//
// It walks templates/<id>/{meta.yaml,docker-compose.yml}, validates each, and
// writes index.json at the repo root:
//
//	{ "version": 1, "templates": [ <manifest>, ... ] }
//
// Each entry is the full meta.yaml (id, name, description, category, version,
// icon, links, maintainers, variables) — everything the gallery and deploy
// dialog need. The compose is NOT inlined; a Meshploy install fetches a
// template's docker-compose.yml lazily when it deploys.
//
// Usage (run from the repo root):
//
//	go run ./tools/build-index           # validate + (re)write index.json
//	go run ./tools/build-index --check   # validate only; fail if index.json is stale (CI on PRs)
//
// No timestamp is written, so index.json changes only when a template changes —
// keeping the CI commit meaningful and the diff clean.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	templatesDir = "templates"
	indexPath    = "index.json"
)

var (
	categories    = map[string]bool{"database": true, "cms": true, "analytics": true, "queue": true, "monitoring": true, "application": true}
	generators    = map[string]bool{"password": true, "secret64": true, "hex32": true, "uuid": true, "subdomain": true}
	placeholderRe = regexp.MustCompile(`\$\{([A-Z0-9_]+)\}`)
	imageRe       = regexp.MustCompile(`(?m)^\s*image:\s*['"]?([^'"#\s]+)`)
)

// manifest mirrors meta.yaml. The json tags MUST match the product's
// templates.Manifest so a Meshploy install can unmarshal index.json directly.
type manifest struct {
	ID          string     `yaml:"id"          json:"id"`
	Name        string     `yaml:"name"        json:"name"`
	Description string     `yaml:"description" json:"description"`
	Category    string     `yaml:"category"    json:"category"`
	Version     string     `yaml:"version"     json:"version"`
	Icon        string     `yaml:"icon"        json:"icon"`
	Links       links      `yaml:"links"       json:"links"`
	Maintainers []string   `yaml:"maintainers" json:"maintainers,omitempty"`
	Variables   []variable `yaml:"variables"   json:"variables"`
}

type links struct {
	Website string `yaml:"website" json:"website,omitempty"`
	Source  string `yaml:"source"  json:"source,omitempty"`
}

type variable struct {
	Key      string  `yaml:"key"      json:"key"`
	Prompt   string  `yaml:"prompt"   json:"prompt,omitempty"`
	Required bool    `yaml:"required" json:"required,omitempty"`
	Generate string  `yaml:"generate" json:"generate,omitempty"`
	Expose   *expose `yaml:"expose"   json:"expose,omitempty"`
}

type expose struct {
	Service string `yaml:"service" json:"service"`
	Port    int    `yaml:"port"    json:"port"`
}

type catalog struct {
	Version   int         `json:"version"`
	Templates []*manifest `json:"templates"`
}

func main() {
	check := flag.Bool("check", false, "validate only; fail if index.json is stale")
	flag.Parse()

	manifests, errs := loadAndValidate()
	if len(errs) > 0 {
		fmt.Fprintln(os.Stderr, "index build FAILED:")
		fmt.Fprintln(os.Stderr)
		for _, e := range errs {
			fmt.Fprintf(os.Stderr, "  [x] %s\n", e)
		}
		os.Exit(1)
	}

	rendered, err := render(&catalog{Version: 1, Templates: manifests})
	if err != nil {
		fmt.Fprintf(os.Stderr, "render index.json: %v\n", err)
		os.Exit(1)
	}

	if *check {
		existing, _ := os.ReadFile(indexPath)
		if !bytes.Equal(existing, rendered) {
			fmt.Fprintln(os.Stderr, "index build FAILED:")
			fmt.Fprintln(os.Stderr)
			fmt.Fprintln(os.Stderr, "  [x] index.json is stale — run `go run ./tools/build-index` and commit the result.")
			os.Exit(1)
		}
		fmt.Printf("OK: %d template(s) valid; index.json up to date\n", len(manifests))
		return
	}

	if err := os.WriteFile(indexPath, rendered, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "write index.json: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("OK: wrote index.json with %d template(s)\n", len(manifests))
}

// render marshals the catalog with a trailing newline, matching what --check compares.
func render(c *catalog) ([]byte, error) {
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

func loadAndValidate() ([]*manifest, []string) {
	entries, err := os.ReadDir(templatesDir)
	if err != nil {
		return nil, []string{fmt.Sprintf("read %s/: %v", templatesDir, err)}
	}

	var manifests []*manifest
	var errs []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		m, tErrs := validateTemplate(e.Name())
		errs = append(errs, tErrs...)
		if m != nil {
			manifests = append(manifests, m)
		}
	}
	sort.Slice(manifests, func(i, j int) bool { return manifests[i].ID < manifests[j].ID })
	return manifests, errs
}

func validateTemplate(id string) (*manifest, []string) {
	var errs []string
	dir := filepath.Join(templatesDir, id)
	metaPath := filepath.Join(dir, "meta.yaml")
	composePath := filepath.Join(dir, "docker-compose.yml")

	metaB, err := os.ReadFile(metaPath)
	if err != nil {
		return nil, []string{fmt.Sprintf("%s: missing or unreadable meta.yaml", id)}
	}
	composeB, err := os.ReadFile(composePath)
	if err != nil {
		return nil, []string{fmt.Sprintf("%s: missing or unreadable docker-compose.yml", id)}
	}

	var m manifest
	if err := yaml.Unmarshal(metaB, &m); err != nil {
		return nil, []string{fmt.Sprintf("%s: meta.yaml is not valid YAML: %v", id, err)}
	}

	// ── Manifest fields ──────────────────────────────────────────────────────
	if m.ID != id {
		errs = append(errs, fmt.Sprintf("%s: meta.yaml id (%q) must equal the directory name", id, m.ID))
	}
	if m.Name == "" {
		errs = append(errs, fmt.Sprintf("%s: meta.yaml missing required field 'name'", id))
	}
	if m.Description == "" {
		errs = append(errs, fmt.Sprintf("%s: meta.yaml missing required field 'description'", id))
	}
	if m.Version == "" {
		errs = append(errs, fmt.Sprintf("%s: meta.yaml missing required field 'version'", id))
	}
	if m.Category == "" {
		errs = append(errs, fmt.Sprintf("%s: meta.yaml missing required field 'category'", id))
	} else if !categories[m.Category] {
		errs = append(errs, fmt.Sprintf("%s: category %q is not a known category", id, m.Category))
	}

	// ── Variables ────────────────────────────────────────────────────────────
	keys := map[string]bool{}    // all declared keys
	envKeys := map[string]bool{} // keys that must appear as ${VAR} (env-substitution)
	for i, v := range m.Variables {
		if v.Key == "" {
			errs = append(errs, fmt.Sprintf("%s: variable #%d has no key", id, i))
			continue
		}
		if keys[v.Key] {
			errs = append(errs, fmt.Sprintf("%s: duplicate variable key %q", id, v.Key))
		}
		keys[v.Key] = true

		hasPrompt := v.Prompt != ""
		hasGen := v.Generate != ""
		if hasPrompt == hasGen {
			errs = append(errs, fmt.Sprintf("%s: variable %q must be exactly one of 'prompt' or 'generate'", id, v.Key))
		}
		if hasGen && !generators[v.Generate] {
			errs = append(errs, fmt.Sprintf("%s: variable %q has unknown generator %q", id, v.Key, v.Generate))
		}
		// subdomain vars drive routing, not env substitution — exempt from needing a ${VAR}.
		if v.Generate != "subdomain" {
			envKeys[v.Key] = true
		}
	}

	// ── Variable ↔ placeholder bijection ─────────────────────────────────────
	compose := string(composeB)
	placeholders := map[string]bool{}
	for _, mm := range placeholderRe.FindAllStringSubmatch(compose, -1) {
		placeholders[mm[1]] = true
	}
	for ph := range placeholders {
		if !keys[ph] {
			errs = append(errs, fmt.Sprintf("%s: compose uses ${%s} with no matching variable", id, ph))
		}
	}
	for k := range envKeys {
		if !placeholders[k] {
			errs = append(errs, fmt.Sprintf("%s: variable %q is never used as ${%s} in the compose", id, k, k))
		}
	}

	// ── Pinned image tags ────────────────────────────────────────────────────
	for _, mm := range imageRe.FindAllStringSubmatch(compose, -1) {
		ref := mm[1]
		if strings.Contains(ref, "${") {
			continue // image comes from a variable — can't check statically
		}
		if !imagePinned(ref) {
			errs = append(errs, fmt.Sprintf("%s: image %q is not pinned (add an explicit non-latest tag)", id, ref))
		}
	}

	return &m, errs
}

// imagePinned reports whether ref carries an explicit, non-"latest" tag. It
// inspects only the segment after the last '/' so a registry host:port prefix
// isn't mistaken for a tag.
func imagePinned(ref string) bool {
	name := ref
	if i := strings.LastIndex(ref, "/"); i >= 0 {
		name = ref[i+1:]
	}
	i := strings.LastIndex(name, ":")
	if i < 0 {
		return false // no tag → resolves to :latest
	}
	return !strings.EqualFold(name[i+1:], "latest")
}
