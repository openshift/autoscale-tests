# AGENTS.md — openshift/autoscale-tests

Guidance for AI agents working in this repository. See [README.md](README.md) for user
documentation and [CONTRIBUTING.md](CONTRIBUTING.md) for the contribution workflow.

## Project Overview

This repository contains the end-to-end test suites for OpenShift's autoscaling
components: the Horizontal Pod Autoscaler, the Vertical Pod Autoscaler, the Custom
Metrics Autoscaler (KEDA), the Cluster Resource Override admission webhook, and the
Cluster Autoscaler. It is owned by the OpenShift Autoscaling team and is the
consolidation target for autoscaling tests migrating out of
the internal `openshift-tests-private` repository.

There is **no product code here**. Everything in this repo either asserts something
about a component that lives in another repository, or is shared machinery for doing
so. The product repositories are listed in [CONTRIBUTING.md](CONTRIBUTING.md); this one
only tests them.

Every meaningful test requires a live OpenShift cluster. There is no envtest, no fake
client, and no controller to unit-test. That single fact drives most of what follows:
you cannot verify a change here by running `go test`.

## Repository Structure

```
pkg/framework/            Shared helpers for the root suites
  framework.go            Client/scheme construction, pod/namespace/deployment/HPA
                          helpers, testdata template rendering
  operators.go            OLM install/uninstall, the `Operators` registry, namespace
                          constants
  resource_consumer.go    resource-consumer workload + background CPU/memory load loops
  vpa.go                  VerticalPodAutoscaler(Controller) helpers, recommendations
  keda.go                 ScaledObject helpers, KEDA scale assertions
  cro.go                  ClusterResourceOverride helpers, resource assertions
test/e2e/                 Root E2E suites; one package per component
  hpa/ vpa/ cro/ cma/     <name>_init_test.go bootstraps Ginkgo, <name>_test.go is the
                          suite, README.md lists the scenarios
testdata/                 Go-template YAML fixtures, rendered at runtime by path
cas/                      Cluster Autoscaler suite — a SEPARATE Go module
  Makefile                CAS-only lint/vendor/e2e targets
  hack/ci-integration.sh  Ginkgo CLI entrypoint used by CI and `make test-e2e-cas`
  pkg/e2e_test.go         Suite bootstrap, platform-dependent timeout tuning
  pkg/framework/          CAS framework: machines, machinesets, nodes, gatherers
  pkg/framework/ginkgo-labels.go  The label taxonomy (CAS only)
  pkg/machineapi/         The actual CAS specs
  vendor/                 Vendored deps (CAS only; the root module is not vendored)
Makefile                  Root targets; delegates CAS targets to `make -C cas`
.golangci.yml             Lint config for BOTH modules (cas/ finds it by upward search)
.ci-operator.yaml         OpenShift CI build-root image
```

## Architecture: What Is Not Obvious

### Two Go modules, two toolchains

`github.com/openshift/autoscale-tests` (root) and
`github.com/openshift/autoscale-tests/cas` are independent modules. The root module is
not vendored and runs specs through plain `go test`; `cas/` is vendored
(`GOFLAGS=-mod=vendor`) and runs specs through the vendored Ginkgo CLI via
`cas/hack/ci-integration.sh`, which sets the JUnit report path from `$JUNIT_DIR`. Root
commands do not traverse `cas/` and vice versa.

### Typed for core APIs, unstructured for everything else

`newScheme()` in `pkg/framework/framework.go` registers only the client-go scheme plus
`core/v1`, `apps/v1`, `autoscaling/v1`, and `autoscaling/v2`. Everything else — OLM
`Subscription`/`OperatorGroup`/`ClusterServiceVersion`, KEDA `ScaledObject`, VPA
`VerticalPodAutoscaler` and `VerticalPodAutoscalerController`, CRO
`ClusterResourceOverride` — is manipulated as `unstructured.Unstructured` through the
controller-runtime client or the dynamic client. This is deliberate: it keeps the root
module free of dependencies on the four product repositories, at the cost of
stringly-typed field access.

