#!/usr/bin/env bash
# Container contract for DEP-001. Requires Docker. Fail closed if the daemon
# is missing so this is a real check, not an unimplemented stub.
# Binds container :1514 UDP+TCP with cap_drop ALL and no NET_BIND_SERVICE.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
IMAGE="${LABSYSLOG_TEST_IMAGE:-ghcr.io/hilather/labsyslog:test}"
NAME="labsyslog-container-test-$$"
COMPOSE="${ROOT}/examples/compose.smoke.yaml"
CONFIG="${ROOT}/testdata/container/config.yaml"

if ! command -v docker >/dev/null 2>&1; then
	echo "docker is required for make test-container" >&2
	exit 1
fi
if ! docker info >/dev/null 2>&1; then
	echo "docker daemon is not available for make test-container" >&2
	exit 1
fi
if ! command -v curl >/dev/null 2>&1; then
	echo "curl is required for make test-container" >&2
	exit 1
fi
if ! command -v python3 >/dev/null 2>&1; then
	echo "python3 is required for make test-container" >&2
	exit 1
fi
if [ ! -f "${CONFIG}" ]; then
	echo "missing ${CONFIG}" >&2
	exit 1
fi
if grep -E '^[[:space:]]*cap_add:' "${COMPOSE}" >/dev/null; then
	echo "${COMPOSE} must not cap_add (no NET_BIND_SERVICE)" >&2
	exit 1
fi

WORKDIR="$(mktemp -d)"
TOKEN="${WORKDIR}/token"
cleanup() {
	docker rm -f "${NAME}" >/dev/null 2>&1 || true
	rm -rf "${WORKDIR}"
}
trap cleanup EXIT

# Frozen secret mount is 0o644 so UID 65532 can read a bind-mounted file.
python3 - "${TOKEN}" <<'PY'
import secrets
import sys
from pathlib import Path
Path(sys.argv[1]).write_text(secrets.token_urlsafe(48), encoding="utf-8")
PY
chmod 644 "${TOKEN}"
if [ "$(wc -c < "${TOKEN}")" -lt 32 ]; then
	echo "minted token is shorter than 32 bytes" >&2
	exit 1
fi

echo "building ${IMAGE}"
docker build -t "${IMAGE}" "${ROOT}"

inspect_user="$(docker image inspect --format '{{.Config.User}}' "${IMAGE}")"
if [ "${inspect_user}" != "65532:65532" ]; then
	echo "image User=${inspect_user}, want 65532:65532" >&2
	exit 1
fi

licenses="$(docker image inspect --format '{{index .Config.Labels "org.opencontainers.image.licenses"}}' "${IMAGE}")"
if [ "${licenses}" != "Apache-2.0" ]; then
	echo "image license label=${licenses}, want Apache-2.0" >&2
	exit 1
fi

cmd="$(docker image inspect --format '{{json .Config.Cmd}}' "${IMAGE}")"
if [ "${cmd}" != '["serve","--config=/etc/labsyslog/config.yaml","--management-listen=:8088"]' ]; then
	echo "image Cmd=${cmd}, want serve + config + --management-listen=:8088" >&2
	exit 1
fi

entrypoint="$(docker image inspect --format '{{json .Config.Entrypoint}}' "${IMAGE}")"
if [ "${entrypoint}" != '["/labsyslog"]' ]; then
	echo "image Entrypoint=${entrypoint}, want [\"/labsyslog\"]" >&2
	exit 1
fi

hc="$(docker image inspect --format '{{json .Config.Healthcheck.Test}}' "${IMAGE}")"
case "${hc}" in
*CMD-SHELL*)
	echo "image HEALTHCHECK=${hc} must be exec form, not shell" >&2
	exit 1
	;;
esac
case "${hc}" in
'["CMD",'*)
	;;
*)
	echo "image HEALTHCHECK=${hc}, want JSON array starting with CMD" >&2
	exit 1
	;;
esac
case "${hc}" in
*'/v1/health/ready'*)
	;;
