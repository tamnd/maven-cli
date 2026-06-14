# maven

Search Maven Central artifacts

`maven` is a single pure-Go binary. It speaks to Maven Central over plain
HTTPS, shapes the responses into clean records, and pipes into the rest of your
tools. No API key, nothing to run alongside it.

## Install

```bash
go install github.com/tamnd/maven-cli/cmd/maven@latest
```

Or grab a prebuilt binary from the [releases](https://github.com/tamnd/maven-cli/releases), or run
the container image:

```bash
docker run --rm ghcr.io/tamnd/maven:latest --help
```

## Usage

```bash
maven --help
maven search "spring core"
maven info org.springframework spring-core
maven versions org.springframework spring-core --limit 20
```

## Development

```
cmd/maven/  thin main, wires cli.Root into fang
cli/        the cobra command tree
maven/      the library: HTTP client and data models
docs/       tago documentation site
```

```bash
make build      # ./bin/maven
make test       # go test ./...
make vet        # go vet ./...
```

## Releasing

Push a version tag and GitHub Actions runs GoReleaser, which builds the
archives, Linux packages, the multi-arch GHCR image, checksums, SBOMs, and a
cosign signature:

```bash
git tag v0.1.1
git push --tags
```

The Homebrew and Scoop steps self-disable until their tokens exist, so the first
release works with no extra secrets.

## License

Apache-2.0. See [LICENSE](LICENSE).
