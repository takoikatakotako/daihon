// Package parser loads movie.yml and the character files it imports.
package parser

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/takoikatakotako/daihon/internal/schema"
	"gopkg.in/yaml.v3"
)

// Project is a loaded movie.yml together with the directory it was found in.
// Every asset path in Movie is relative to Root.
type Project struct {
	Root  string
	File  string
	Movie *schema.Movie
}

// Load reads movie.yml, merges its imports and applies defaults.
//
// Imports are layered first and the main file wins on conflict, so a project
// can pull in a shared character pack and still tweak one field locally.
func Load(path string) (*Project, error) {
	root := filepath.Dir(path)

	main, err := decodeFile(path)
	if err != nil {
		return nil, err
	}

	merged := &schema.Movie{Characters: map[string]*schema.Character{}}
	for _, imp := range main.Imports {
		impPath := filepath.Join(root, imp)
		sub, err := decodeFile(impPath)
		if err != nil {
			return nil, fmt.Errorf("import %s: %w", imp, err)
		}
		// A character pack should be self-contained, so its asset paths are
		// written relative to the pack itself. Rebase them onto the project
		// root now, and everything downstream sees one coordinate system.
		rebaseCharacters(sub.Characters, filepath.Dir(impPath), root)
		for id, c := range sub.Characters {
			merged.Characters[id] = c
		}
		for name, p := range sub.Fonts {
			if merged.Fonts == nil {
				merged.Fonts = map[string]string{}
			}
			merged.Fonts[name] = rebase(p, filepath.Dir(impPath), root)
		}
	}

	for id, c := range main.Characters {
		merged.Characters[id] = c
	}
	main.Characters = merged.Characters
	if main.Fonts == nil {
		main.Fonts = merged.Fonts
	} else {
		for name, p := range merged.Fonts {
			if _, ok := main.Fonts[name]; !ok {
				main.Fonts[name] = p
			}
		}
	}

	main.ApplyDefaults()
	return &Project{Root: root, File: path, Movie: main}, nil
}

func decodeFile(path string) (*schema.Movie, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m schema.Movie
	if err := yaml.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("%s: %w", filepath.Base(path), err)
	}
	return &m, nil
}

func rebaseCharacters(cs map[string]*schema.Character, from, root string) {
	for _, c := range cs {
		for name, p := range c.Expressions {
			c.Expressions[name] = rebase(p, from, root)
		}
		if c.Lipsync != nil {
			c.Lipsync.Open = rebase(c.Lipsync.Open, from, root)
			c.Lipsync.Close = rebase(c.Lipsync.Close, from, root)
		}
	}
}

// rebase rewrites a path written relative to `from` so that it is relative to `root`.
func rebase(p, from, root string) string {
	if p == "" || filepath.IsAbs(p) {
		return p
	}
	abs := filepath.Join(from, p)
	rel, err := filepath.Rel(root, abs)
	if err != nil {
		return p
	}
	return rel
}

// Abs resolves a project-relative asset path against the project root.
func (p *Project) Abs(rel string) string {
	if rel == "" {
		return ""
	}
	if filepath.IsAbs(rel) {
		return rel
	}
	return filepath.Join(p.Root, rel)
}
