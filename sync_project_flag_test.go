package main

import "testing"

func TestChooseSyncProjectUsesFlagAndSkipsPrompt(t *testing.T) {
	projects := []SyncProject{{ID: "retest_pm_2bc7", Name: "Retest"}, {ID: "other", Name: "Other"}}

	chosen, ok, err := chooseSyncProject(projects, "retest_pm_2bc7", "FROM")
	if err != nil || !ok || chosen.ID != "retest_pm_2bc7" {
		t.Fatalf("flagged project = %+v ok=%v err=%v", chosen, ok, err)
	}

	_, ok, err = chooseSyncProject(projects, "missing", "FROM")
	if err == nil || ok {
		t.Fatalf("missing project ok=%v err=%v", ok, err)
	}

	_, ok, err = chooseSyncProject(projects, "", "FROM")
	if err != nil || ok {
		t.Fatalf("empty flag should leave the picker in charge: ok=%v err=%v", ok, err)
	}
}
