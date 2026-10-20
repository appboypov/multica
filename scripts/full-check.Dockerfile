# Image of the full-check container that scripts/full-check.sh starts.
# The base is MadSpec's crabbox-local image (`crabbox-local build-image`); this
# adds the toolchains of the check at the versions go.mod, package.json and
# pnpm-lock.yaml pin, and the pnpm store and Go modules those lockfiles name,
# so a check spends no time installing or downloading them. The build context
# holds only this file and the lockfiles; full-check.sh streams it as a tar.
FROM madspec-crabbox-local:1
ARG DEBIAN_FRONTEND=noninteractive
RUN curl -fsSL https://go.dev/dl/go1.26.6.linux-arm64.tar.gz | tar -C /usr/local -xz \
 && ln -s /usr/local/go/bin/go /usr/local/bin/go && ln -s /usr/local/go/bin/gofmt /usr/local/bin/gofmt
RUN install -d /usr/share/postgresql-common/pgdg \
 && curl -fsSL -o /usr/share/postgresql-common/pgdg/apt.postgresql.org.asc https://www.postgresql.org/media/keys/ACCC4CF8.asc \
 && echo "deb [signed-by=/usr/share/postgresql-common/pgdg/apt.postgresql.org.asc] https://apt.postgresql.org/pub/repos/apt noble-pgdg main" > /etc/apt/sources.list.d/pgdg.list \
 && apt-get update && apt-get install -y --no-install-recommends postgresql-17 \
 && rm -rf /var/lib/apt/lists/*
RUN curl -fsSL https://astral.sh/uv/install.sh | env UV_INSTALL_DIR=/usr/local/bin INSTALLER_NO_MODIFY_PATH=1 sh
RUN corepack enable && COREPACK_HOME=/usr/local/share/corepack corepack prepare pnpm@10.28.2 --activate \
 && chmod -R a+rX /usr/local/share/corepack
ENV COREPACK_HOME=/usr/local/share/corepack
RUN install -d -o crabbox -g crabbox /home/crabbox/.cache \
 && PLAYWRIGHT_BROWSERS_PATH=/home/crabbox/.cache/ms-playwright npx --yes playwright@1.58.2 install --with-deps chromium \
 && chown -R crabbox:crabbox /home/crabbox/.cache
COPY pnpm-lock.yaml pnpm-workspace.yaml .npmrc /tmp/deps/
COPY server/go.mod server/go.sum /tmp/deps/server/
RUN chown -R crabbox:crabbox /tmp/deps \
 && runuser -u crabbox -- env COREPACK_HOME=/usr/local/share/corepack sh -c \
   'cd /tmp/deps && pnpm fetch --store-dir /home/crabbox/.cache/pnpm-store \
    && cd server && GOMODCACHE=/home/crabbox/.cache/go-mod go mod download' \
 && rm -rf /tmp/deps
