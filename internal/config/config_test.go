package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
}

func TestLoadFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ten.local.toml")
	writeFile(t, path, `
[vars]
git_email = "taro@example.com"

[tools.git]
links = { "home:.gitconfig" = "git/.gitconfig.local" }
`)

	got, ok, err := LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	if !ok {
		t.Fatalf("expected ok=true for an existing file")
	}
	if got.Vars["git_email"] != "taro@example.com" {
		t.Fatalf("unexpected vars: %+v", got.Vars)
	}
	want := Tool{Links: map[string]string{"home:.gitconfig": "git/.gitconfig.local"}}
	if !reflect.DeepEqual(got.Tools["git"], want) {
		t.Fatalf("unexpected tools[git]: %+v", got.Tools["git"])
	}
}

func TestLoadFile_MissingFileIsOptional(t *testing.T) {
	dir := t.TempDir()
	file, ok, err := LoadFile(filepath.Join(dir, "does-not-exist.toml"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatalf("expected ok=false for missing file")
	}
	if len(file.Tools) != 0 {
		t.Fatalf("expected empty File, got %+v", file)
	}
}

func TestMerge_LocalOverridesRepoWholeTool(t *testing.T) {
	repo := File{Tools: map[string]Tool{
		"git":  {Links: map[string]string{"home:.gitconfig": "git/.gitconfig"}, PostApply: "echo repo"},
		"nvim": {Links: map[string]string{"xdg:nvim": "nvim"}},
	}}
	local := &File{
		Vars: map[string]string{"k": "v"},
		Tools: map[string]Tool{
			"git": {Links: map[string]string{"home:.gitconfig": "git/.gitconfig.local"}},
		},
	}

	got, err := Merge(repo, nil, local)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantGit := Tool{Links: map[string]string{"home:.gitconfig": "git/.gitconfig.local"}}
	if !reflect.DeepEqual(got.Tools["git"], wantGit) {
		t.Fatalf("expected whole-tool replace, got %+v", got.Tools["git"])
	}
	if _, ok := got.Tools["nvim"]; !ok {
		t.Fatalf("expected nvim to survive merge untouched")
	}
}

func TestMerge_OverrideChainBaseProfileLocal(t *testing.T) {
	base := File{Tools: map[string]Tool{
		"git": {Links: map[string]string{"home:.gitconfig": "git/.gitconfig"}},
	}}
	profile := &File{Tools: map[string]Tool{
		"git": {Links: map[string]string{"home:.gitconfig": "git/.gitconfig.work"}},
	}}
	local := &File{Tools: map[string]Tool{
		"git": {Links: map[string]string{"home:.gitconfig": "git/.gitconfig.local"}},
	}}

	got, err := Merge(base, profile, local)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := Tool{Links: map[string]string{"home:.gitconfig": "git/.gitconfig.local"}}
	if !reflect.DeepEqual(got.Tools["git"], want) {
		t.Fatalf("expected local to win override chain, got %+v", got.Tools["git"])
	}
}

func TestMerge_VarsOverrideChainBaseProfileLocal(t *testing.T) {
	base := File{Vars: map[string]string{"git_email": "base@example.com", "shared": "base"}}
	profile := &File{Vars: map[string]string{"git_email": "profile@example.com"}}
	local := &File{Vars: map[string]string{"git_email": "local@example.com"}}

	got, err := Merge(base, profile, local)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Vars["git_email"] != "local@example.com" {
		t.Fatalf("expected local to win vars override chain, got %q", got.Vars["git_email"])
	}
	if got.Vars["shared"] != "base" {
		t.Fatalf("expected base-only var to survive, got %q", got.Vars["shared"])
	}
}

func TestMerge_NilProfileAndLocalUseBaseOnly(t *testing.T) {
	base := File{Tools: map[string]Tool{"git": {}}}
	got, err := Merge(base, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := got.Tools["git"]; !ok {
		t.Fatalf("expected base tool to survive with nil profile and local, got %+v", got.Tools)
	}
	if !got.Enabled["git"] {
		t.Fatalf("expected git enabled by fallback, got %+v", got.Enabled)
	}
}

func TestMerge_EnabledToolsUnionAcrossLayers(t *testing.T) {
	base := File{
		EnabledTools: []string{"git"},
		Tools:        map[string]Tool{"git": {}, "git-work": {}, "local-tool": {}},
	}
	profile := &File{EnabledTools: []string{"git-work"}}
	local := &File{EnabledTools: []string{"local-tool"}, Tools: map[string]Tool{"local-tool": {}}}

	got, err := Merge(base, profile, local)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, name := range []string{"git", "git-work", "local-tool"} {
		if !got.Enabled[name] {
			t.Fatalf("expected %q enabled, got %+v", name, got.Enabled)
		}
	}
}

func TestMerge_EnabledToolsErrorsOnUndefinedTool(t *testing.T) {
	base := File{EnabledTools: []string{"ghost"}, Tools: map[string]Tool{}}
	if _, err := Merge(base, nil, nil); err == nil {
		t.Fatalf("expected error for enabled_tools referencing undefined tool")
	}
}
