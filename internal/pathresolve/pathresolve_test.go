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
		{name: "custom absolute", key: "custom:/etc/foo", want: "/etc/foo"},
		{name: "custom tilde expansion", key: "custom:~/bin/foo", want: "/home/taro/bin/foo"},
		{name: "custom relative path errors", key: "custom:relative/path", wantErr: true},
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
