# Installation

English | [日本語](installation.ja.md)

Prebuilt binaries are published for Linux and macOS (amd64/arm64); Windows is not supported.

Just want to try it out? Run the install script, which downloads the latest release, verifies its checksum, and installs it to `$HOME/.local/bin` (override with `INSTALL_DIR`; pin a version with `TEN_VERSION`):

```bash
curl -fsSL https://raw.githubusercontent.com/rinsyan0518/ten/main/install.sh | sh
```

Or via Go:

```bash
go install github.com/rinsyan0518/ten/cmd/ten@latest
```

Or clone and build:

```bash
git clone https://github.com/rinsyan0518/ten.git
cd ten
go build -o ten ./cmd/ten
```

Next: [Quick start](quick-start.md)
