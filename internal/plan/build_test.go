package plan_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/rinsyan0518/ten/internal/config"
	"github.com/rinsyan0518/ten/internal/pathresolve"
	"github.com/rinsyan0518/ten/internal/plan"
	"github.com/rinsyan0518/ten/internal/render"
	"github.com/rinsyan0518/ten/internal/state"
)

// fakeInspector is a read-only filesystem double: entries maps a path to
// what Lstat would see, files maps a path to its regular-file content.
// Paths absent from entries simply don't exist.
type fakeInspector struct {
	entries map[string]plan.Entry
	files   map[string][]byte
}

func (f *fakeInspector) Inspect(path string) (plan.Entry, error) {
	return f.entries[path], nil
}

func (f *fakeInspector) ReadFile(path string) ([]byte, error) {
	content, ok := f.files[path]
	if !ok {
		return nil, fmt.Errorf("fakeInspector: no file at %s", path)
	}
	return content, nil
}

func testBuildEnv() pathresolve.Env {
	return pathresolve.Env{Home: "/home/taro", XDGConfigHome: "/home/taro/.config"}
}

// sources marks the given paths as existing regular files, the minimum
// for a link source to be considered valid.
func sources(paths ...string) map[string]plan.Entry {
	entries := map[string]plan.Entry{}
	for _, p := range paths {
		entries[p] = plan.Entry{Exists: true}
	}
	return entries
}

func TestBuild_OrdersToolsByDependencyWithHooks(t *testing.T) {
	merged := config.Merged{
		DotfilesRoot: "/dotfiles",
		Tools: map[string]config.Tool{
			"git": {Links: map[string]string{"home:.gitconfig": "git/.gitconfig"}, After: "echo git-done"},
			"git-work": {
				DependsOn: []string{"git"},
				Before:    "echo work-start",
				Templates: map[string]string{"home:.gitconfig.local": "git/gitconfig.local.tmpl"},
			},
		},
		Enabled: map[string]bool{"git": true, "git-work": true},
	}
	fx := &fakeInspector{
		entries: sources("/dotfiles/git/.gitconfig"),
		files:   map[string][]byte{"/dotfiles/git/gitconfig.local.tmpl": []byte("hello")},
	}

	p, err := plan.Build(plan.BuildParams{Merged: merged, Current: state.State{ManagedResources: map[string]state.Resource{}}, Env: testBuildEnv(), Inspector: fx})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	if len(p.Tools) != 2 || p.Tools[0].Tool != "git" || p.Tools[1].Tool != "git-work" {
		t.Fatalf("expected git before git-work, got %+v", p.Tools)
	}
	if p.Tools[0].After != "echo git-done" || p.Tools[1].Before != "echo work-start" {
		t.Fatalf("hooks not carried into the plan: %+v", p.Tools)
	}
	if len(p.Tools[0].Links) != 1 || p.Tools[0].Links[0].Action != plan.ActionCreate {
		t.Fatalf("expected a create link step for git, got %+v", p.Tools[0].Links)
	}
	if len(p.Tools[1].Templates) != 1 || p.Tools[1].Templates[0].Action != plan.ActionCreate {
		t.Fatalf("expected a create template step for git-work, got %+v", p.Tools[1].Templates)
	}
}

