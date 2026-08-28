package pathresolve

import "testing"

func TestResolve(t *testing.T) {
	env := Env{Home: "/home/taro", XDGConfigHome: "/home/taro/.config"}

	tests := []struct {
		name    string
		key     string
		want    string
		wantErr bool
	}{
		{name: "home prefix", key: "home:.gitconfig", want: "/home/taro/.gitconfig"},
		{name: "xdg prefix", key: "xdg:nvim", want: "/home/taro/.config/nvim"},
		{name: "unknown prefix errors", key: "unknown:.foo", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Resolve(env, tt.key)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil (result %q)", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEqualPaths(t *testing.T) {
	tests := []struct {
		name string
		a, b string
		want bool
	}{
		{name: "identical", a: "/dotfiles/git/.gitconfig", b: "/dotfiles/git/.gitconfig", want: true},
		{name: "trailing slash", a: "/dotfiles/nvim/", b: "/dotfiles/nvim", want: true},
		{name: "redundant components", a: "/dotfiles/git/../git/.gitconfig", b: "/dotfiles/git/.gitconfig", want: true},
		{name: "double slashes", a: "/dotfiles//git/.gitconfig", b: "/dotfiles/git/.gitconfig", want: true},
		{name: "different paths", a: "/dotfiles/git/.gitconfig", b: "/elsewhere/.gitconfig", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := EqualPaths(tt.a, tt.b); got != tt.want {
				t.Fatalf("EqualPaths(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestExpandHome(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{name: "bare tilde", path: "~", want: "/home/taro"},
		{name: "tilde slash prefix", path: "~/dotfiles", want: "/home/taro/dotfiles"},
		{name: "absolute path unchanged", path: "/etc/foo", want: "/etc/foo"},
		{name: "relative path unchanged", path: "relative/path", want: "relative/path"},
		{name: "tilde without slash unchanged", path: "~foo", want: "~foo"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ExpandHome(tt.path, "/home/taro"); got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFromOS(t *testing.T) {
	tests := []struct {
		name          string
		xdgConfigHome string
		xdgStateHome  string
		want          Env
	}{
		{
			name: "defaults under home when XDG vars are unset",
			want: Env{Home: "/home/taro", XDGConfigHome: "/home/taro/.config", XDGStateHome: "/home/taro/.local/state"},
		},
		{
			name:          "honors XDG overrides",
			xdgConfigHome: "/custom/config",
			xdgStateHome:  "/custom/state",
			want:          Env{Home: "/home/taro", XDGConfigHome: "/custom/config", XDGStateHome: "/custom/state"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("XDG_CONFIG_HOME", tt.xdgConfigHome)
			t.Setenv("XDG_STATE_HOME", tt.xdgStateHome)
			if got := FromOS("/home/taro"); got != tt.want {
				t.Fatalf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}
