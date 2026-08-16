package dockertest_test

import (
	"testing"

	"github.com/rinsyan0518/ten/internal/testutil/dockertest"
)

func TestSandbox_WriteAndReadFile(t *testing.T) {
	sb := dockertest.NewSandbox(t)

	sb.WriteFile(t, sb.Home()+"/greeting.txt", "hello sandbox\n")
	got := sb.ReadFile(t, sb.Home()+"/greeting.txt")
	if got != "hello sandbox\n" {
		t.Fatalf("got %q", got)
	}
}
