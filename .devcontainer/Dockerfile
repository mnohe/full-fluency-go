FROM golang:1-bookworm

ARG DEBIAN_FRONTEND=noninteractive
ARG USERNAME=vscode
ARG USER_UID=1000
ARG USER_GID=$USER_UID

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        bash-completion \
        bubblewrap \
        build-essential \
        ca-certificates \
        curl \
        dnsutils \
        fd-find \
        fzf \
        git \
        gnupg \
        file \
        htop \
        iproute2 \
        jq \
        less \
        lsof \
        make \
        man-db \
        nano \
        netcat-openbsd \
        nodejs \
        npm \
        openssh-client \
        pkg-config \
        procps \
        python3 \
        python3-pip \
        ripgrep \
        rsync \
        shellcheck \
        sqlite3 \
        strace \
        sudo \
        tar \
        tmux \
        tree \
        unzip \
        vim \
        wget \
        xz-utils \
        zip \
    && ln -sf /usr/bin/fdfind /usr/local/bin/fd \
    && rm -rf /var/lib/apt/lists/*

RUN groupadd --gid "${USER_GID}" "${USERNAME}" \
    && useradd --uid "${USER_UID}" --gid "${USER_GID}" -m "${USERNAME}" -s /bin/bash \
    && echo "${USERNAME} ALL=(root) NOPASSWD:ALL" > "/etc/sudoers.d/${USERNAME}" \
    && chmod 0440 "/etc/sudoers.d/${USERNAME}"

ENV GOPATH=/go
ENV PATH="${GOPATH}/bin:${PATH}"
ENV GOTOOLCHAIN=auto

RUN go install golang.org/x/tools/gopls@latest \
    && go install github.com/go-delve/delve/cmd/dlv@latest \
    && go install honnef.co/go/tools/cmd/staticcheck@latest \
    && go install mvdan.cc/gofumpt@latest \
    && go install golang.org/x/tools/cmd/goimports@latest \
    && go install golang.org/x/vuln/cmd/govulncheck@latest \
    && go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest \
    && go install github.com/cweill/gotests/gotests@latest \
    && go install github.com/fatih/gomodifytags@latest \
    && go install github.com/josharian/impl@latest

RUN mkdir -p /go /home/${USERNAME}/.cache/go-build \
    && chown -R "${USERNAME}:${USERNAME}" /go "/home/${USERNAME}/.cache"

USER ${USERNAME}
WORKDIR /workspaces/go
