#!/usr/bin/env sh
set -eu

: "${DELL_HOST:?Set DELL_HOST to the Dell Tailscale IP or hostname.}"
: "${DEPLOY_USER:=root}"
: "${IMAGE:=hpa-scale-to-zero-poc:dev}"
: "${UI_IMAGE:=hpa-scale-to-zero-control-ui:dev}"

root_dir=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
remote_dir=/root/hpa-scale-to-zero-poc
archive=/tmp/hpa-scale-to-zero-poc.tar.gz

cleanup() {
  rm -f "$archive"
}
trap cleanup EXIT

git -C "$root_dir" archive --format=tar.gz --output="$archive" HEAD
ssh "${DEPLOY_USER}@${DELL_HOST}" "mkdir -p '${remote_dir}'"
scp "$archive" "${DEPLOY_USER}@${DELL_HOST}:/tmp/hpa-scale-to-zero-poc.tar.gz"
ssh "${DEPLOY_USER}@${DELL_HOST}" "tar -C '${remote_dir}' -xzf /tmp/hpa-scale-to-zero-poc.tar.gz && docker build -t '${IMAGE}' '${remote_dir}' && docker build --build-arg TARGET=control-ui -t '${UI_IMAGE}' '${remote_dir}' && docker save '${IMAGE}' '${UI_IMAGE}' -o /tmp/hpa-scale-to-zero-poc-image.tar && k3s ctr images import /tmp/hpa-scale-to-zero-poc-image.tar && k3s kubectl apply -f '${remote_dir}/config/install.yaml' && k3s kubectl -n hpa-scale-to-zero rollout restart deployment/prometheus deployment/prometheus-adapter deployment/queue-lag-exporter deployment/hpa-control-ui && k3s kubectl -n hpa-scale-to-zero rollout status deployment/prometheus --timeout=180s && k3s kubectl -n hpa-scale-to-zero rollout status deployment/prometheus-adapter --timeout=180s && k3s kubectl -n hpa-scale-to-zero rollout status deployment/queue-lag-exporter --timeout=180s && k3s kubectl -n hpa-scale-to-zero rollout status deployment/hpa-control-ui --timeout=180s"