### Operators are installed on demand, and only uninstalled if we installed them

The VPA, CRO, and CMA suites check `IsOperatorInstalled` in `BeforeSuite`. If the
operator is absent they subscribe to it from the `redhat-operators` catalog
(`InstallOperatorByKey`, driven by the `Operators` map in
`pkg/framework/operators.go`) and record that they did. `AfterSuite` uninstalls **only
in that case**, so a suite run against a cluster with a pre-installed operator leaves
it alone. The suites are therefore safe to run repeatedly and in any order on a
long-lived cluster.

### Load generation is driven from the test process

`pkg/framework/resource_consumer.go` deploys the upstream `resource-consumer` image and
then drives load from the test binary by POSTing to each pod through the API server's
pod `proxy` subresource, splitting the requested total across the running pods. Each
request asks for ~40 seconds of load and a background goroutine re-sends every 30
seconds, so load persists across scale-ups only as long as the loop is alive.
`CleanUp()` closes the stop channels; skipping it leaves goroutines loading pods after
the spec has finished.

### Cluster-singleton resources

Test namespaces are unique per spec (`CreateTestNamespace` appends `UnixNano`), but two
resources are cluster-wide singletons that specs mutate in place:

- CRO: `ClusterResourceOverride` named `cluster` (cluster-scoped)
- VPA: `VerticalPodAutoscalerController` named `default` in
  `openshift-vertical-pod-autoscaler`

Anything that changes their spec is globally visible for the duration of the run.

## Common Pitfalls

1. **`go test ./...` proves nothing here.** There are no unit tests and no envtest;
   `make test-unit` passes on an empty result set. A change is only verified by running
   the affected suite against a real cluster. Never report a change as tested because
   the module compiles.

2. **Root commands skip `cas/`.** `make lint`, `make fmt`, and `make test-unit` operate
   on the root module only. CAS has its own targets (`make cas-lint`,
   `make -C cas vendor`). A change touching both modules needs both sets of commands.

3. **The two modules lint with different golangci-lint versions.** The root Makefile
   `go install`s `v2.2.1`; `cas/` runs the version vendored in `cas/go.mod` (currently
   `v2.11.1`) via `go run ./vendor/...`. They share the single root `.golangci.yml`,
   which `cas/` picks up by upward directory search. A lint rule that passes in one
   module can fail in the other — do not "fix" a CAS lint error by editing the root
   config without checking both.

4. **Adding a typed API means updating `newScheme()`.** If you add a typed client call
   for an API group that is not registered, it fails at runtime with "no kind is
   registered", not at compile time. Either register the scheme or use
   `unstructured` like the existing VPA/KEDA/CRO helpers do.

5. **`testdata/` is resolved from the source file's location, not the working
   directory.** `RenderTemplate` (`pkg/framework/framework.go`) computes the repo root
   as `runtime.Caller(0)` plus `../..`. Moving `pkg/framework/` deeper or relocating
   `testdata/` breaks template rendering at runtime with no compile error. There is
   also no compile-time reference from Go to any template filename — renaming a file in
   `testdata/` will not break the build.

6. **Do not add an unconditional operator uninstall.** `AfterSuite` deliberately skips
   the uninstall when the operator was already present. Removing that guard would
   uninstall a product operator out from under a shared or long-lived CI cluster.

7. **Do not let CRO or VPA specs assume exclusive ownership of the singletons.** The
   `ClusterResourceOverride` named `cluster` and the `VerticalPodAutoscalerController`
   named `default` are shared. Specs that mutate them must restore or overwrite state
   explicitly rather than relying on ordering.

8. **Ginkgo labels exist only in the CAS module.** `cas/pkg/framework/ginkgo-labels.go`
   defines the taxonomy, and the CAS Makefile targets filter on it. The root suites
   carry no labels, so passing `GINKGO_FLAGS="--label-filter=..."` to
   `make test-e2e-hpa` and friends silently matches nothing.

