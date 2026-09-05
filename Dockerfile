# LabSyslog production image: ghcr.io/hilather/labsyslog
#
# Multi-stage, static binary, numeric non-root UID, no shell.
# Run with a read-only root filesystem, cap_drop ALL, and no-new-privileges.
# Product YAML binds :514 (SWAP-001 / integrator compose). This image never
# runs as root. Smoke in this repo binds :1514 and does not add NET_BIND_SERVICE.

FROM golang:1.26.7-alpine AS build
WORKDIR /src

RUN apk add --no-cache ca-certificates tzdata

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_TIME=unknown
ARG TARGETARCH

RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH:-amd64} go build -trimpath \
	-ldflags="-s -w \
	-X github.com/hilather/go-lab-syslog/internal/buildinfo.version=${VERSION} \
	-X github.com/hilather/go-lab-syslog/internal/buildinfo.commit=${COMMIT} \
	-X github.com/hilather/go-lab-syslog/internal/buildinfo.buildTime=${BUILD_TIME}" \
	-o /out/labsyslog ./cmd/labsyslog \
	&& printf 'labsyslog:x:65532:65532:labsyslog:/:/sbin/nologin\n' > /out/passwd \
	&& printf 'labsyslog:x:65532:\n' > /out/group \
	&& cp /etc/ssl/certs/ca-certificates.crt /out/ca-certificates.crt \
	&& cp LICENSE /out/LICENSE

FROM scratch

LABEL org.opencontainers.image.title="labsyslog" \
	org.opencontainers.image.description="Receive-only laboratory syslog sink" \
	org.opencontainers.image.source="https://github.com/hilather/go-lab-syslog" \
	org.opencontainers.image.url="https://github.com/hilather/go-lab-syslog" \
	org.opencontainers.image.licenses="Apache-2.0" \
	org.opencontainers.image.vendor="hilather" \
	org.opencontainers.image.documentation="https://github.com/hilather/go-lab-syslog/blob/main/docs/11-deployment.md"

COPY --from=build /out/passwd /etc/passwd
COPY --from=build /out/group /etc/group
COPY --from=build /out/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /out/labsyslog /labsyslog
COPY --from=build /out/LICENSE /LICENSE

USER 65532:65532
EXPOSE 514/udp 514/tcp 8088/tcp
WORKDIR /

HEALTHCHECK --interval=10s --timeout=3s --start-period=3s --retries=3 \
	CMD ["/labsyslog", "healthcheck", "--url=http://127.0.0.1:8088/v1/health/ready"]

ENTRYPOINT ["/labsyslog"]
CMD ["serve", "--config=/etc/labsyslog/config.yaml", "--management-listen=:8088"]
