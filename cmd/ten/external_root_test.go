package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func runTenCmd(t *testing.T, args ...string) (stdout string, err error) {
	t.Helper()
	cmd := newRootCmd()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetArgs(args)
	err = cmd.Execute()
	return buf.String(), err
}

func TestRootAdd_RegistersNewRoot(t *testing.T) {
	home := newInitTestHome(t)
	dotfilesRoot := t.TempDir()
	workRoot := t.TempDir()

	if _, err := runTenCmd(t, "init", "--path", dotfilesRoot); err != nil {
		t.Fatalf("execute init: %v", err)
	}
	if _, err := runTenCmd(t, "root", "add", "work", workRoot); err != nil {
		t.Fatalf("execute root add: %v", err)
	}

	got := loadState(t, home).ExternalRoots
	if len(got) != 1 || got[0].Name != "work" {
		t.Fatalf("expected one registered root named work, got %+v", got)
	}
	gotPath, err := filepath.EvalSymlinks(got[0].Path)
	if err != nil {
		t.Fatalf("resolve got path: %v", err)
	}
	wantPath, err := filepath.EvalSymlinks(workRoot)
	if err != nil {
		t.Fatalf("resolve want path: %v", err)
	}
	if gotPath != wantPath {
		t.Fatalf("Path = %q, want %q", gotPath, wantPath)
	}
}

func TestRootAdd_UpdatesPathInPlaceWithoutMovingPosition(t *testing.T) {
	home := newInitTestHome(t)
	dotfilesRoot := t.TempDir()
	firstPath := t.TempDir()
	secondPath := t.TempDir()
	otherRoot := t.TempDir()

	if _, err := runTenCmd(t, "init", "--path", dotfilesRoot); err != nil {
		t.Fatalf("execute init: %v", err)
	}
	if _, err := runTenCmd(t, "root", "add", "work", firstPath); err != nil {
		t.Fatalf("execute root add work: %v", err)
	}
	if _, err := runTenCmd(t, "root", "add", "client2", otherRoot); err != nil {
		t.Fatalf("execute root add client2: %v", err)
	}
	if _, err := runTenCmd(t, "root", "add", "work", secondPath); err != nil {
		t.Fatalf("execute re-add work: %v", err)
	}

	got := loadState(t, home).ExternalRoots
	if len(got) != 2 {
		t.Fatalf("expected re-adding \"work\" to update in place, not append, got %+v", got)
	}
	if got[0].Name != "work" || got[1].Name != "client2" {
		t.Fatalf("expected registration order [work, client2] to be preserved, got %+v", got)
	}
	gotPath, err := filepath.EvalSymlinks(got[0].Path)
	if err != nil {
		t.Fatalf("resolve got path: %v", err)
	}
	wantPath, err := filepath.EvalSymlinks(secondPath)
	if err != nil {
		t.Fatalf("resolve want path: %v", err)
	}
	if gotPath != wantPath {
		t.Fatalf("expected work's path to be updated to %q, got %q", wantPath, gotPath)
	}
}

func TestRootAdd_ErrorsWhenPathDoesNotExist(t *testing.T) {
	home := newInitTestHome(t)
	dotfilesRoot := t.TempDir()

	if _, err := runTenCmd(t, "init", "--path", dotfilesRoot); err != nil {
		t.Fatalf("execute init: %v", err)
	}
	if _, err := runTenCmd(t, "root", "add", "work", filepath.Join(home, "does-not-exist")); err == nil {
		t.Fatalf("expected root add to fail for a nonexistent path")
	}
}

func TestRootAdd_ErrorsOnInvalidName(t *testing.T) {
	dotfilesRoot := t.TempDir()
	workRoot := t.TempDir()
	newInitTestHome(t)

	if _, err := runTenCmd(t, "init", "--path", dotfilesRoot); err != nil {
		t.Fatalf("execute init: %v", err)
	}
	if _, err := runTenCmd(t, "root", "add", "work root!", workRoot); err == nil {
		t.Fatalf("expected root add to reject a name containing spaces/punctuation")
	}
}

func TestRootAdd_ErrorsWhenDotfilesRootNotInitialized(t *testing.T) {
	newInitTestHome(t)
	workRoot := t.TempDir()

	if _, err := runTenCmd(t, "root", "add", "work", workRoot); err == nil {
		t.Fatalf("expected root add to fail before `ten init` has been run")
	}
}

func TestRootRemove_ErrorsWhenNameNotRegistered(t *testing.T) {
	dotfilesRoot := t.TempDir()
	newInitTestHome(t)

	if _, err := runTenCmd(t, "init", "--path", dotfilesRoot); err != nil {
		t.Fatalf("execute init: %v", err)
	}
	if _, err := runTenCmd(t, "root", "remove", "ghost"); err == nil {
		t.Fatalf("expected root remove to fail for an unregistered name")
	}
}

func TestRootRemove_RemovesRegisteredRoot(t *testing.T) {
	home := newInitTestHome(t)
	dotfilesRoot := t.TempDir()
	workRoot := t.TempDir()

	if _, err := runTenCmd(t, "init", "--path", dotfilesRoot); err != nil {
		t.Fatalf("execute init: %v", err)
	}
	if _, err := runTenCmd(t, "root", "add", "work", workRoot); err != nil {
		t.Fatalf("execute root add: %v", err)
	}
	if _, err := runTenCmd(t, "root", "remove", "work"); err != nil {
		t.Fatalf("execute root remove: %v", err)
	}

	got := loadState(t, home).ExternalRoots
	if len(got) != 0 {
		t.Fatalf("expected no external roots after remove, got %+v", got)
	}
}

func TestRootList_PrintsRegisteredRootsInOrder(t *testing.T) {
	dotfilesRoot := t.TempDir()
	workRoot := t.TempDir()
	client2Root := t.TempDir()
	newInitTestHome(t)

	if _, err := runTenCmd(t, "init", "--path", dotfilesRoot); err != nil {
		t.Fatalf("execute init: %v", err)
	}
	if _, err := runTenCmd(t, "root", "add", "work", workRoot); err != nil {
		t.Fatalf("execute root add work: %v", err)
	}
	if _, err := runTenCmd(t, "root", "add", "client2", client2Root); err != nil {
		t.Fatalf("execute root add client2: %v", err)
	}

	out, err := runTenCmd(t, "root", "list")
	if err != nil {
		t.Fatalf("execute root list: %v", err)
	}
	workIdx := strings.Index(out, "work")
	client2Idx := strings.Index(out, "client2")
	if workIdx == -1 || client2Idx == -1 || workIdx > client2Idx {
		t.Fatalf("expected list to print work before client2 in registration order, got: %s", out)
	}
}