*)
	echo "image HEALTHCHECK=${hc}, want /v1/health/ready" >&2
	exit 1
	;;
esac
case "${hc}" in
*healthcheck*)
	;;
*)
	echo "image HEALTHCHECK=${hc}, want exec-form labsyslog healthcheck" >&2
	exit 1
	;;
esac

if docker compose version >/dev/null 2>&1; then
	docker compose -f "${COMPOSE}" config >/dev/null
else
	echo "docker compose plugin not available; compose file parse skipped" >&2
fi

docker run -d --name "${NAME}" \
	--read-only \
	--cap-drop=ALL \
	--security-opt=no-new-privileges:true \
	--tmpfs /tmp:rw,noexec,nosuid,size=16m \
	-v "${CONFIG}:/etc/labsyslog/config.yaml:ro" \
	-v "${TOKEN}:/run/secrets/labsyslog-token:ro" \
	-p 127.0.0.1::1514/udp \
	-p 127.0.0.1::1514/tcp \
	-p 127.0.0.1::8088/tcp \
	"${IMAGE}"

if [ "$(docker inspect --format '{{.State.Running}}' "${NAME}")" != "true" ]; then
	echo "container is not running" >&2
	docker inspect --format 'status={{.State.Status}} exit={{.State.ExitCode}} error={{.State.Error}}' "${NAME}" >&2 || true
	docker logs "${NAME}" >&2 || true
	exit 1
fi

readonly_root="$(docker inspect --format '{{.HostConfig.ReadonlyRootfs}}' "${NAME}")"
if [ "${readonly_root}" != "true" ]; then
	echo "HostConfig.ReadonlyRootfs=${readonly_root}, want true" >&2
	exit 1
fi

capadd="$(docker inspect --format '{{json .HostConfig.CapAdd}}' "${NAME}")"
if [ "${capadd}" != "null" ] && [ "${capadd}" != "[]" ]; then
	echo "CapAdd=${capadd}, want none (no NET_BIND_SERVICE)" >&2
	exit 1
fi

capdrop="$(docker inspect --format '{{json .HostConfig.CapDrop}}' "${NAME}")"
case "${capdrop}" in
*ALL*)
	;;
*)
	echo "CapDrop=${capdrop}, want ALL" >&2
	exit 1
	;;
esac

privileged="$(docker inspect --format '{{.HostConfig.Privileged}}' "${NAME}")"
if [ "${privileged}" != "false" ]; then
	echo "Privileged=${privileged}, want false" >&2
	exit 1
fi

assert_identity() {
	local uid capeef pid
	pid="$(docker inspect --format '{{.State.Pid}}' "${NAME}")"
	if [ -n "${pid}" ] && [ "${pid}" != "0" ] && [ -r "/proc/${pid}/status" ]; then
		uid="$(awk '/^Uid:/{print $2}' "/proc/${pid}/status")"
		capeef="$(awk '/^CapEff:/{print $2}' "/proc/${pid}/status")"
		if [ "${uid}" = "65532" ] && [ "${capeef}" = "0000000000000000" ]; then
			return 0
		fi
		echo "host /proc Uid=${uid} CapEff=${capeef}, want 65532 / 0000000000000000" >&2
		return 1
	fi
	echo "CapEff not readable from this client; accepted CapDrop=${capdrop} Privileged=${privileged}" >&2
	return 0
}
assert_identity

mgmt_port="$(docker port "${NAME}" 8088/tcp | head -n1 | awk -F: '{print $NF}')"
udp_port="$(docker port "${NAME}" 1514/udp | head -n1 | awk -F: '{print $NF}')"
tcp_port="$(docker port "${NAME}" 1514/tcp | head -n1 | awk -F: '{print $NF}')"
if [ -z "${mgmt_port}" ] || [ -z "${udp_port}" ] || [ -z "${tcp_port}" ]; then
	echo "published ports missing mgmt=${mgmt_port} udp=${udp_port} tcp=${tcp_port}" >&2
	docker inspect --format '{{json .HostConfig.PortBindings}}' "${NAME}" >&2 || true
	docker logs "${NAME}" >&2 || true
	exit 1
