# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](http://keepachangelog.com/)
and this project adheres to [Semantic Versioning](http://semver.org/).

## 1.4.0 (2020-11-19)

### Changed
* nri-docker will no longer report containers that have been stopped for more than 24 hours.
  This value can be configured using the `EXITED_CONTAINERS_TTL` environment variable using 
  any value that can be parsed into a `time.Duration`, i.e. `1s`, `1m`, `1h`.
  To replicate the old behavior of the integration, set this environment variable to `0` (zero).

## 1.3.3 (2020-11-12)
### Changed
* Add metadata to samples from Fargate (#50)

## 1.3.2 (2020-07-17)
### Changed
* Set the correct integration Version

## 1.3.1 (2020-07-17)
### Changed
* Fixed bug in detection of non-running container in ECS environments.

## 1.3.0 (2020-05-11)
### Added
* Added support for cgroup driver 'systemd'.

## 1.2.1 (2020-04-15)
### Added
* Add enable condition in config for when the FARGATE env var is `"true"`.

## 1.2.0 (2020-04-01)
### Added
* **BETA** support for Fargate container metrics. For more information or access request please contact mfuentes@newrelic.com.
* Metric `memoryUsageLimitPercent` that reports the usage of the container memory as
  a percentage of the limit. If there is no limit defined, this metric is not reported.
* Renamed metrics: `processCount` to `threadCount`; `processCountLimit` to `threadCountLimit`

### Changed
* Metric `memorySizeLimitBytes` is not reported anymore when there is no such limit
  (before it was reported as `0`)

## 1.1.1 (2020-02-07)
### Changed

- This version fixes missing Docker container metrics improving Linux cgroup path detection. This issue was caused by cgroup not being mounted in the standard path `/sys/fs/cgroup`. This version can now discover cgroup different from the standard path.
- The auto-detected Cgroup path can be overwritten by the new config parameter 'cgroup_path'.
- Note: cgroup PIDs (process and thread count) are not available on Kernel versions lower than 4.3 [see support](http://man7.org/linux/man-pages/man7/cgroups.7.html). Therefore column threadCount won't be available for these systems.--

## 1.0.1 (2020-01-13)
### Changed
Updated execution conditions for integrations v4.

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
