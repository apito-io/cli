package main

import (
	"fmt"
	"testing"
)

func TestIsDraftStagedDespiteSyncError(t *testing.T) {
	err := fmt.Errorf(`graphql errors: stage schema mutation: draft staged locally but system sync failed: replica sync: open ltx file /data/apito-db/sqlite/system/.system.db-litestream/ltx/0/0000000000000004-0000000000000004.ltx: no such file or directory`)
	if !isDraftStagedDespiteSyncError(err) {
		t.Fatal("expected litestream replica error to count as staged draft success")
	}
}

func TestIsDraftStagedDespiteSyncError_RejectsRealFailures(t *testing.T) {
	err := fmt.Errorf("graphql errors: permission denied")
	if isDraftStagedDespiteSyncError(err) {
		t.Fatal("expected unrelated graphql error to remain a failure")
	}
}
