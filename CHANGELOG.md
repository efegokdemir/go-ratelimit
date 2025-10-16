# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Initial open source release
- Dual rate limiting (request count and bandwidth)
- Sliding window algorithm for accurate rate limiting
- Redis-based distributed rate limiting
- Resilient fallback to in-memory limiting when Redis is unavailable
- Dynamic per-client configuration via API
- RFC-compliant rate limit headers
- Customizable key generation functions
- Header-based rate limiting with sanitizers
- Selective path-based rate limiting
- Comprehensive middleware support
- Admin API for runtime configuration
- Structured logging with Zap and OpenTelemetry support
- Extensive test coverage
- Documentation and examples

### Changed
- Standardized licensing under Apache 2.0

### Security
- Added input sanitizers for header-based rate limiting
- Implemented secure defaults for all configurations

## [0.1.0] - 2025-01-01

Initial internal release (not publicly available)
