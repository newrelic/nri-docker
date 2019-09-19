#!/bin/sh

if [ "$NRIA_DOCKER_ENABLED" = "yes" ]; then
    # enable nri-docker integration
    cp /etc/newrelic-infra/integrations.d/docker-config.yml.sample /etc/newrelic-infra/integrations.d/docker-config.yml
fi

tini -- /usr/bin/newrelic-infra
