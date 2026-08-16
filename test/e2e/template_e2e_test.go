package e2e_test

import (
	"strings"
	"testing"

	"github.com/rinsyan0518/ten/internal/testutil/dockertest"
)

func TestApply_RendersTemplateWithVars(t *testing.T) {
	sb := dockertest.NewSandbox(t)
	home := sb.Home()

	sb.Init(t, home, home+"/dotfiles")
	sb.WriteFile(t, home+"/dotfiles/ten.local.toml", `
[vars]
git_email = "taro@work.example.com"
`)
	sb.WriteFile(t, home+"/dotfiles/ten.toml", `
[tools.git-work]
templates = { "home:.gitconfig.local" = "git/gitconfig.local.tmpl" }
`)
	sb.WriteFile(t, home+"/dotfiles/git/gitconfig.local.tmpl", "[user]\n\temail = {{ .Vars.git_email }}\n")

	out, code := sb.Run(t, home, "apply")
	if code != 0 {
		t.Fatalf("ten apply failed (exit %d): %s", code, out)
	}

	got := sb.ReadFile(t, home+"/.gitconfig.local")
	want := "[user]\n\temail = taro@work.example.com\n"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	if !strings.Contains(out, "render template") {
		t.Fatalf("expected output to mention template rendering, got: %s", out)
	}
}

func TestApply_TemplateBacksUpExistingFile(t *testing.T) {
	sb := dockertest.NewSandbox(t)
	home := sb.Home()

	sb.Init(t, home, home+"/dotfiles")
	sb.WriteFile(t, home+"/dotfiles/ten.local.toml", `
[vars]
git_email = "taro@work.example.com"
`)
	sb.WriteFile(t, home+"/dotfiles/ten.toml", `
[tools.git-work]
templates = { "home:.gitconfig.local" = "git/gitconfig.local.tmpl" }
`)
	sb.WriteFile(t, home+"/dotfiles/git/gitconfig.local.tmpl", "email = {{ .Vars.git_email }}\n")
	sb.WriteFile(t, home+"/.gitconfig.local", "old content\n")

	out, code := sb.Run(t, home, "apply")
	if code != 0 {
		t.Fatalf("ten apply failed (exit %d): %s", code, out)
	}
	findOut, _, code := sb.Exec(t, "find "+home+"/.ten_backup -name .gitconfig.local")
	if code != 0 || strings.TrimSpace(findOut) == "" {
		t.Fatalf("expected a backup of the old .gitconfig.local, find output: %q", findOut)
	}
}

func TestApply_TemplateSecondRunDoesNotReBackup(t *testing.T) {
	sb := dockertest.NewSandbox(t)
	home := sb.Home()

	sb.Init(t, home, home+"/dotfiles")
	sb.WriteFile(t, home+"/dotfiles/ten.local.toml", `
[vars]
git_email = "taro@work.example.com"
`)
	sb.WriteFile(t, home+"/dotfiles/ten.toml", `
[tools.git-work]
templates = { "home:.gitconfig.local" = "git/gitconfig.local.tmpl" }
`)
	sb.WriteFile(t, home+"/dotfiles/git/gitconfig.local.tmpl", "email = {{ .Vars.git_email }}\n")

	if _, code := sb.Run(t, home, "apply"); code != 0 {
		t.Fatalf("first apply failed")
	}
	if _, code := sb.Run(t, home, "apply"); code != 0 {
		t.Fatalf("second apply failed")
	}

	findOut, _, _ := sb.Exec(t, "find "+home+"/.ten_backup -type f 2>/dev/null")
	if strings.TrimSpace(findOut) != "" {
		t.Fatalf("expected no backup from re-rendering a ten-managed template, found: %q", findOut)
	}
	got := sb.ReadFile(t, home+"/.gitconfig.local")
	if got != "email = taro@work.example.com\n" {
		t.Fatalf("unexpected content after second apply: %q", got)
	}
}
