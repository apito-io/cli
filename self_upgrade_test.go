package main

import "testing"

func TestPrivilegedReplaceDoesNotCopyOverRunningBinary(t *testing.T) {
	const dest = "/usr/local/bin/apito"
	cmds := privilegedReplaceCommands("/tmp/apito-new", dest)
	if len(cmds) != 3 || cmds[0][0] != "cp" || cmds[0][2] != dest+".new" {
		t.Fatalf("stage = %v", cmds)
	}
	if cmds[1][0] != "chmod" || cmds[1][1] != "755" || cmds[1][2] != dest+".new" {
		t.Fatalf("chmod = %v", cmds[1])
	}
	if cmds[2][0] != "mv" || cmds[2][1] != dest+".new" || cmds[2][2] != dest {
		t.Fatalf("replace = %v", cmds[2])
	}
	for _, cmd := range cmds {
		if cmd[0] == "cp" && cmd[len(cmd)-1] == dest {
			t.Fatal("cp writes the running binary")
		}
	}
}

func TestPrivilegedBackupCopiesToANewFile(t *testing.T) {
	cmds := privilegedBackupCommands("/usr/local/bin/apito", "/usr/local/bin/apito.backup")
	if len(cmds) != 1 || cmds[0][0] != "cp" || cmds[0][2] != "/usr/local/bin/apito.backup" {
		t.Fatalf("backup = %v", cmds)
	}
	if cmds[0][2] == "/usr/local/bin/apito" {
		t.Fatal("backup overwrites the running binary")
	}
}
