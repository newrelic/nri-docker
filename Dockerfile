ARG infra_image=newrelic/infrastructure:latest

FROM golang:1.10 as builder

WORKDIR /go/src/github.com/newrelic/nri-docker
COPY . .

RUN make compile && \
    strip ./bin/nri-docker

FROM $infra_image
COPY --from=builder /go/src/github.com/newrelic/nri-docker/bin/nri-docker /var/db/newrelic-infra/newrelic-integrations/bin/nri-docker
COPY --from=builder /go/src/github.com/newrelic/nri-docker/docker-definition.yml /var/db/newrelic-infra/newrelic-integrations/docker-definition.yml
COPY --from=builder /go/src/github.com/newrelic/nri-docker/docker-config.yml.sample /etc/newrelic-infra/integrations.d/docker-config.yml.sample
COPY --from=builder /go/src/github.com/newrelic/nri-docker/newrelic-infra.sh /newrelic-infra.sh

ENTRYPOINT ["/bin/sh"]
CMD ["/newrelic-infra.sh"]
