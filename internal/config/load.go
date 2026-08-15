package config

import (
	"os"

	"github.com/BurntSushi/toml"
)

// LoadFile reads and parses a ten config file (ten.toml,
// ten.<profile>.toml, or ten.local.toml) at path. If the file does not
// exist — or can't be statted at all, e.g. because a parent directory
// component doesn't exist or isn't a directory — it returns a zero-value
// File and ok=false without error; all three file kinds are optional,
// and callers that care about *why* a path is unusable (e.g. apply's
// checkDesiredState) check that separately.
func LoadFile(path string) (file File, ok bool, err error) {
	if _, statErr := os.Stat(path); statErr != nil {
		return File{}, false, nil
	}
	if _, err := toml.DecodeFile(path, &file); err != nil {
		return File{}, false, err
	}
	return file, true, nil
}
