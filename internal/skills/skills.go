// Package skills discovers reusable instruction "skills" stored on disk as
// */SKILL.md files. Each skill is a directory containing a SKILL.md whose
// optional YAML-ish frontmatter carries a name/description and whose markdown
// body is the skill content the model can pull in on demand (PRD F15).
//
// The loader is deliberately dependency-free: frontmatter is hand-parsed (no
// YAML library) and malformed files are skipped rather than failing the whole
// load, so a single bad skill never hides the good ones.
package skills

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Skill is a single discovered skill. Name and Description come from the
// SKILL.md frontmatter (Name falls back to the directory name); Content is the
// markdown body; Path is the absolute path to the SKILL.md file.
type Skill struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Content     string `json:"content,omitempty"`
	Path        string `json:"path"`
}

const skillFileName = "SKILL.md"

// DefaultDir resolves the skills directory, mirroring sessions.DefaultRoot. An
// explicit ZERO_SKILLS_DIR override wins; otherwise it is
// $XDG_DATA_HOME/zero/skills or ~/.local/share/zero/skills. The directory is
// NOT created — a missing directory simply yields no skills.
func DefaultDir(env map[string]string) string {
	if override := strings.TrimSpace(envValue(env, "ZERO_SKILLS_DIR")); override != "" {
		return override
	}
	dataHome := strings.TrimSpace(envValue(env, "XDG_DATA_HOME"))
	home := strings.TrimSpace(envValue(env, "HOME"))
	if home == "" {
		if userHome, err := os.UserHomeDir(); err == nil {
			home = userHome
		}
	}
	base := dataHome
	if base == "" {
		base = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(base, "zero", "skills")
}

// Load scans dir for */SKILL.md files and returns the parsed skills sorted by
// name. A missing directory yields an empty slice with no error; individual
// malformed skill files are skipped rather than failing the whole load.
//
// NOTE: Load currently scans a single root (ZERO_SKILLS_DIR / the data dir).
// Plugin-declared skill paths (the plugins manifest "skills" array) are NOT yet
// merged into this lookup; multi-root loading is tracked as a separate feature.
func Load(dir string) ([]Skill, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return []Skill{}, nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []Skill{}, nil
		}
		return nil, err
	}

	skills := make([]Skill, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		manifestPath := filepath.Join(dir, entry.Name(), skillFileName)
		data, err := os.ReadFile(manifestPath)
		if err != nil {
			// Missing or unreadable SKILL.md (including the case where it is a
			// directory) is skipped, not fatal — one bad skill must not hide the rest.
			continue
		}
		absPath := manifestPath
		if resolved, absErr := filepath.Abs(manifestPath); absErr == nil {
			absPath = resolved
		}
		skills = append(skills, parseSkill(entry.Name(), absPath, string(data)))
	}

	sort.Slice(skills, func(left int, right int) bool {
		return skills[left].Name < skills[right].Name
	})
	return skills, nil
}

// List loads the skills directory and returns each skill without its (possibly
// large) Content body — handy for `zero skills` listings.
func List(dir string) ([]Skill, error) {
	loaded, err := Load(dir)
	if err != nil {
		return nil, err
	}
	listed := make([]Skill, 0, len(loaded))
	for _, skill := range loaded {
		skill.Content = ""
		listed = append(listed, skill)
	}
	return listed, nil
}

// Get loads the named skill from dir, returning false if it is not found.
func Get(dir string, name string) (Skill, bool) {
	loaded, err := Load(dir)
	if err != nil {
		return Skill{}, false
	}
	target := strings.TrimSpace(name)
	for _, skill := range loaded {
		if skill.Name == target {
			return skill, true
		}
	}
	return Skill{}, false
}

// parseSkill splits optional `---`-delimited frontmatter from the markdown body.
// Frontmatter is a simple line parser for `name:`/`description:` keys (no YAML
// dependency). Without frontmatter, Name defaults to the directory name and
// Description is empty.
func parseSkill(dirName string, path string, raw string) Skill {
	body := raw
	name := dirName
	description := ""

	normalized := strings.ReplaceAll(raw, "\r\n", "\n")
	if frontmatter, remainder, ok := splitFrontmatter(normalized); ok {
		body = remainder
		if parsedName := frontmatterValue(frontmatter, "name"); parsedName != "" {
			name = parsedName
		}
		description = frontmatterValue(frontmatter, "description")
	}

	return Skill{
		Name:        name,
		Description: description,
		Content:     strings.TrimSpace(body),
		Path:        path,
	}
}

// splitFrontmatter detects a leading `---` line, captures lines up to the
// closing `---`, and returns the frontmatter block plus the remaining body. It
// reports ok=false when there is no opening delimiter or no closing delimiter
// (in which case the whole input is treated as body).
func splitFrontmatter(normalized string) (string, string, bool) {
	if !strings.HasPrefix(normalized, "---\n") && normalized != "---" {
		return "", "", false
	}
	lines := strings.Split(normalized, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return "", "", false
	}
	for index := 1; index < len(lines); index++ {
		if strings.TrimSpace(lines[index]) == "---" {
			frontmatter := strings.Join(lines[1:index], "\n")
			body := strings.Join(lines[index+1:], "\n")
			return frontmatter, body, true
		}
	}
	// No closing delimiter — not valid frontmatter; treat the whole file as body.
	return "", "", false
}

// frontmatterValue reads a single `key: value` pair from the frontmatter block.
// Matching is case-insensitive on the key; the first occurrence wins.
func frontmatterValue(frontmatter string, key string) string {
	prefix := strings.ToLower(key) + ":"
	for _, line := range strings.Split(frontmatter, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToLower(trimmed), prefix) {
			value := strings.TrimSpace(trimmed[len(prefix):])
			return strings.Trim(value, `"'`)
		}
	}
	return ""
}

func envValue(env map[string]string, key string) string {
	if env != nil {
		return env[key]
	}
	return os.Getenv(key)
}
