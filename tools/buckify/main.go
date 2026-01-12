// Command buckify generates BUCK files for Go packages under vendor.
package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type packageInfo struct {
	packageName string
	imports     map[string]struct{}
}

// main delegates to run and exits on failure.
func main() {
	if err := run(); err != nil {
		slog.Error("buckify failed", "error", err)
		os.Exit(1)
	}
}

// run orchestrates BUCK generation and reports progress.
func run() error {
	rootDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("read working directory: %w", err)
	}

	vendorDir := filepath.Join(rootDir, "vendor")
	packages, err := collectPackages(vendorDir)
	if err != nil {
		return fmt.Errorf("collect vendor packages: %w", err)
	}

	nameByImport := buildNameByImport(packages)
	relPaths := sortedRelPaths(packages)
	for _, rel := range relPaths {
		info := packages[rel]
		buckRel, deps, err := writeBuck(vendorDir, rel, info, nameByImport)
		if err != nil {
			return fmt.Errorf("write BUCK for %s: %w", rel, err)
		}
		slog.Info("generated BUCK", "path", filepath.ToSlash(buckRel), "deps", formatDeps(deps))
	}

	slog.Info("generated BUCK files", "count", len(relPaths))
	return nil
}

// buildNameByImport maps vendor import paths to Buck target names.
func buildNameByImport(packages map[string]*packageInfo) map[string]string {
	nameByImport := make(map[string]string, len(packages))
	for rel, info := range packages {
		name := info.packageName
		if name == "" {
			name = path.Base(rel)
		}
		nameByImport[rel] = name
	}
	return nameByImport
}

// sortedRelPaths returns vendor import paths in sorted order.
func sortedRelPaths(packages map[string]*packageInfo) []string {
	relPaths := make([]string, 0, len(packages))
	for rel := range packages {
		relPaths = append(relPaths, rel)
	}
	sort.Strings(relPaths)
	return relPaths
}

// collectPackages discovers Go packages under vendor and extracts imports.
func collectPackages(vendorDir string) (map[string]*packageInfo, error) {
	packages := make(map[string]*packageInfo)

	err := filepath.WalkDir(vendorDir, func(dirPath string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !d.IsDir() {
			return nil
		}

		rel, err := filepath.Rel(vendorDir, dirPath)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == "." {
			return nil
		}

		info, ok, err := parseDir(dirPath)
		if err != nil {
			return fmt.Errorf("parse %s: %w", rel, err)
		}
		if !ok {
			return nil
		}

		packages[rel] = info
		return nil
	})

	if err != nil {
		return nil, err
	}

	return packages, nil
}

// parseDir inspects non-test Go files in a directory to collect imports.
func parseDir(dir string) (*packageInfo, bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, false, err
	}

	info := &packageInfo{imports: make(map[string]struct{})}
	fileSet := token.NewFileSet()

	for _, entry := range entries {
		if shouldSkipEntry(entry) {
			continue
		}

		filePath := filepath.Join(dir, entry.Name())
		file, err := parser.ParseFile(fileSet, filePath, nil, parser.ImportsOnly)
		if err != nil {
			return nil, false, err
		}

		if err := ensurePackageName(info, file.Name.Name); err != nil {
			return nil, false, err
		}

		if err := addImports(info, file.Imports); err != nil {
			return nil, false, err
		}
	}

	if info.packageName == "" {
		return nil, false, nil
	}

	return info, true, nil
}

// shouldSkipEntry reports whether a directory entry is not a Go source file to parse.
func shouldSkipEntry(entry os.DirEntry) bool {
	if entry.IsDir() {
		return true
	}
	name := entry.Name()
	return !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go")
}

// ensurePackageName verifies consistent package naming across files.
func ensurePackageName(info *packageInfo, name string) error {
	if info.packageName == "" {
		info.packageName = name
		return nil
	}
	if info.packageName != name {
		return fmt.Errorf("package name mismatch: %s vs %s", info.packageName, name)
	}
	return nil
}

// addImports collects non-cgo imports from parsed files.
func addImports(info *packageInfo, imports []*ast.ImportSpec) error {
	for _, imp := range imports {
		pathValue, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			return err
		}
		if pathValue == "C" {
			continue
		}
		info.imports[pathValue] = struct{}{}
	}
	return nil
}

// formatDeps renders deps as a compact list without spaces for log output.
func formatDeps(deps []string) string {
	if len(deps) == 0 {
		return "[]"
	}
	return "[" + strings.Join(deps, ",") + "]"
}

// writeBuck writes the go_library rule for a vendor package.
func writeBuck(vendorDir, rel string, info *packageInfo, nameByImport map[string]string) (string, []string, error) {
	name := info.packageName
	if name == "" {
		name = path.Base(rel)
	}

	var deps []string
	for imp := range info.imports {
		depName, ok := nameByImport[imp]
		if !ok {
			continue
		}
		deps = append(deps, "//vendor/"+imp+":"+depName)
	}
	sort.Strings(deps)

	depsBlock := renderDepsBlock(deps)
	content := fmt.Sprintf(`go_library(
    name = %q,
    srcs = glob(["*.go"], exclude = ["*_test.go"]),
    package_name = %q,
%s    visibility = ["PUBLIC"],
)
`, name, rel, depsBlock)

	buckRel := filepath.Join("vendor", filepath.FromSlash(rel), "BUCK")
	buckPath := filepath.Join(vendorDir, filepath.FromSlash(rel), "BUCK")
	if err := os.WriteFile(buckPath, []byte(content), 0644); err != nil {
		return "", nil, err
	}
	return buckRel, deps, nil
}

// renderDepsBlock formats deps for inclusion in a BUCK go_library rule.
func renderDepsBlock(deps []string) string {
	if len(deps) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("    deps = [\n")
	for _, dep := range deps {
		b.WriteString(fmt.Sprintf("        %q,\n", dep))
	}
	b.WriteString("    ],\n")
	return b.String()
}
