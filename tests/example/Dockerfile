FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Create host operator (bob) and AI agent (alice) users
RUN groupadd --gid 1000 bob \
 && useradd --uid 1000 --gid 1000 --create-home --shell /bin/bash bob \
 && groupadd --gid 1001 alice \
 && useradd --uid 1001 --gid 1001 --create-home --shell /bin/bash alice

# Credential directories owned by bob (mode 700)
RUN install -d -m 700 -o bob -g bob \
      /home/bob/.ssh \
      /home/bob/.aws \
      /home/bob/.gnupg \
      /home/bob/.kube \
      /home/bob/.config

# Synthetic key files - placeholder content, realistic structure
RUN echo 'PLACEHOLDER_SSH_KEY' > /home/bob/.ssh/id_rsa \
 && echo 'PLACEHOLDER_SSH_KEY' > /home/bob/.ssh/id_ed25519 \
 && chown -R bob:bob /home/bob/.ssh \
 && chmod 600 /home/bob/.ssh/id_rsa /home/bob/.ssh/id_ed25519

# Synthetic AWS config file
RUN printf '[default]\nregion = us-east-1\n' > /home/bob/.aws/credentials \
 && chown bob:bob /home/bob/.aws/credentials \
 && chmod 600 /home/bob/.aws/credentials

# Synthetic kubeconfig
RUN printf 'apiVersion: v1\nkind: Config\n' > /home/bob/.kube/config \
 && chown bob:bob /home/bob/.kube/config \
 && chmod 600 /home/bob/.kube/config

# Alice workspace
RUN install -d -m 755 -o alice -g alice /home/alice/workspace

# Copy sandbox-probe binary and probe config
COPY bin/sandbox-probe /usr/local/bin/sandbox-probe
RUN chmod 755 /usr/local/bin/sandbox-probe \
 && mkdir -p /etc/sandbox-probe
COPY tests/example/alice-sandbox.yaml /etc/sandbox-probe/alice-sandbox.yaml

USER alice
WORKDIR /home/alice/workspace

CMD ["sandbox-probe", "scan", "--config", "/etc/sandbox-probe/alice-sandbox.yaml"]
