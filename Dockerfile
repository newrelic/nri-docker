ARG infra_image=newrelic/infrastructure-bundle

FROM golang:1.23.2 as builder

WORKDIR /go/src/github.com/newrelic/nri-docker
COPY . .

RUN make compile && \
    make test && \
    strip ./bin/nri-docker

FROM $infra_image

RUN rm -rf /etc/newrelic-infra/integrations.d/*
COPY --from=builder /go/src/github.com/newrelic/nri-docker/bin/nri-docker /var/db/newrelic-infra/newrelic-integrations/bin/
COPY --from=builder /go/src/github.com/newrelic/nri-docker/dev-config.yml.sample /etc/newrelic-infra/integrations.d/docker-config.yml
