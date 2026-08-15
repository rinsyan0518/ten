package main

import "testing"

func TestParseVersion(t *testing.T) {
	tests := []struct {
		tag  string
		want version
		ok   bool
	}{
		{"v0.1.0", version{major: 0, minor: 1, patch: 0}, true},
		{"v1.2.3-rc.4", version{major: 1, minor: 2, patch: 3, rc: 4, isRC: true}, true},
		{"v1", version{}, false},
		{"0.1.0", version{}, false},
		{"v0.1.0-beta", version{}, false},
		{"v0.1.0-rc.0", version{major: 0, minor: 1, patch: 0, rc: 0, isRC: true}, true},
	}
	for _, tt := range tests {
		got, ok := parseVersion(tt.tag)
		if ok != tt.ok {
			t.Errorf("parseVersion(%q) ok = %v, want %v", tt.tag, ok, tt.ok)
			continue
		}
		if ok && got != tt.want {
			t.Errorf("parseVersion(%q) = %+v, want %+v", tt.tag, got, tt.want)
		}
	}
}

func TestVersionCompare(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"v0.1.5", "v0.1.3", 1},
		{"v0.1.3", "v0.1.5", -1},
		{"v0.2.0", "v0.2.0-rc.1", 1}, // final sorts after its own rc
		{"v0.2.0-rc.1", "v0.2.0", -1},
		{"v0.2.0-rc.1", "v0.2.0-rc.2", -1},
		{"v0.2.0-rc.3", "v0.1.9", 1}, // rc still beats a lower major.minor.patch
		{"v0.10.0", "v0.9.0", 1},     // numeric, not lexicographic
		{"v0.2.0", "v0.2.0", 0},
		{"v1.0.0", "v0.99.99", 1},
	}
	for _, tt := range tests {
		a, ok := parseVersion(tt.a)
		if !ok {
			t.Fatalf("parseVersion(%q) failed", tt.a)
		}
		b, ok := parseVersion(tt.b)
		if !ok {
			t.Fatalf("parseVersion(%q) failed", tt.b)
		}
		if got := a.compare(b); got != tt.want {
			t.Errorf("(%q).compare(%q) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestFindViolation(t *testing.T) {
	tests := []struct {
		name         string
		newTag       string
		existingTags []string
		want         string
	}{
		{
			name:         "no existing tags",
			newTag:       "v0.1.0",
			existingTags: nil,
			want:         "",
		},
		{
			name:         "strictly newer than everything",
			newTag:       "v0.1.6",
			existingTags: []string{"v0.1.0", "v0.1.5"},
			want:         "",
		},
		{
			name:         "older than an existing release",
			newTag:       "v0.1.3",
			existingTags: []string{"v0.1.0", "v0.1.5"},
			want:         "v0.1.5",
		},
		{
			// git itself rejects a non-force push of a tag name that
			// already exists remotely, so by the time this runs in CI,
			// newTag's own entry in `git tag -l` is always this same
			// push — never a genuine second, distinct release of the
			// same version. It must be excluded, or every release would
			// find itself and report a violation against itself.
			name:         "excludes itself from comparison",
			newTag:       "v0.2.0",
			existingTags: []string{"v0.1.5", "v0.2.0"},
			want:         "",
		},
		{
			name:         "promoting an rc to final is allowed",
			newTag:       "v0.2.0",
			existingTags: []string{"v0.2.0-rc.1", "v0.2.0-rc.2"},
			want:         "",
		},
		{
			name:         "rc after its own final is blocked",
			newTag:       "v0.2.0-rc.3",
			existingTags: []string{"v0.2.0"},
			want:         "v0.2.0",
		},
		{
			name:         "ignores tags that don't match the format",
			newTag:       "v0.1.0",
			existingTags: []string{"not-a-version", "v0.0.9"},
			want:         "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			newVersion, ok := parseVersion(tt.newTag)
			if !ok {
				t.Fatalf("parseVersion(%q) failed", tt.newTag)
			}
			if got := findViolation(tt.newTag, newVersion, tt.existingTags); got != tt.want {
				t.Errorf("findViolation(%q, ..., %v) = %q, want %q", tt.newTag, tt.existingTags, got, tt.want)
			}
		})
	}
}
