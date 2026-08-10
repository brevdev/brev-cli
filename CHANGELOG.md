# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/)
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased](https://github.com/brevdev/brev-cli/compare/v0.0.0...HEAD)

### Added

[WIP] Add tailscale vpn client embedded with Brev.

- `brev join` for Brev/NetBird network membership, `brev leave` for membership teardown, and node-wide `brev disable-ssh`.

### Changed

- `brev join` no longer enables SSH. `brev enable-ssh` requires an existing joined membership and reconnects its tunnel when needed.

### Deprecated

- `brev register` and `brev deregister` remain compatibility aliases for `join` and `leave`, and warn on stderr when executed.

### Migration

- Scripts using `--ssh-port` must run `brev join` followed by `brev enable-ssh`.