fi

ok=0
for _ in $(seq 1 40); do
	if curl -fsS "http://127.0.0.1:${mgmt_port}/v1/health/ready" >/dev/null 2>&1; then
		ok=1
		break
	fi
	sleep 0.25
done
if [ "${ok}" -ne 1 ]; then
	echo "management ready check failed on 127.0.0.1:${mgmt_port}" >&2
	docker logs "${NAME}" >&2 || true
	exit 1
fi

if ! docker exec "${NAME}" /labsyslog version >/dev/null; then
	echo "non-root exec of /labsyslog version failed" >&2
	exit 1
fi
if ! docker exec "${NAME}" /labsyslog healthcheck --url=http://127.0.0.1:8088/v1/health/ready >/dev/null; then
	echo "in-container HTTP ready healthcheck failed" >&2
	exit 1
fi
if docker exec "${NAME}" /bin/sh -c true >/dev/null 2>&1; then
	echo "image has a shell at /bin/sh" >&2
	exit 1
fi
if docker exec "${NAME}" /bin/busybox true >/dev/null 2>&1; then
	echo "image has busybox" >&2
	exit 1
fi

SMOKE_TOKEN="$(tr -d '\r\n' < "${TOKEN}")"
AUTH=("Authorization: Bearer ${SMOKE_TOKEN}")

unauth="$(curl -sS -o /dev/null -w '%{http_code}' "http://127.0.0.1:${mgmt_port}/v1/messages")"
if [ "${unauth}" != "401" ]; then
	echo "unauthenticated GET /v1/messages status=${unauth}, want 401" >&2
	exit 1
fi

send_syslog() {
	python3 - "$1" 127.0.0.1 "$2" "$3" <<'PY'
import socket
import sys

kind, host, port_s, payload = sys.argv[1], sys.argv[2], sys.argv[3], sys.argv[4]
port = int(port_s)
data = payload.encode()
if kind == "udp":
    sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
    sock.sendto(data, (host, port))
    sock.close()
elif kind == "tcp-octet":
    frame = f"{len(data)} ".encode() + data
    sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    sock.settimeout(5)
    sock.connect((host, port))
    sock.sendall(frame)
    sock.close()
else:
    raise SystemExit(f"unknown kind {kind}")
PY
}

RFC3164='<14>Sep  4 20:52:35 testhost labsyslog: container-smoke-udp'
RFC5424='<165>1 2026-09-04T20:52:35.000Z sut-1 sshd 1234 ID47 [sshd@0 user="alice"] container-smoke-tcp'

send_syslog udp "${udp_port}" "${RFC3164}"
send_syslog tcp-octet "${tcp_port}" "${RFC5424}"

wait_contains() {
	local needle="$1"
	local body
	body="$(curl -fsS -H "${AUTH[0]}" -H 'Content-Type: application/json' \
		-d "{\"timeout\":\"8s\",\"filter\":{\"messageContains\":\"${needle}\"}}" \
		"http://127.0.0.1:${mgmt_port}/v1/messages:wait")"
	if ! printf '%s\n' "${body}" | grep -q "${needle}"; then
		echo "wait missing ${needle}: ${body}" >&2
		docker logs "${NAME}" >&2 || true
		exit 1
	fi
}

wait_contains "container-smoke-udp"
wait_contains "container-smoke-tcp"

reset_body="$(curl -fsS -X POST -H "${AUTH[0]}" "http://127.0.0.1:${mgmt_port}/v1/state:reset")"
if ! printf '%s\n' "${reset_body}" | grep -q '"kind":"LabSyslog"'; then
	echo "reset missing LabSyslog state: ${reset_body}" >&2
	exit 1
fi

listed="$(curl -fsS -H "${AUTH[0]}" "http://127.0.0.1:${mgmt_port}/v1/messages")"
if printf '%s\n' "${listed}" | grep -q 'container-smoke'; then
	echo "messages after reset still contain smoke payload: ${listed}" >&2
	exit 1
fi

echo "container contract ok image=${IMAGE}"
