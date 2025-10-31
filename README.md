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
