# Use Ubuntu 16.04 as the base image
FROM ubuntu:16.04

# Define Go version
ARG GO_VERSION=1.23.2
ARG ARCH='amd64'
ARG GH_VERSION='2.61.0'

# Install dependencies
RUN apt-get update && apt-get install -y \
    gnupg-agent \
    unzip \
    zip \
    curl \
    wget \
    expect \
    git \
    tar \
    gcc \
    jq \
    g++ \
    gnupg2 \
    gnupg-agent \
    debsigs \
    rpm \
    build-essential \
    software-properties-common \
    python-software-properties \
    gcc-arm-linux-gnueabi \
    dpkg-sig \
    gcc-aarch64-linux-gnu

# Install Go
RUN curl -sSL https://golang.org/dl/go${GO_VERSION}.linux-${ARCH}.tar.gz -o go${GO_VERSION}.linux-${ARCH}.tar.gz && \
    tar -C /usr/local -xzf go${GO_VERSION}.linux-${ARCH}.tar.gz && \
    rm go${GO_VERSION}.linux-${ARCH}.tar.gz

# Set Go environment variables
ENV PATH="/usr/local/go/bin:${PATH}"
ENV GOPATH="/go"

# Optional: Set Go environment flags
ENV GOFLAGS="-buildvcs=false"

# Since the user does not match the owners of the repo "git rev-parse --is-inside-work-tree" fails and goreleaser does not populate projectName
# https://stackoverflow.com/questions/72978485/git-submodule-update-failed-with-fatal-detected-dubious-ownership-in-repositor
RUN git config --global --add safe.directory '*'
RUN curl -L https://github.com/cli/cli/releases/download/v${GH_VERSION}/gh_${GH_VERSION}_linux_amd64.deb -o gh_${GH_VERSION}_linux_amd64.deb
RUN dpkg -i gh_${GH_VERSION}_linux_amd64.deb
