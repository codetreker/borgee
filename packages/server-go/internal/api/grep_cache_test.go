package api_test

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
)

type cachedSourceFile struct {
	Path string
	Name string
	Body []byte
}

var cachedSourceDirs sync.Map

func cachedFilesUnder(t *testing.T, dir string) []cachedSourceFile {
	t.Helper()
	abs, err := filepath.Abs(dir)
	if err != nil {
		abs = dir
	}
	if files, ok := cachedSourceDirs.Load(abs); ok {
		return files.([]cachedSourceFile)
	}
	files := make([]cachedSourceFile, 0, 64)
	_ = filepath.WalkDir(abs, func(path string, d os.DirEntry, err error) error {
		if err != nil || d == nil || d.IsDir() {
			return nil
		}
		body, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		files = append(files, cachedSourceFile{Path: path, Name: d.Name(), Body: body})
		return nil
	})
	actual, _ := cachedSourceDirs.LoadOrStore(abs, files)
	return actual.([]cachedSourceFile)
}

func cachedGoFilesUnder(t *testing.T, dir string, includeTests bool) []cachedSourceFile {
	t.Helper()
	all := cachedFilesUnder(t, dir)
	files := make([]cachedSourceFile, 0, len(all))
	for _, f := range all {
		if !strings.HasSuffix(f.Name, ".go") {
			continue
		}
		if !includeTests && strings.HasSuffix(f.Name, "_test.go") {
			continue
		}
		files = append(files, f)
	}
	return files
}

func countRegexpInCachedGoFiles(t *testing.T, dir string, re *regexp.Regexp, includeTests bool) int {
	t.Helper()
	count := 0
	for _, f := range cachedGoFilesUnder(t, dir, includeTests) {
		count += len(re.FindAllIndex(f.Body, -1))
	}
	return count
}

func assertNoRegexpInCachedGoFiles(t *testing.T, dir string, re *regexp.Regexp, includeTests bool, msg string) {
	t.Helper()
	for _, f := range cachedGoFilesUnder(t, dir, includeTests) {
		if loc := re.FindIndex(f.Body); loc != nil {
			t.Errorf(msg, f.Path, f.Body[loc[0]:loc[1]])
		}
	}
}

func assertNoTokensInCachedGoFiles(t *testing.T, dir string, tokens []string, includeTests bool, msg string) {
	t.Helper()
	for _, f := range cachedGoFilesUnder(t, dir, includeTests) {
		for _, tok := range tokens {
			if bytes.Contains(f.Body, []byte(tok)) {
				t.Errorf(msg, tok, f.Path)
			}
		}
	}
}
