# OpenShift Autoscale Tests

End-to-end test suites for the autoscaling components of OpenShift, owned by the
OpenShift Autoscaling team.

OpenShift ships several independent autoscalers, each in its own repository with its
own operator, operand, and for some products, its own release cadence. This repository is the single place where
their end-to-end behaviour is verified against a real cluster: it holds no product
code, only the tests and the shared Go framework they are built on. It is the
consolidation target for autoscaling tests that previously lived in the internal
`openshift-tests-private` repository.

## What is tested

| Suite | Directory | Component under test | Operator repository |
|-------|-----------|----------------------|---------------------|
| HPA | `test/e2e/hpa/` | Horizontal Pod Autoscaler — CPU, memory, and container-resource scaling | Built into OpenShift (`kube-controller-manager`); no operator |
| VPA | `test/e2e/vpa/` | Vertical Pod Autoscaler — recommender, admission controller, updater | [vertical-pod-autoscaler-operator](https://github.com/openshift/vertical-pod-autoscaler-operator) |
| CRO | `test/e2e/cro/` | Cluster Resource Override admission webhook | [cluster-resource-override-admission-operator](https://github.com/openshift/cluster-resource-override-admission-operator) |
| CMA | `test/e2e/cma/` | Custom Metrics Autoscaler (KEDA) — cron, CPU, memory, scale-to-zero | [custom-metrics-autoscaler-operator](https://github.com/openshift/custom-metrics-autoscaler-operator) |
| CAS | `cas/` | Cluster Autoscaler, machine approver, ProvisioningRequest | [cluster-autoscaler-operator](https://github.com/openshift/cluster-autoscaler-operator) |

The suites install the operator under test from the `redhat-operators` catalog when it
is not already present, so a stock OpenShift cluster is enough to run them.

The root module (`test/e2e/`) and CAS (`cas/`) are separate Go modules with independent
dependency management. (`test/e2e/`) may be further divided into separate Go modules in the future.

## Product documentation

The features exercised by these suites are documented for OpenShift users here:

- [Automatically scaling pods with the horizontal pod autoscaler](https://docs.openshift.com/container-platform/latest/nodes/pods/nodes-pods-autoscaling.html)
- [Automatically adjusting pod resource levels with the vertical pod autoscaler](https://docs.openshift.com/container-platform/latest/nodes/pods/nodes-pods-vertical-autoscaler.html)
- [Automatically scaling pods with the Custom Metrics Autoscaler Operator](https://docs.openshift.com/container-platform/latest/nodes/cma/nodes-cma-autoscaling-custom.html)
- [Overcommitting and the Cluster Resource Override Operator](https://docs.openshift.com/container-platform/latest/nodes/clusters/nodes-cluster-overcommit.html)
- [Applying autoscaling to a cluster](https://docs.openshift.com/container-platform/latest/machine_management/applying-autoscaling.html)

## Prerequisites

- An OpenShift cluster with `KUBECONFIG` set
- OLM (Operator Lifecycle Manager) for operator-based suites (VPA, CRO, CMA)
- For CAS tech-preview tests: a cluster with `TechPreviewNoUpgrade` or `DevPreviewNoUpgrade` feature set

## Running Tests

All test targets require a live OpenShift cluster.

### Quick Reference

```bash
# Root suites (HPA, VPA, CRO, CMA)
make test-e2e                   # All root E2E tests
make test-e2e-hpa               # HPA only
make test-e2e-vpa               # VPA only
make test-e2e-cro               # CRO only
make test-e2e-cma               # CMA (KEDA) only

# Cluster Autoscaler
make test-e2e-cas               # CAS non-periodic tests
make test-e2e-cas-periodic      # CAS periodic tests
make test-e2e-cas-techpreview   # CAS tests requiring a TechPreview cluster
```

Extra Ginkgo flags can be passed via `GINKGO_FLAGS`:

```bash
make test-e2e-hpa GINKGO_FLAGS="--label-filter=slow"
```

### Lint and Verify

```bash
make lint          # Root module
make cas-lint      # CAS module
make check         # Lint + unit tests
```

## CI

This repository is consumed by OpenShift CI. The `.ci-operator.yaml` defines the build-root image. Prow job definitions live in the [openshift/release](https://github.com/openshift/release/tree/master/ci-operator/config/openshift/autoscale-tests) repository.

When running in CI, the framework detects the environment via `OPENSHIFT_CI` and `ARTIFACT_DIR` variables and writes JUnit XML reports and artifacts accordingly.

## Project Layout

```
.
├── Makefile                 # Root build/test/lint targets
├── .ci-operator.yaml        # OpenShift CI build-root config
├── go.mod                   # Root Go module
├── pkg/framework/           # Shared test helpers (HPA, VPA, CRO, CMA)
├── test/e2e/                # Root E2E suites
│   ├── hpa/
│   ├── vpa/
│   ├── cro/
│   └── cma/
├── testdata/                # Test fixture YAMLs
└── cas/                     # CAS module (separate go.mod)
    ├── Makefile
    ├── hack/                # CI scripts (ci-integration.sh, formatting)
    └── pkg/
        ├── framework/       # CAS test framework, Ginkgo labels, helpers
        └── machineapi/      # Autoscaler and ProvisioningRequest tests
```

Each suite has its own README describing prerequisites and the scenarios it covers:
[HPA](test/e2e/hpa/README.md), [VPA](test/e2e/vpa/README.md),
[CRO](test/e2e/cro/README.md), [CMA](test/e2e/cma/README.md), [CAS](cas/README.md).

## More

- [CONTRIBUTING.md](CONTRIBUTING.md) — contribution workflow, PR conventions, review policy
- [AGENTS.md](AGENTS.md) — guidance for AI agents working in this repository
- [OpenShift CI configuration](https://github.com/openshift/release/tree/master/ci-operator/config/openshift/autoscale-tests)
- Operands under test: [kubernetes-autoscaler](https://github.com/openshift/kubernetes-autoscaler) (CAS, VPA),
  [kedacore-keda](https://github.com/openshift/kedacore-keda) (CMA),
  [cluster-resource-override-admission](https://github.com/openshift/cluster-resource-override-admission) (CRO)
