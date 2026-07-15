// Package devicemigrations embeds the ordered Device SQLite schema.
package devicemigrations

import (
	"crypto/sha256"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
)

//go:embed *.sql
var migrationFiles embed.FS

type Migration struct {
	Version  int
	Name     string
	SQL      string
	Checksum [sha256.Size]byte
}

// List returns every embedded migration in strictly increasing order.
func List() ([]Migration, error) {
	names, err := fs.Glob(migrationFiles, "*.sql")
	if err != nil {
		return nil, fmt.Errorf("list device migrations: %w", err)
	}
	sort.Strings(names)
	migrations := make([]Migration, 0, len(names))
	lastVersion := 0
	for _, name := range names {
		if len(name) < 5 || name[4] != '_' {
			return nil, fmt.Errorf("invalid device migration filename %q", name)
		}
		version, err := strconv.Atoi(name[:4])
		if err != nil || version != lastVersion+1 {
			return nil, fmt.Errorf("non-contiguous device migration %q after version %d", name, lastVersion)
		}
		contents, err := migrationFiles.ReadFile(name)
		if err != nil {
			return nil, fmt.Errorf("read device migration %q: %w", name, err)
		}
		migrations = append(migrations, Migration{
			Version:  version,
			Name:     name,
			SQL:      string(contents),
			Checksum: sha256.Sum256(contents),
		})
		lastVersion = version
	}
	if len(migrations) == 0 {
		return nil, fmt.Errorf("no device migrations embedded")
	}
	return migrations, nil
}
