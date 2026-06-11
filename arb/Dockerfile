# syntax=docker/dockerfile:1

ARG ADJUDICATION_REPO=https://github.com/jsmorph/adjudication.git
ARG ADJUDICATION_REF=arbattest
ARG PI_CODING_AGENT_PACKAGE=@earendil-works/pi-coding-agent@0.78.0

FROM golang:1.25-trixie AS build

ARG ADJUDICATION_REPO
ARG ADJUDICATION_REF

ENV ELAN_HOME=/opt/elan
ENV PATH=/opt/elan/bin:$PATH

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        build-essential \
        ca-certificates \
        curl \
        git \
        make \
        xz-utils \
    && rm -rf /var/lib/apt/lists/*

RUN curl -fsSL https://raw.githubusercontent.com/leanprover/elan/master/elan-init.sh -o /tmp/elan-init.sh \
    && sh /tmp/elan-init.sh -y --default-toolchain leanprover/lean4:v4.27.0 \
    && rm /tmp/elan-init.sh

WORKDIR /src

RUN git clone --branch "$ADJUDICATION_REF" --depth 1 "$ADJUDICATION_REPO" adjudication

WORKDIR /src/adjudication/arb

RUN make build

FROM debian:trixie-slim AS pi-rootfs

ARG PI_CODING_AGENT_PACKAGE

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        ca-certificates \
        curl \
        git \
        nodejs \
        npm \
        openssl \
        sudo \
    && rm -rf /var/lib/apt/lists/*

RUN npm install -g node@22.19.0 \
    && node --version \
    && npm install -g "${PI_CODING_AGENT_PACKAGE}" \
    && npm cache clean --force

RUN useradd -m user \
    && echo 'user ALL=(ALL) NOPASSWD:ALL' > /etc/sudoers.d/user

FROM pi-rootfs AS pi-export

RUN tar --numeric-owner --one-file-system \
    --exclude=./dev \
    --exclude=./proc \
    --exclude=./run \
    --exclude=./sys \
    --exclude=./tmp \
    -C / \
    -cpf /tmp/agentcourt-pi-sandbox-rootfs.tar \
    .

FROM docker:26-cli AS docker-cli

FROM debian:trixie-slim

ENV DEBIAN_FRONTEND=noninteractive
ENV AAR_PI_IMAGE=agentcourt-pi-sandbox:latest
ENV AAR_PI_ROOTFS_TAR=/opt/images/agentcourt-pi-sandbox-rootfs.tar

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates \
    && rm -rf /var/lib/apt/lists/*

COPY --from=docker-cli /usr/local/bin/docker /usr/local/bin/docker
COPY --from=build /src/adjudication /opt/adjudication
COPY --from=pi-export /tmp/agentcourt-pi-sandbox-rootfs.tar /opt/images/agentcourt-pi-sandbox-rootfs.tar

RUN cat > /usr/local/bin/aar-entrypoint <<'EOF' \
    && chmod 755 /usr/local/bin/aar-entrypoint
#!/bin/sh
set -eu

if [ -S /var/run/docker.sock ]; then
    if ! docker image inspect "$AAR_PI_IMAGE" >/dev/null 2>&1; then
        echo "Importing embedded Pi image as $AAR_PI_IMAGE" >&2
        docker import \
            --change 'ENTRYPOINT ["pi"]' \
            --change 'WORKDIR /home/user/work' \
            "$AAR_PI_ROOTFS_TAR" \
            "$AAR_PI_IMAGE" >/dev/null
    fi
fi

exec /opt/adjudication/arb/.bin/aar "$@"
EOF

WORKDIR /opt/adjudication/arb

ENTRYPOINT ["/usr/local/bin/aar-entrypoint"]
CMD ["help"]
