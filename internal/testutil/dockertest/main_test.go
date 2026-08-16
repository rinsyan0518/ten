package dockertest_test

import (
	"os"
	"testing"

	"github.com/rinsyan0518/ten/internal/testutil/dockertest"
)

func TestMain(m *testing.M) {
	os.Exit(dockertest.RunWithSharedContainer(m, dockertest.Config{
		Context:    "../../..",
		Dockerfile: "Dockerfile.test",
	}))
}
