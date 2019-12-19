# Change Log

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](http://keepachangelog.com/)
and this project adheres to [Semantic Versioning](http://semver.org/).

## 1.0.0 (2019-12-09)
### Changed
Updated agent dependency to newrelic-infra >= 1.8.0
Updated config files to agent integrations format v4.
Fixed target OS to Linux.

### Containerised agent
Replaced entrypoint from /usr/bin/newrelic-infra to /usr/bin/newrelic-infra-service

## 0.6.0 (2019-11-20)
### Changed
- Renamed the integration executable from nr-docker to nri-docker in order to be consistent with the package naming. **Important Note:** if you have any security module rules (eg. SELinux), alerts or automation that depends on the name of this binary, these will have to be updated.

## 0.5.1 - 2019-09-25

- Added support for custom Cgroup parent paths

## 0.5.0 - 2019-09-23

- Removing `hostname: localhost` metric field, since the Agent now won't need
  it to decorate it with the proper hostname.
- Fixed a probrem in the cgroups library that prevented most metrics to be
  fetched in containers without Swap accounting in cgroups.

## 0.1.0 - 2019-09-20
### Added

- Initial version
