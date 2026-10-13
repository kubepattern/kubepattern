package linter

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// repoPatternRoots are the directories and files (relative to this package) whose YAML holds the
// Patterns shipped with the repository or evaluated in the replication package (copied in
// internal/kube/testdata/replication).
var repoPatternRoots = []string{
	"../../charts/kubepattern/templates/example.yaml",
	"../kube/testdata/replication/patterns",
	"../kube/testdata/replication/patterns-v2",
	"../kube/testdata/replication/krateo/patterns",
	"../kube/testdata/replication/krateo/patterns-v2",
	"../kube/testdata/replication/krateo/fixtures",
}

// helmDirective matches a line made only of a Helm template directive ({{- if ... }}).
var helmDirective = regexp.MustCompile(`(?m)^\s*\{\{-?.*-?\}\}\s*$`)

// Every Pattern of the repository lints with the current linter: a stricter rule must not
// silently skip a shipped or evaluated Pattern at run time.
func TestRepositoryPatternsLint(t *testing.T) {
	count := 0
	for _, root := range repoPatternRoots {
		err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || (!strings.HasSuffix(path, ".yaml") && !strings.HasSuffix(path, ".yml")) {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			data = helmDirective.ReplaceAll(data, nil)
			dec := yaml.NewDecoder(bytes.NewReader(data))
			for i := 0; ; i++ {
				var doc map[string]any
				err := dec.Decode(&doc)
				if errors.Is(err, io.EOF) {
					break
				}
				if err != nil {
					t.Errorf("%s: document %d is not valid YAML: %v", path, i, err)
					break
				}
				if doc["kind"] != "Pattern" && doc["kind"] != "PatternAsCode" {
					continue
				}
				out, err := yaml.Marshal(doc)
				if err != nil {
					t.Fatalf("%s: %v", path, err)
				}
				count++
				if _, err := Lint(out); err != nil {
					t.Errorf("%s: document %d does not lint: %v", path, i, err)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walking %s: %v", root, err)
		}
	}
	// 1 chart example + 17 evaluation + 8 v2 + 12 Krateo + 10 Krateo v2 + fixtures.
	if count < 48 {
		t.Errorf("linted %d Patterns, expected at least 48: a root moved?", count)
	}
	t.Logf("linted %d Patterns", count)
}
