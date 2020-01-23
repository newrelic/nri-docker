# New Relic Infrastructure Integration for Docker

Reports status and metrics of Docker containers running into a host.

## Disclaimer

This integration is at the moment at private beta stage. Running it does not
guarantee you get the full functionality in the New Relic site.

## Requirements

* Linux (Windows support TBD)
* New Relic Infrastructure Agent **1.5.42** or higher
    - ⚠️ prior versions won't provide the data that is required for
      full functionality in the New Relic Site.
* Docker

## Configuration and running

ℹ️ Since version 1.8.32, the New Relic Infrastructure agent bundles
the Docker integration, so there is no need to do anything to monitor
your containers.