func TestBuild_LinkActions(t *testing.T) {
	target := "/home/taro/.gitconfig"
	source := "/dotfiles/git/.gitconfig"
	tests := []struct {
		name  string
		entry plan.Entry
		want  plan.Action
	}{
		{name: "missing target is created", entry: plan.Entry{}, want: plan.ActionCreate},
		{name: "correct symlink is a noop", entry: plan.Entry{Exists: true, IsSymlink: true, LinkDest: source}, want: plan.ActionNoop},
		{name: "symlink elsewhere is replaced", entry: plan.Entry{Exists: true, IsSymlink: true, LinkDest: "/elsewhere"}, want: plan.ActionReplace},
		{name: "regular file is replaced", entry: plan.Entry{Exists: true}, want: plan.ActionReplace},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			merged := config.Merged{
				DotfilesRoot: "/dotfiles",
				Tools:        map[string]config.Tool{"git": {Links: map[string]string{"home:.gitconfig": "git/.gitconfig"}}},
				Enabled:      map[string]bool{"git": true},
			}
			entries := sources(source)
			entries[target] = tt.entry
			fx := &fakeInspector{entries: entries}

			p, err := plan.Build(plan.BuildParams{Merged: merged, Current: state.State{ManagedResources: map[string]state.Resource{}}, Env: testBuildEnv(), Inspector: fx})
			if err != nil {
				t.Fatalf("Build: %v", err)
			}
			if len(p.Tools) != 1 || len(p.Tools[0].Links) != 1 {
				t.Fatalf("expected one link step, got %+v", p.Tools)
			}
			if got := p.Tools[0].Links[0].Action; got != tt.want {
				t.Fatalf("action = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuild_ErrorsWhenLinkSourceMissing(t *testing.T) {
	merged := config.Merged{
		DotfilesRoot: "/dotfiles",
		Tools:        map[string]config.Tool{"git": {Links: map[string]string{"home:.gitconfig": "git/.gitconfig"}}},
		Enabled:      map[string]bool{"git": true},
	}
	fx := &fakeInspector{entries: map[string]plan.Entry{}}

	_, err := plan.Build(plan.BuildParams{Merged: merged, Current: state.State{ManagedResources: map[string]state.Resource{}}, Env: testBuildEnv(), Inspector: fx})
	if err == nil || !strings.Contains(err.Error(), "/dotfiles/git/.gitconfig") {
		t.Fatalf("expected error naming the missing source, got %v", err)
	}
}

func TestBuild_TemplateActions(t *testing.T) {
	target := "/home/taro/.gitconfig.local"
	rendered := "email=taro@example.com"
	tests := []struct {
		name    string
		entry   plan.Entry
		content []byte
		managed bool // previously managed as a template per state
		want    plan.Action
	}{
		{name: "missing target is created", entry: plan.Entry{}, want: plan.ActionCreate},
		{name: "unmanaged existing file is replaced with backup", entry: plan.Entry{Exists: true}, content: []byte("user's own file"), want: plan.ActionReplace},
		{name: "managed file with same content is a noop", entry: plan.Entry{Exists: true}, content: []byte(rendered), managed: true, want: plan.ActionNoop},
		{name: "managed file with stale content is updated", entry: plan.Entry{Exists: true}, content: []byte("old render"), managed: true, want: plan.ActionUpdate},
		{name: "managed target that is now a symlink is updated without content compare", entry: plan.Entry{Exists: true, IsSymlink: true, LinkDest: "/dotfiles/git/x"}, managed: true, want: plan.ActionUpdate},
		{name: "unmanaged symlink target is replaced with backup", entry: plan.Entry{Exists: true, IsSymlink: true, LinkDest: "/dotfiles/git/x"}, want: plan.ActionReplace},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			merged := config.Merged{
				DotfilesRoot: "/dotfiles",
				Vars:         map[string]string{"git_email": "taro@example.com"},
				Tools:        map[string]config.Tool{"git": {Templates: map[string]string{"home:.gitconfig.local": "git/tmpl"}}},
				Enabled:      map[string]bool{"git": true},
			}
			entries := map[string]plan.Entry{target: tt.entry}
			files := map[string][]byte{"/dotfiles/git/tmpl": []byte("email={{ .Vars.git_email }}")}
			if tt.content != nil {
				files[target] = tt.content
			}
			current := state.State{ManagedResources: map[string]state.Resource{}}
			if tt.managed {
				current.ManagedResources[target] = state.Resource{Tool: "git", Type: "template", Source: "/dotfiles/git/tmpl"}
			}
			fx := &fakeInspector{entries: entries, files: files}

			p, err := plan.Build(plan.BuildParams{Merged: merged, Current: current, Env: testBuildEnv(), Inspector: fx})
			if err != nil {
				t.Fatalf("Build: %v", err)
			}
			if len(p.Tools) != 1 || len(p.Tools[0].Templates) != 1 {
				t.Fatalf("expected one template step, got %+v", p.Tools)
			}
			step := p.Tools[0].Templates[0]
			if step.Action != tt.want {
				t.Fatalf("action = %q, want %q", step.Action, tt.want)
			}
			if step.Action != plan.ActionNoop && string(step.Content) != rendered {
				t.Fatalf("step should carry the rendered content, got %q", step.Content)
			}
		})
	}
}

func TestBuild_SetsTenToolPerToolWhenRendering(t *testing.T) {
	merged := config.Merged{
		DotfilesRoot: "/dotfiles",
		Tools: map[string]config.Tool{
			"git":  {Templates: map[string]string{"home:.a": "a.tmpl"}},
			"nvim": {Templates: map[string]string{"home:.b": "b.tmpl"}},
		},
		Enabled: map[string]bool{"git": true, "nvim": true},
	}
	fx := &fakeInspector{
		entries: map[string]plan.Entry{},
		files: map[string][]byte{
			"/dotfiles/a.tmpl": []byte("tool={{ .Ten.Tool }}"),
			"/dotfiles/b.tmpl": []byte("tool={{ .Ten.Tool }}"),
		},
	}

	p, err := plan.Build(plan.BuildParams{Merged: merged, Current: state.State{ManagedResources: map[string]state.Resource{}}, Env: testBuildEnv(), Ten: render.SystemInfo{Hostname: "h"}, Inspector: fx})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	got := map[string]string{}
	for _, tp := range p.Tools {
		got[tp.Tool] = string(tp.Templates[0].Content)
	}
	if got["git"] != "tool=git" || got["nvim"] != "tool=nvim" {
		t.Fatalf("Ten.Tool not set per tool: %+v", got)
	}
}

func TestBuild_OnceEligibility(t *testing.T) {
	tracked := state.State{ManagedResources: map[string]state.Resource{
		"/home/taro/.gitconfig": {Tool: "git", Type: "symlink", Source: "/dotfiles/git/.gitconfig"},
	}}
	tests := []struct {
		name    string
		tool    config.Tool
		current state.State
		want    string
	}{
		{
			name:    "untracked target arms once",
			tool:    config.Tool{Links: map[string]string{"home:.gitconfig": "git/.gitconfig"}, Once: "echo first"},
			current: state.State{ManagedResources: map[string]state.Resource{}},
			want:    "echo first",
		},
		{
			name:    "already tracked target does not arm once",
			tool:    config.Tool{Links: map[string]string{"home:.gitconfig": "git/.gitconfig"}, Once: "echo first"},
			current: tracked,
			want:    "",
		},
		{
			name:    "hooks-only tool never arms once",
			tool:    config.Tool{Before: "echo b", Once: "echo first", After: "echo a"},
			current: state.State{ManagedResources: map[string]state.Resource{}},
			want:    "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			merged := config.Merged{
				DotfilesRoot: "/dotfiles",
				Tools:        map[string]config.Tool{"git": tt.tool},
				Enabled:      map[string]bool{"git": true},
			}
			fx := &fakeInspector{entries: sources("/dotfiles/git/.gitconfig")}

			p, err := plan.Build(plan.BuildParams{Merged: merged, Current: tt.current, Env: testBuildEnv(), Inspector: fx})
			if err != nil {
				t.Fatalf("Build: %v", err)
			}
			if len(p.Tools) != 1 {
				t.Fatalf("expected one tool plan, got %+v", p.Tools)
			}
			if got := p.Tools[0].Once; got != tt.want {
				t.Fatalf("Once = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuild_PruneActions(t *testing.T) {
	target := "/home/taro/.config/old"
	source := "/dotfiles/old/x"
	tests := []struct {
		name       string
		res        state.Resource
		entries    map[string]plan.Entry
		want       plan.Action
		wantReason string
		wantErr    string
	}{
		{
			name:    "owned symlink with no backup is removed",
			res:     state.Resource{Tool: "old", Type: "symlink", Source: source},
			entries: map[string]plan.Entry{target: {Exists: true, IsSymlink: true, LinkDest: source}},
			want:    plan.ActionRemove,
		},
		{
			name: "owned symlink with backup is restored",
			res:  state.Resource{Tool: "old", Type: "symlink", Source: source, BackupPath: "/home/taro/.ten_backup/x"},
			entries: map[string]plan.Entry{
				target:                       {Exists: true, IsSymlink: true, LinkDest: source},
				"/home/taro/.ten_backup/x":   {Exists: true},
			},
			want: plan.ActionRestore,
		},
		{
			name:       "symlink replaced by a real file is skipped",
			res:        state.Resource{Tool: "old", Type: "symlink", Source: source},
			entries:    map[string]plan.Entry{target: {Exists: true}},
			want:       plan.ActionSkip,
			wantReason: "no longer a symlink",
		},
		{
			name:       "symlink now pointing elsewhere is skipped",
			res:        state.Resource{Tool: "old", Type: "symlink", Source: source},
			entries:    map[string]plan.Entry{target: {Exists: true, IsSymlink: true, LinkDest: "/elsewhere"}},
			want:       plan.ActionSkip,
			wantReason: "points at",
		},
		{
			name:    "missing target with no backup still removes the record",
			res:     state.Resource{Tool: "old", Type: "symlink", Source: source},
			entries: map[string]plan.Entry{},
			want:    plan.ActionRemove,
		},
		{
			name:    "recorded backup that no longer exists is an error",
			res:     state.Resource{Tool: "old", Type: "symlink", Source: source, BackupPath: "/home/taro/.ten_backup/gone"},
			entries: map[string]plan.Entry{target: {Exists: true, IsSymlink: true, LinkDest: source}},
			wantErr: "/home/taro/.ten_backup/gone",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			merged := config.Merged{
				DotfilesRoot: "/dotfiles",
				Tools:        map[string]config.Tool{},
				Enabled:      map[string]bool{},
			}
			current := state.State{ManagedResources: map[string]state.Resource{target: tt.res}}
			fx := &fakeInspector{entries: tt.entries}

			p, err := plan.Build(plan.BuildParams{Merged: merged, Current: current, Env: testBuildEnv(), Inspector: fx})
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error mentioning %q, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Build: %v", err)
			}
			if len(p.Prunes) != 1 {
				t.Fatalf("expected one prune step, got %+v", p.Prunes)
			}
			step := p.Prunes[0]
			if step.Action != tt.want {
				t.Fatalf("action = %q, want %q", step.Action, tt.want)
			}
			if tt.wantReason != "" && !strings.Contains(step.SkipReason, tt.wantReason) {
				t.Fatalf("SkipReason = %q, want it to contain %q", step.SkipReason, tt.wantReason)
			}
		})
	}
}

func TestBuild_OmitsToolsWithNothingToDo(t *testing.T) {
	merged := config.Merged{
		DotfilesRoot: "/dotfiles",
		Tools:        map[string]config.Tool{"empty": {}},
		Enabled:      map[string]bool{"empty": true},
	}
	fx := &fakeInspector{entries: map[string]plan.Entry{}}

	p, err := plan.Build(plan.BuildParams{Merged: merged, Current: state.State{ManagedResources: map[string]state.Resource{}}, Env: testBuildEnv(), Inspector: fx})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if len(p.Tools) != 0 {
		t.Fatalf("expected a tool with no hooks and no resources to be omitted, got %+v", p.Tools)
	}
}
