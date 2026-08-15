// Command checktag validates that a release tag is well-formed
// (vMAJOR.MINOR.PATCH or vMAJOR.MINOR.PATCH-rc.N) and strictly newer than
// every v* tag already in the repo, per SemVer precedence (a final release
// sorts after its own rc's). Used by `make check-tag` and release.yml
// before a release is ever cut.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

var tagPattern = regexp.MustCompile(`^v(\d+)\.(\d+)\.(\d+)(?:-rc\.(\d+))?$`)

type version struct {
	major, minor, patch, rc int
	isRC                    bool
}

// parseVersion parses a tag like "v0.2.0" or "v0.2.0-rc.3". ok is false if
// tag doesn't match the vMAJOR.MINOR.PATCH[-rc.N] format.
func parseVersion(tag string) (v version, ok bool) {
	m := tagPattern.FindStringSubmatch(tag)
	if m == nil {
		return version{}, false
	}
	v.major, _ = strconv.Atoi(m[1])
	v.minor, _ = strconv.Atoi(m[2])
	v.patch, _ = strconv.Atoi(m[3])
	if m[4] != "" {
		v.rc, _ = strconv.Atoi(m[4])
		v.isRC = true
	}
	return v, true
}

// compare returns -1, 0, or +1 as a sorts before, equal to, or after b.
func (a version) compare(b version) int {
	if a.major != b.major {
		return cmpInt(a.major, b.major)
	}
	if a.minor != b.minor {
		return cmpInt(a.minor, b.minor)
	}
	if a.patch != b.patch {
		return cmpInt(a.patch, b.patch)
	}
	switch {
	case a.isRC && !b.isRC:
		return -1
	case !a.isRC && b.isRC:
		return 1
	case a.isRC && b.isRC:
		return cmpInt(a.rc, b.rc)
	default:
		return 0
	}
}

func cmpInt(a, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

// findViolation returns the first tag in existingTags that makes newTag
// invalid to release (i.e. isn't strictly newer than it), or "" if newTag
// is newer than everything.
func findViolation(newTag string, newVersion version, existingTags []string) string {
	for _, existing := range existingTags {
		if existing == newTag {
			continue
		}
		existingVersion, ok := parseVersion(existing)
		if !ok {
			continue
		}
		if existingVersion.compare(newVersion) >= 0 {
			return existing
		}
	}
	return ""
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: checktag vX.Y.Z[-rc.N]")
		os.Exit(1)
	}
	tag := os.Args[1]

	newVersion, ok := parseVersion(tag)
	if !ok {
		fmt.Fprintf(os.Stderr, "error: %q is not a valid tag (expected vMAJOR.MINOR.PATCH or vMAJOR.MINOR.PATCH-rc.N)\n", tag)
		os.Exit(1)
	}

	if err := exec.Command("git", "fetch", "--tags", "--force", "--quiet").Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: git fetch --tags failed: %v\n", err)
		os.Exit(1)
	}

	out, err := exec.Command("git", "tag", "-l", "v*").Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: git tag -l failed: %v\n", err)
		os.Exit(1)
	}

	if violation := findViolation(tag, newVersion, strings.Fields(string(out))); violation != "" {
		fmt.Fprintf(os.Stderr, "error: %q is not newer than existing tag %q\n", tag, violation)
		os.Exit(1)
	}

	fmt.Printf("ok: %q is newer than all existing release tags\n", tag)
}
