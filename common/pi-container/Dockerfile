FROM debian:trixie-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates curl git nodejs npm openssl sudo && \
    rm -rf /var/lib/apt/lists/*

ARG PI_MONO_REPO=https://github.com/jsmorph/pi-mono.git
ARG PI_MONO_REF=fix/abort-listener-warning
RUN git clone --depth 1 --branch ${PI_MONO_REF} ${PI_MONO_REPO} /tmp/pi-mono && \
    cd /tmp/pi-mono && \
    npm install && \
    npm run build --workspace @mariozechner/pi-ai && \
    npm run build --workspace @mariozechner/pi-agent-core && \
    npm run build --workspace @mariozechner/pi-tui && \
    npm run build --workspace @mariozechner/pi-coding-agent && \
    npm install -g ./packages/coding-agent && \
    rm -rf /tmp/pi-mono
COPY --from=pi_acp . /tmp/pi-acp
RUN cd /tmp/pi-acp && \
    npm ci && \
    npm run build && \
    npm install -g . && \
    rm -rf /tmp/pi-acp
COPY patch-openai-responses-websearch.js /tmp/patch-openai-responses-websearch.js
RUN node /tmp/patch-openai-responses-websearch.js && rm /tmp/patch-openai-responses-websearch.js

RUN useradd -m user && echo 'user ALL=(ALL) NOPASSWD:ALL' > /etc/sudoers.d/user
USER user
WORKDIR /home/user/work
ENTRYPOINT ["pi"]
