FROM golang:1.10 as builder

WORKDIR /go/src/github.com/newrelic/nri-docker
COPY . .

RUN make compile && \
    strip ./bin/nr-docker

FROM newrelic/infrastructure:latest
COPY --from=builder /go/src/github.com/newrelic/nri-docker/bin/nr-docker /var/db/newrelic-infra/newrelic-integrations/bin/nr-docker
COPY --from=builder /go/src/github.com/newrelic/nri-docker/docker-definition.yml /var/db/newrelic-infra/newrelic-integrations/docker-definition.yml
COPY --from=builder /go/src/github.com/newrelic/nri-docker/docker-config.yml.sample /etc/newrelic-infra/integrations.d/docker-config.yml.sample
COPY --from=builder /go/src/github.com/newrelic/nri-docker/newrelic-infra.sh /newrelic-infra.sh

ENTRYPOINT ["/bin/sh"]
CMD ["/newrelic-infra.sh"]
