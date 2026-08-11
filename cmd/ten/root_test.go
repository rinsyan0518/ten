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
