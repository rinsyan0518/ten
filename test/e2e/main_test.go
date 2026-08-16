package e2e_test

import (
	"os"
	"testing"

	"github.com/rinsyan0518/ten/internal/dockertest"
)

func TestMain(m *testing.M) {
	os.Exit(dockertest.RunWithSharedContainer(m))
}
