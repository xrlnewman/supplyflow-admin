package main

import (
	"database/sql"
	"testing"
)

func TestSelectSupplyStoreUsesSQLWhenConfigured(t *testing.T) {
	if _, ok := selectSupplyStore(&SQLStore{db: &sql.DB{}}).(*SupplySQLStore); !ok {
		t.Fatal("SQLStore must select SupplySQLStore")
	}
}

func TestSelectSupplyStoreKeepsMemoryForDemo(t *testing.T) {
	if _, ok := selectSupplyStore(NewMemoryStore()).(*SupplyMemoryStore); !ok {
		t.Fatal("memory CareStore must select SupplyMemoryStore")
	}
}
