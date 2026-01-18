package main

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"path/filepath"
	"strings"
)

// HTMLEngine is a custom template engine for Fiber
type HTMLEngine struct {
	templates *template.Template
	fs        fs.FS
}

// NewHTMLEngine creates a new HTML template engine
func NewHTMLEngine(fsys fs.FS) *HTMLEngine {
	engine := &HTMLEngine{
		fs: fsys,
	}
	engine.load()
	return engine
}

// load loads all templates from the filesystem
func (e *HTMLEngine) load() {
	e.templates = template.New("")

	// Define template functions
	e.templates.Funcs(template.FuncMap{
		"safeHTML": func(s string) template.HTML {
			return template.HTML(s)
		},
		"formatTime": func(t interface{}) string {
			return fmt.Sprintf("%v", t)
		},
	})

	// Walk through all template files
	fs.WalkDir(e.fs, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".html") {
			return nil
		}

		// Read template content
		content, err := fs.ReadFile(e.fs, path)
		if err != nil {
			return err
		}

		// Parse template with its path as name (without .html extension)
		name := strings.TrimSuffix(path, ".html")
		name = strings.ReplaceAll(name, "\\", "/") // Normalize path separators

		_, err = e.templates.New(name).Parse(string(content))
		if err != nil {
			fmt.Printf("Error parsing template %s: %v\n", name, err)
		}

		return nil
	})
}

// Render renders a template
func (e *HTMLEngine) Render(w io.Writer, name string, binding interface{}, layout ...string) error {
	// Normalize the name
	name = strings.ReplaceAll(name, "\\", "/")

	// Get the template
	tmpl := e.templates.Lookup(name)
	if tmpl == nil {
		return fmt.Errorf("template %s not found", name)
	}

	// Check if we have a layout
	if len(layout) > 0 && layout[0] != "" {
		layoutName := strings.ReplaceAll(layout[0], "\\", "/")
		layoutTmpl := e.templates.Lookup(layoutName)
		if layoutTmpl != nil {
			// Render the content template first
			var contentBuf bytes.Buffer
			if err := tmpl.Execute(&contentBuf, binding); err != nil {
				return err
			}

			// Add content to binding
			data := make(map[string]interface{})
			if m, ok := binding.(map[string]interface{}); ok {
				for k, v := range m {
					data[k] = v
				}
			}
			data["Content"] = template.HTML(contentBuf.String())

			// Render layout with content
			return layoutTmpl.Execute(w, data)
		}
	}

	// Render without layout
	return tmpl.Execute(w, binding)
}

// Load reloads templates (for development)
func (e *HTMLEngine) Load() error {
	e.load()
	return nil
}

// templatePath normalizes a template path
func templatePath(name string) string {
	// Clean the path
	name = filepath.Clean(name)
	// Remove .html if present
	name = strings.TrimSuffix(name, ".html")
	// Normalize separators
	return strings.ReplaceAll(name, "\\", "/")
}
