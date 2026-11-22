# cli-zoo: a disposable environment with CLIs from help-text families the
# development host lacks (see docs/research/20261115_194227_UTC_cli_cross_section.md).
# treemand runs real CLIs with arguments, so a broad sweep belongs here, where
# nothing a CLI does can touch a developer's machine.
#
#   task zoo:build   build the image (from the repo root)
#   task zoo:smoke   full discovery of every zoo CLI; fails on a crash, error or empty tree
#   task zoo:capture write root-help fixtures into the corpus
#
# Every version is pinned and at least two weeks old when chosen; binaries
# downloaded outside a package manager are checked against their SHA-256.
#
# Multi-arch (linux/amd64, linux/arm64): BuildKit sets TARGETARCH, so the image
# builds natively on an x86 CI runner and on an arm64 Mac. Each raw download
# pins one SHA-256 per architecture, from the project's published checksums.

FROM --platform=$BUILDPLATFORM golang:1.26-bookworm AS build
ARG TARGETARCH
WORKDIR /src
COPY cli/treemand/go.mod cli/treemand/go.sum ./
RUN go mod download
COPY cli/treemand/ ./
RUN CGO_ENABLED=0 GOOS=linux GOARCH="${TARGETARCH}" go build -o /treemand .

FROM ubuntu:24.04
ARG TARGETARCH
ARG TERRAFORM_VERSION=1.16.7
ARG TERRAFORM_SHA256_amd64=5454efcf7fc87f0c2113111b9a5c7182f0ca1c6f0f23a1e110a50770a8fe473b
ARG TERRAFORM_SHA256_arm64=2d6a3f1098de94f1672aaa4e6873ef91ade9eac38851d3a403e2009193208de4
ARG PROMETHEUS_VERSION=3.15.1
ARG PROMETHEUS_SHA256_amd64=6a726885328ad9efebbf36e876ce2fb6225468a71fd1e7ce3ba666d5f4d97a84
ARG PROMETHEUS_SHA256_arm64=ceb6d21493c37ca0b4513b147642797c68cf39a0f50a444d50e1e80c74f21b26
ENV DEBIAN_FRONTEND=noninteractive \
    PIPX_HOME=/opt/pipx PIPX_BIN_DIR=/usr/local/bin \
    NPM_CONFIG_UPDATE_NOTIFIER=false \
    HEROKU_SKIP_ANALYTICS=1 HEROKU_DISABLE_AUTOUPDATE=1

# clap (ripgrep, fd, bat), Maven, Ruby for thor, Node for yargs/oclif, pipx for click/cleo.
RUN apt-get update -qq && apt-get install -y -qq --no-install-recommends \
      ca-certificates curl unzip jq man-db \
      ripgrep fd-find bat maven ruby nodejs npm pipx \
    && ln -s /usr/bin/fdfind /usr/local/bin/fd \
    && ln -s /usr/bin/batcat /usr/local/bin/bat \
    && rm -rf /var/lib/apt/lists/*

# mitchellh/cli (terraform) and kingpin (promtool), verified downloads.
RUN case "${TARGETARCH}" in \
      amd64) tf_sha="${TERRAFORM_SHA256_amd64}" prom_sha="${PROMETHEUS_SHA256_amd64}" ;; \
      arm64) tf_sha="${TERRAFORM_SHA256_arm64}" prom_sha="${PROMETHEUS_SHA256_arm64}" ;; \
      *) echo "cli-zoo: no pinned downloads for architecture '${TARGETARCH}'" >&2; exit 1 ;; \
    esac \
    && curl -fsSLo /tmp/tf.zip "https://releases.hashicorp.com/terraform/${TERRAFORM_VERSION}/terraform_${TERRAFORM_VERSION}_linux_${TARGETARCH}.zip" \
    && echo "${tf_sha}  /tmp/tf.zip" | sha256sum -c - \
    && unzip -q /tmp/tf.zip terraform -d /usr/local/bin && rm /tmp/tf.zip \
    && curl -fsSLo /tmp/prom.tgz "https://github.com/prometheus/prometheus/releases/download/v${PROMETHEUS_VERSION}/prometheus-${PROMETHEUS_VERSION}.linux-${TARGETARCH}.tar.gz" \
    && echo "${prom_sha}  /tmp/prom.tgz" | sha256sum -c - \
    && tar -xzf /tmp/prom.tgz -C /tmp && mv /tmp/prometheus-*/promtool /usr/local/bin/ && rm -rf /tmp/prom*

# click (black, flask, httpie) and cleo (poetry).
RUN pipx install black==26.10.1 && pipx install flask==3.1.3 \
    && pipx install poetry==2.5.1 && pipx install httpie==3.2.4

# yargs (eslint, jest), oclif (heroku), thor.
RUN npm install -g --no-fund --no-audit eslint@10.14.0 jest@30.5.2 heroku@11.12.1 \
    && gem install --no-document thor -v 1.5.0

COPY --from=build /treemand /usr/local/bin/treemand
COPY test/cli-zoo/run.sh /usr/local/bin/zoo
WORKDIR /work
ENTRYPOINT ["zoo"]
CMD ["smoke"]
