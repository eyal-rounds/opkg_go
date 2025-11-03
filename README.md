# opkg_go

This repository provides a Go implementation of the `opkg` package manager. The
rewrite focuses on compatibility with existing configuration files while taking
advantage of Go's standard library for networking, concurrency and filesystem
operations.

## Features

- Parses legacy `opkg.conf` files, including `src/gz`, `dest`, `option` and
  `include` directives.
- Concurrently downloads repository indexes using Go goroutines and the
  `net/http` client instead of shelling out to external tools like `curl`.
- Supports listing packages, showing package metadata and downloading package
  archives into the configured cache directory.
- Reads the local status database to report installed packages.

## Building

```bash
$ go build ./...
```

## Packaging

To build a distributable binary that can be installed system-wide or shipped in a
package repository, run the command below. It produces the `opkg` executable in
the `dist/` directory, which you can then integrate into your packaging tooling
of choice (for example, creating a `.deb`, `.rpm`, or `opkg` package).

```bash
$ go build -o dist/opkg ./cmd/opkg
```

If you prefer to install the binary directly into your Go toolchain's `GOBIN`
directory, use `go install`:

```bash
$ go install ./cmd/opkg
```

## Usage

```bash
$ opkg -conf /etc/opkg/opkg.conf update
$ opkg list
$ opkg info busybox
$ opkg install busybox
```

Set the `OPKG_CONF` environment variable to override the configuration path.

## License

This project is licensed under the same terms as the original opkg project.
