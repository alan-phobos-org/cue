# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.2.1] - 2026-01-13

### Fixed
- CI workflow now creates stub dist directory for Go embed during tests
- Frontend tests now install dependencies before running
- Release workflow simplified to use build.sh dist

### Changed
- Distribution tarball now includes README, LICENSE, and CHANGELOG

## [0.2.0] - 2026-01-11

### Fixed
- Security improvements in authentication middleware and API handlers
- Frontend error handling and state management improvements

### Changed
- Removed prototype HTML file (technical debt cleanup)
- Reorganized documentation structure

## [0.1.0] - 2026-01-11

### Added
- REST API backend (Go) with SQLite storage and FTS5 full-text search
- React frontend (TypeScript) with markdown rendering
- Items with titles, optional primary links (URL or file path), and markdown content
- Single-user mode with TLS encryption
- Multi-user mode with mTLS client certificate authentication
- Token-based authentication for API access (JWT-style tokens)
- Version information via `/api/status` endpoint
- Public `/api/health` endpoint for load balancer health checks
- Build system with lint, test, dist, and deploy targets
- PKI certificate generation for mTLS deployments

[Unreleased]: https://github.com/alan-phobos-org/cue/compare/v0.2.1...HEAD
[0.2.1]: https://github.com/alan-phobos-org/cue/compare/v0.2.0...v0.2.1
[0.2.0]: https://github.com/alan-phobos-org/cue/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/alan-phobos-org/cue/releases/tag/v0.1.0
