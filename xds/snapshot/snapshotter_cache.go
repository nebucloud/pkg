package snapshot

import (
	"context"

	memdb "github.com/hashicorp/go-memdb"
)

// MemDBProvider is a provider for MemDB.
type MemDBProvider struct {
	db *memdb.MemDB
}

// NewMemDBProvider creates a new MemDBProvider.
func NewMemDBProvider(db *memdb.MemDB) *MemDBProvider {
	return &MemDBProvider{db: db}
}

// GetDatabase returns the MemDB database.
func (p *MemDBProvider) GetDatabase(ctx context.Context) (Database, error) {
	return p.db, nil
}
