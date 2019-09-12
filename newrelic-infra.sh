#!/bin/sh

if [ ! -z "$NRIA_MONITOR_DOCKER" ]; then
    # enable nri-docker integration
    cp /etc/newrelic-infra/integrations.d/docker-config.yml.sample /etc/newrelic-infra/integrations.d/docker-config.yml
fi

/usr/bin/newrelic-infra
