FROM newrelic/infrastructure-bundle:latest

ADD integrations.d/nri-config.yml /etc/newrelic-infra/integrations.d
ADD ../../bin/* /var/db/newrelic-infra/newrelic-integrations/bin/

RUN chmod +x /var/db/newrelic-infra/newrelic-integrations/bin/* || true
