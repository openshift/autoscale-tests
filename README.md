# OpenShift Autoscale Tests

End-to-end test suites for autoscaling components on OpenShift clusters.

## Test Suites

| Suite | Directory | Description |
|-------|-----------|-------------|
| HPA | `test/e2e/hpa/` | Horizontal Pod Autoscaler (CPU, memory, container-resource scaling) |
| VPA | `test/e2e/vpa/` | Vertical Pod Autoscaler (recommender, admission controller, updater) |
| CRO | `test/e2e/cro/` | Cluster Resource Override admission webhook |
| CMA | `test/e2e/cma/` | Custom Metrics Autoscaler / KEDA (cron, CPU, memory, scale-to-zero) |
| CAS | `cas/` | Cluster Autoscaler (autoscaler, machine approver, ProvisioningRequest) |

The root module (`test/e2e/`) and CAS (`cas/`) are separate Go modules with independent dependency management.

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

## CAS Ginkgo Labels

CAS tests use [Ginkgo labels](https://onsi.github.io/ginkgo/#spec-labels) for test selection. The available labels are defined in `cas/pkg/framework/ginkgo-labels.go`:

| Label | Purpose |
|-------|---------|
| `autoscaler` | Cluster Autoscaler tests |
| `capi` | Cluster API tests |
| `ccm` | Cloud Controller Manager tests |
| `mapi` | Machine API tests |
| `machine-approver` | Machine Approver tests |
| `machine-health-check` | Machine Health Check tests |
| `periodic` | Tests meant for periodic CI runs |
| `tech-preview` | Tests requiring a TechPreview/DevPreview cluster |
| `disruptive` | Tests that may affect cluster stability |
| `LEVEL0` | Critical tests that block release if failed |
| `dev-only` | Tests that require a dev account |
| `qe-only` | Tests that require a QE account |
| `connected-only` | Tests that require a connected cluster |

Labels drive the Makefile targets:
- `test-e2e-cas` runs `--label-filter='!periodic'`
- `test-e2e-cas-periodic` runs `--label-filter='periodic'`
- `test-e2e-cas-techpreview` runs `--label-filter='tech-preview'`

## CI

This repository is consumed by OpenShift CI. The `.ci-operator.yaml` defines the build-root image. Prow job definitions live in the [openshift/release](https://github.com/openshift/release) repository.

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