9. **Timeouts are the difference between a test and a flake.** Autoscaling is slow:
   HPA reacts on a metrics-scrape cadence, VPA eviction waits on the updater, CAS waits
   on real machine provisioning. Use the `wait.PollUntilContextTimeout` helpers already
   in `pkg/framework/`, never `time.Sleep` as a synchronisation primitive, and set
   timeouts generously. `cas/pkg/e2e_test.go` already extends `WaitShort`/`WaitMedium`/
   `WaitLong` on the slower platforms — follow that pattern rather than hardcoding.

10. **CI does not run most of this repo.** Only the CAS lint and CAS e2e jobs exist, all
    marked `optional`, and the cloud jobs are gated on changes under `cas/`. Nothing
    runs HPA, VPA, CRO, or CMA automatically. A green PR is not evidence; say what you
    ran with `/verified by ...`.

11. **Prow jobs are not configured in this repository.** They live in
    [openshift/release](https://github.com/openshift/release/tree/master/ci-operator/config/openshift/autoscale-tests).
    Adding a Makefile target does not add a CI job — that is a separate PR in a separate
    repo.

## Human-in-the-Loop Triggers

Stop and consult a human before:

- **Adding, removing, or renaming a Makefile test target** — CI invokes these by name
  from `openshift/release`, so a rename silently breaks jobs in another repository
- **Changing the `Operators` map** (`pkg/framework/operators.go`) — the catalog source,
  channel, and namespace determine which product build is under test
- **Making a spec disruptive** — anything that drains nodes, deletes machines, or
  changes cluster-scoped config affects every other suite sharing the CI cluster; in
  the CAS module this also means applying the `disruptive` label
- **Changing `.ci-operator.yaml` or the build-root image** — this affects the promoted
  `autoscale-tests` image and the Go toolchain used by CI
- **Adding a new top-level Go module or a new vendor directory** — a structural choice
  with ongoing maintenance cost
- **Deleting or skipping an existing spec** — losing coverage needs a human decision,
  not a fix for a failing test
- **Anything touching release branches** (`release-4.23`, `release-5.0`, `release-5.1`,
  `release-5.2`) — these track OpenShift releases and are cherry-pick targets

## Paired Changes

| If you change... | Also update... |
|-----------------|----------------|
| Add a suite under `test/e2e/` | A `test-e2e-<name>` target in the root `Makefile`, a `README.md` in the suite directory, the suite table in `README.md`, and a Prow job in `openshift/release` |
| A helper in `pkg/framework/` | Run all four root suites (HPA, VPA, CRO, CMA) — they share it |
| Add a typed API group to a framework helper | `newScheme()` in `pkg/framework/framework.go` |
| Add or rename a file in `testdata/` | The `RenderTemplate` call site that names it as a string |
| Add a Ginkgo label | `cas/pkg/framework/ginkgo-labels.go` and the label filter in `cas/Makefile` |
| `cas/go.mod` | `make -C cas vendor`, committing `cas/vendor/` separately from logic changes |
| Root `go.mod` | `go mod tidy`; commit `go.mod` and `go.sum` (the root module is not vendored) |
| Add an operator to install | The `Operators` map and the namespace constants in `pkg/framework/operators.go` |
| A component's test scenarios | The scenario list in that suite's `README.md` |

## Further Reading

- [README.md](README.md) — project overview, how to run the suites, project layout
- [CONTRIBUTING.md](CONTRIBUTING.md) — review policy, PR conventions, Prow commands,
  test expectations
- Per-suite scenario docs: [HPA](test/e2e/hpa/README.md), [VPA](test/e2e/vpa/README.md),
  [CRO](test/e2e/cro/README.md), [CMA](test/e2e/cma/README.md), [CAS](cas/README.md)
- [OpenShift CI documentation](https://docs.ci.openshift.org/)
