# HPA Scale To Zero PoC

This project demonstrates Kubernetes v1.37 HorizontalPodAutoscaler scale-to-zero using an external metric.

## Architecture

```text
queue-lag-exporter -> Prometheus -> Prometheus Adapter -> HPA -> queue-worker
```

The exporter publishes `queue_consumer_lag`. Prometheus scrapes it every 10 seconds and Prometheus Adapter exposes it through `external.metrics.k8s.io`. The HPA scales `queue-worker` between zero and three replicas, targeting one worker per 30 queued tasks.

The worker is intentionally a `busybox` sleep loop. It represents a queue consumer without needing a real queue.

## Requirements

- Kubernetes v1.37 with `HPAScaleToZero` enabled. It is enabled by default in v1.37.
- Docker and Tailscale SSH access to the Dell k3s host.

## Deploy

```sh
make test
make deploy-dell DELL_HOST=100.77.239.77
```

The deploy command builds the exporter on the Dell, imports it into k3s containerd, then applies Prometheus, Prometheus Adapter, the worker, and the HPA.

## Test

Wait for the external metrics API and inspect the initial metric:

```sh
ssh root@100.77.239.77 'k3s kubectl get apiservice v1beta1.external.metrics.k8s.io'
ssh root@100.77.239.77 "k3s kubectl get --raw '/apis/external.metrics.k8s.io/v1beta1/namespaces/hpa-scale-to-zero/queue_consumer_lag?labelSelector=name%3Dworker_tasks'"
```

With queue lag set to `0`, the HPA scales the worker to zero and reports `ScaledToZero=True`:

```sh
ssh root@100.77.239.77 'k3s kubectl -n hpa-scale-to-zero get hpa queue-worker; k3s kubectl -n hpa-scale-to-zero describe hpa queue-worker'
```

Set queue lag to 30. The exporter restarts, Prometheus scrapes the new metric, and the HPA scales the worker back to one replica. This usually takes 30 to 60 seconds.

```sh
ssh root@100.77.239.77 'k3s kubectl -n hpa-scale-to-zero set env deployment/queue-lag-exporter QUEUE_LAG=30; k3s kubectl -n hpa-scale-to-zero rollout status deployment/queue-lag-exporter --timeout=120s'
ssh root@100.77.239.77 'k3s kubectl -n hpa-scale-to-zero get hpa,deployment,pods; k3s kubectl -n hpa-scale-to-zero describe hpa queue-worker'
```

Set `QUEUE_LAG=0` again to observe the next scale down:

```sh
ssh root@100.77.239.77 'k3s kubectl -n hpa-scale-to-zero set env deployment/queue-lag-exporter QUEUE_LAG=0'
```

## Cleanup

```sh
ssh root@100.77.239.77 'k3s kubectl delete namespace hpa-scale-to-zero; k3s kubectl delete apiservice v1beta1.external.metrics.k8s.io'
```

## Safety

This PoC does not expose Services externally and does not modify existing workloads. It creates only the `hpa-scale-to-zero` namespace plus cluster-scoped RBAC and the External Metrics APIService. The cleanup command removes all of them.
