# New Relic Infrastructure Integration for Docker

Reports status and metrics of Docker containers running into a host.

## Disclaimer

This integration is at the moment at private beta stage. Running it does not
guarantee you get the full functionality in the New Relic site.

## Requirements

* Linux (Windows support TBD)
* New Relic Infrastructure Agent **1.5.42**
    - ⚠️ prior versions won't provide the data that is required for
      full functionality in the New Relic Site.
* Docker

## Configuration and running

At the current stage of development, the recommended way to run this
integration is inside a containerized agent:

```
docker run -d --name newrelic-infra --network=host --cap-add=SYS_PTRACE \
    -v "/:/host:ro" -v "/var/run/docker.sock:/var/run/docker.sock" \
    -e NRIA_DOCKER_ENABLED="true" -e NRIA_CONNECT_ENABLED="true" \
    -e NRIA_LICENSE_KEY="<your new relic license key>" \
    xxxx:xxxx
```

The next configuration options need to be passed to the environment of the Agent:

* `NRIA_LICENSE_KEY`: your New Relic Infrastructure license key.
* `NRIA_DOCKER_ENABLED="true"`: this will automatically execute the bundled
  `nri-docker` integration, without extra installation steps. 
* `NRIA_CONNECT_ENABLED="true"`: required for the proper identification of
  your containers as entities.
