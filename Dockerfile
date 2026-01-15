# Multi-stage build for pull-secret-mvp
# Stage 1: Builder
FROM registry.access.redhat.com/ubi9/go-toolset:1.23 AS builder

# Git commit passed from build machine (avoids needing git in container)
ARG GIT_COMMIT=unknown

# Build as root in builder stage (safe - final image uses non-root USER 1000)
USER root

# Set working directory
WORKDIR /workspace

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build binary using make to include version, commit, and build date
RUN make build GIT_COMMIT=${GIT_COMMIT}

# Stage 2: Runtime
FROM registry.access.redhat.com/ubi9/ubi-minimal:latest

# Install CA certificates for TLS
RUN microdnf install -y ca-certificates && microdnf clean all

# Create non-root user
RUN useradd -u 1000 -m -s /sbin/nologin pullsecret-job

# Set working directory
WORKDIR /app

# Copy binary from builder (make build outputs to bin/)
COPY --from=builder --chown=1000:1000 /workspace/bin/pull-secret /usr/local/bin/pull-secret

# Set permissions
RUN chmod 755 /usr/local/bin/pull-secret

# Use non-root user
USER 1000

# Set entrypoint
ENTRYPOINT ["/usr/local/bin/pull-secret"]

# Default command (can be overridden)
CMD ["run-job", "pull-secret"]
