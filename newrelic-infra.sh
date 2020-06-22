#!/bin/sh

if [ "$NRIA_DOCKER_ENABLED" = "true" ]; then
    # enable nri-docker integration
    cp /etc/newrelic-infra/integrations.d/docker-config.yml.sample /etc/newrelic-infra/integrations.d/docker-config.yml
fi

exec tini -- /usr/bin/newrelic-infra
