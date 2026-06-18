# Changelog

## [1.0.1] - 2026-06-18

### Changed
- Only download the CAIDA prefix2as file when a newer one is available. Each
  refresh now does a lightweight directory-listing check for the latest
  filename first and skips the data download, gzip parse, and route diff when
  it matches the file already ingested. CAIDA publishes roughly daily, so
  previously every refresh interval re-fetched and re-processed an identical
  ~4 MB file.

## [1.0.0]

### Added
- Initial release.
