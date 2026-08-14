package main

import (
	"bytes"
	"testing"
)

func TestRootCmd_Help(t *testing.T) {
	cmd := newRootCmd()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute --help: %v", err)
	}
	if !bytes.Contains(buf.Bytes(), []byte("ten is an idempotent dotfiles manager")) {
		t.Fatalf("expected help output to contain short description, got: %s", buf.String())
	}
}

func TestRootCmd_Version(t *testing.T) {
	cmd := newRootCmd()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--version"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute --version: %v", err)
	}
	if !bytes.Contains(buf.Bytes(), []byte("ten version")) {
		t.Fatalf("expected version output to start with %q, got: %s", "ten version", buf.String())
	}
}
