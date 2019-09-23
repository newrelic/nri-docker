# Change Log

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](http://keepachangelog.com/)
and this project adheres to [Semantic Versioning](http://semver.org/).

## 0.5.0 - 2019-09-23

- Removing `hostname: localhost` metric field, since the Agent now won't need
  it to decorate it with the proper hostname.
- Fixed a probrem in the cgroups library that prevented most metrics to be
  fetched in containers without Swap accounting in cgroups.

## 0.1.0 - 2019-09-20
### Added

- Initial version