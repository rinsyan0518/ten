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

func TestResolveKey(t *testing.T) {
	tests := []struct {
		name          string
		key           string
		xdgConfigHome string
		want          string
		wantErr       bool
	}{
		{name: "home prefix", key: "home:.gitconfig", want: "/home/taro/.gitconfig"},
		{name: "xdg prefix defaults under home/.config", key: "xdg:nvim", want: "/home/taro/.config/nvim"},
		{name: "xdg prefix honors XDG_CONFIG_HOME override", key: "xdg:nvim", xdgConfigHome: "/custom/config", want: "/custom/config/nvim"},
		{name: "unknown prefix errors", key: "unknown:.foo", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("XDG_CONFIG_HOME", tt.xdgConfigHome)
			got, err := ResolveKey("/home/taro", tt.key)
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
