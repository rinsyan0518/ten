package config

import (
	"os"

	"github.com/BurntSushi/toml"
)

// LoadLocal reads and parses ten.local.toml at path.
func LoadLocal(path string) (Local, error) {
	var l Local
	if _, err := toml.DecodeFile(path, &l); err != nil {
		return Local{}, err
	}
	return l, nil
}

// LoadRepo reads and parses a repository config file at path. If the file
// does not exist, it returns a zero-value Repo and ok=false without error
// (repo config files are optional).
func LoadRepo(path string) (repo Repo, ok bool, err error) {
	if _, statErr := os.Stat(path); os.IsNotExist(statErr) {
		return Repo{}, false, nil
	}
	if _, err := toml.DecodeFile(path, &repo); err != nil {
		return Repo{}, false, err
	}
	return repo, true, nil
}
