package devicemigrations

import "testing"

func TestListIsContiguousAndStable(t *testing.T) {
	migrations, err := List()
	if err != nil {
		t.Fatal(err)
	}
	if len(migrations) != 7 {
		t.Fatalf("got %d migrations, want 7", len(migrations))
	}
	for index, migration := range migrations {
		if migration.Version != index+1 || migration.Name == "" || migration.SQL == "" {
			t.Fatalf("invalid migration: %+v", migration)
		}
		if migration.Checksum == [32]byte{} {
			t.Fatalf("missing checksum for %s", migration.Name)
		}
	}
}
