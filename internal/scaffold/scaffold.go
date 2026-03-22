package scaffold

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

// Options holds the configuration for scaffolding a new project.
type Options struct {
	ProjectName string // e.g. "my-service"
	ModuleName  string // e.g. "github.com/myorg/my-service" or "my-service"
}

// templateData is passed to all templates.
type templateData struct {
	ProjectName string // raw project name, e.g. "my-service"
	ModuleName  string // Go module path
	PackageName string // lowercase, no hyphens, e.g. "myservice"
}

// Generate creates the project directory tree at the current working directory.
func Generate(opts Options) error {
	projectDir := opts.ProjectName
	if _, err := os.Stat(projectDir); err == nil {
		return fmt.Errorf("directory %q already exists", projectDir)
	}

	data := templateData{
		ProjectName: opts.ProjectName,
		ModuleName:  opts.ModuleName,
		PackageName: toPackageName(opts.ProjectName),
	}

	files := projectFiles(data)

	for path, content := range files {
		fullPath := filepath.Join(projectDir, path)

		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			return fmt.Errorf("creating directory for %s: %w", path, err)
		}

		rendered, err := renderTemplate(content, data)
		if err != nil {
			return fmt.Errorf("rendering template %s: %w", path, err)
		}

		if err := os.WriteFile(fullPath, []byte(rendered), 0644); err != nil {
			return fmt.Errorf("writing file %s: %w", path, err)
		}

		fmt.Printf("  ✔ %s\n", path)
	}

	// Create empty directories that need to exist but have no files
	emptyDirs := []string{
		"mysql/deploy",
		"mysql/init",
		"mysql/revert",
		"mysql/verify",
	}
	for _, dir := range emptyDirs {
		fullPath := filepath.Join(projectDir, dir)
		if err := os.MkdirAll(fullPath, 0755); err != nil {
			return fmt.Errorf("creating directory %s: %w", dir, err)
		}
		// add a .gitkeep so the directory is tracked by git
		gitkeep := filepath.Join(fullPath, ".gitkeep")
		if err := os.WriteFile(gitkeep, []byte{}, 0644); err != nil {
			return fmt.Errorf("writing .gitkeep in %s: %w", dir, err)
		}
	}

	return nil
}

func renderTemplate(tmplStr string, data templateData) (string, error) {
	tmpl, err := template.New("").Parse(tmplStr)
	if err != nil {
		return "", err
	}
	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// toPackageName converts a project name like "my-service" → "myservice".
func toPackageName(name string) string {
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, "-", "")
	name = strings.ReplaceAll(name, "_", "")
	return name
}
