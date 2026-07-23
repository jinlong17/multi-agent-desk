// Package server contains the ordered, forward-only Control Plane migrations.
package server

import (
	"crypto/sha256"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"
)

//go:embed *.sql
var files embed.FS

type Migration struct {
	Version  int
	Name     string
	SQL      string
	Checksum [32]byte
}

func List() ([]Migration, error) {
	entries, err := fs.Glob(files, "*.sql")
	if err != nil {
		return nil, fmt.Errorf("list embedded server migrations: %w", err)
	}
	sort.Strings(entries)
	migrations := make([]Migration, 0, len(entries))
	for index, name := range entries {
		prefix, _, ok := strings.Cut(name, "_")
		if !ok || len(prefix) != 4 {
			return nil, fmt.Errorf("invalid server migration name %q", name)
		}
		version, err := strconv.Atoi(prefix)
		if err != nil || version != index+1 {
			return nil, fmt.Errorf("server migrations must be contiguous at %q", name)
		}
		contents, err := files.ReadFile(name)
		if err != nil {
			return nil, fmt.Errorf("read server migration %q: %w", name, err)
		}
		migrations = append(migrations, Migration{Version: version, Name: name, SQL: string(contents), Checksum: sha256.Sum256(contents)})
	}
	if len(migrations) == 0 {
		return nil, fmt.Errorf("no server migrations embedded")
	}
	return migrations, nil
}
