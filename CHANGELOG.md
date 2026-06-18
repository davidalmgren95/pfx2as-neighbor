# Changelog

## [1.0.2] - 2026-06-18

### Added
- Respond to `SIGUSR1` by checking for a new prefix2as file out of cycle
  (downloads only if a newer one is available), and to `SIGUSR2` by forcing a
  re-download of the latest file even if it is unchanged.

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
