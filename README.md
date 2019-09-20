# New Relic Infrastructure Integration for Docker

Reports status and metrics of Docker containers running into a host.

## Requirements

* Linux (Windows support TBD)
* New Relic Infrastructure Agent **1.5.42**
    - ⚠️ prior versions won't provide the data that is required for
      full functionality in the New Relic Site.
* Docker

## Running as a container

You need to put your New Relic Infrastructure license key as a value for
the `NRIA_LICENSE_KEY` property.

```
docker run -d --name newrelic-infra --network=host --cap-add=SYS_PTRACE \
-v "/:/host:ro" -v "/var/run/docker.sock:/var/run/docker.sock" \
-e NRIA_DOCKER_ENABLED="true" -e NRIA_CONNECT_ENABLED="true" \
-e NRIA_LICENSE_KEY="<your new relic license key>" \
xxxx:xxxx
```

`xxxx:xxxx` -> image still to be published 