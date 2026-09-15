# Contributing to autoscale-tests

This repository holds the end-to-end test suites for the OpenShift autoscaling
components. See [README.md](README.md) for a project overview and for what each suite
covers.

There is no product code here. A change to this repository changes what we assert
about the autoscalers, which makes review a question of "is this test correct, and
will it stay green for the right reasons?" rather than "is this feature correct?".

## Related resources

| Resource | Link |
|----------|------|
| CI configuration | [openshift/release/.../autoscale-tests/](https://github.com/openshift/release/tree/master/ci-operator/config/openshift/autoscale-tests) |
| AI guidance | [AGENTS.md](AGENTS.md) |
| Cluster Autoscaler operator | [openshift/cluster-autoscaler-operator](https://github.com/openshift/cluster-autoscaler-operator) |
| Vertical Pod Autoscaler operator | [openshift/vertical-pod-autoscaler-operator](https://github.com/openshift/vertical-pod-autoscaler-operator) |
| Custom Metrics Autoscaler operator | [openshift/custom-metrics-autoscaler-operator](https://github.com/openshift/custom-metrics-autoscaler-operator) |
| Cluster Resource Override operator | [openshift/cluster-resource-override-admission-operator](https://github.com/openshift/cluster-resource-override-admission-operator) |
| CAS / VPA operand | [openshift/kubernetes-autoscaler](https://github.com/openshift/kubernetes-autoscaler) |
| CMA operand | [openshift/kedacore-keda](https://github.com/openshift/kedacore-keda) |
| CRO operand | [openshift/cluster-resource-override-admission](https://github.com/openshift/cluster-resource-override-admission) |

## Review and Approval Policy

Every change in every pull request must be understood and approved by two humans. This
can be the PR author and a reviewer, or — if the author used an AI tool and does not
fully understand the contents of the PR — two human reviewers.

**Exception:** PRs authored by deterministic automation tools that are part of our CI
and related systems (whose code has been reviewed by the OpenShift engineering org) can
be merged with a single human review.

Every change should be closely scrutinized for bugs. Review changes from multiple
angles:

- **Product architecture**: Does the test assert the behaviour the product actually
  promises, or does it encode an implementation detail that will churn?
- **Security**: Are there new attack surfaces, credential handling issues, or
  privilege escalations? Test code runs with cluster-admin in CI — watch for leaked
  kubeconfigs, secrets in logs, and unnecessary cluster-scoped writes.
- **Thread safety**: Ginkgo specs and the background load generators in
  `pkg/framework/resource_consumer.go` run concurrently. Are shared resources
  synchronized, and is every started goroutine stopped?
- **Regressions**: Could this make an existing suite flaky, slower, or dependent on
  state left behind by another spec?
- **Effects on other components**: Does the change mutate cluster-singleton resources
  (the CRO `ClusterResourceOverride` or the VPA `VerticalPodAutoscalerController`), or
  install/uninstall an operator, in a way that would disturb other suites sharing the
  cluster?

## PR Title Convention

PR titles should be prefixed with a Jira ticket reference:

```
AUTOSCALE-123: Fix the whatsit in the thingamajig
OCPBUGS-456: Correct nil pointer in scaler shutdown
NO-JIRA: Update Go module dependencies
```

## PR Workflow

This repo uses [OpenShift CI (Prow)](https://docs.ci.openshift.org/) for continuous
integration, not GitHub Actions. PRs are automatically merged once all required tests
pass and the correct labels are present.

### Required labels for merge

- `lgtm` — Added by a reviewer via the `/lgtm` command. Any developer from the
  OpenShift org can add this after reviewing the PR.
- `approved` — Added by an approver listed in the [OWNERS](OWNERS) file via the
  `/approve` command.
- `verified` — Added by anyone in the OpenShift org, but typically by the PR author.

### Useful commands

Comment these on the PR:

| Command | Effect |
|---------|--------|
| `/lgtm` | Add the `lgtm` label after reviewing. In repos using [LGTM mode](https://docs.ci.openshift.org/how-tos/creating-a-pipeline/#the-pipeline-required-command), this also triggers E2E and other second-stage tests. |
| `/lgtm cancel` | Remove the `lgtm` label |
| `/approve` | Add the `approved` label (OWNERS approvers only) |
| `/pipeline required` | Manually trigger all required second-stage tests (e.g., E2Es) without waiting for `/lgtm` |
| `/retest` | Re-run all failed required tests |
| `/retest-required` | Re-run only the failed required tests |
| `/test <test-name>` | Run a specific test, e.g. `/test e2e-aws-autoscaler` |
| `/hold` | Add the `do-not-merge/hold` label to prevent merging |
| `/hold cancel` | Remove the hold and allow merging |
| `/verified` | Mark the PR as verified |
| `/cherry-pick <branch>` | Create a cherry-pick PR to a release branch, e.g. `/cherry-pick release-5.1` |

### LGTM mode and E2E tests

Repos enrolled in [LGTM mode](https://docs.ci.openshift.org/how-tos/creating-a-pipeline/#the-pipeline-required-command)
defer second-stage tests (such as E2Es) until the `/lgtm` label is applied. This avoids
wasting CI resources on PRs that haven't been reviewed yet. If you need to run E2Es
before getting `/lgtm` (e.g., to validate before requesting review), use
`/pipeline required`.

### Jobs in this repository

The presubmits are defined in
[openshift/release](https://github.com/openshift/release/tree/master/ci-operator/config/openshift/autoscale-tests).
Today they are:

| Job | Trigger |
|-----|---------|
| `lint-autoscaler` | Runs `make cas-lint` when `cas/` changes |
| `e2e-aws-autoscaler`, `e2e-azure-autoscaler`, `e2e-gcp-autoscaler`, `e2e-vsphere-autoscaler` | `make test-e2e-cas` on the matching cloud |
| `e2e-<cloud>-autoscaler-periodic` | `make test-e2e-cas-periodic`, run on demand |

All of these are currently marked `optional`, and the cloud jobs are gated on changes
under `cas/`. The HPA, VPA, CRO, and CMA suites do not have Prow jobs yet, so nothing
in CI runs them automatically — run them yourself against a cluster and say so with
`/verified by ...`. Adding a job is a separate PR against `openshift/release`.

### Preventing premature merges

- Add the `WIP:` prefix to the PR title (e.g., `WIP: AUTOSCALE-123: Work in progress`).
  Prow adds the `do-not-merge/work-in-progress` label automatically.
- Use `/hold` to temporarily block merging while awaiting additional review or testing.

## Test Expectations

This repository is itself the test suite, so "add a test" usually means "prove the new
or changed helper works against a real cluster":

- **Unit tests**: Required for new logic that can be exercised without a cluster —
  parsing, template rendering, resource arithmetic such as
  `VerifyContainerResources`. Run with `make test-unit`. Note that `make test-unit`
  passes trivially today; a green run does not mean anything was verified.
- **E2E tests**: Expected for every new feature or behaviour change. Every spec needs a
  live cluster. Run the affected suite (`make test-e2e-hpa`, `make test-e2e-cas`, …)
  and record the result on the PR.
- **Blast radius**: A change under `pkg/framework/` is shared by the HPA, VPA, CRO, and
  CMA suites — run all four, not just the one you were working on. A change under
  `cas/pkg/framework/` affects the whole CAS suite.

New specs must clean up after themselves: create a per-test namespace with
`CreateTestNamespace`, delete it in `AfterEach`, and call `CleanUp()` on any
`ResourceConsumer` you create.

## Verified Label

Use `/verified` to indicate changes have been verified. Examples:

```
/verified
/verified by e2e-aws-autoscaler
/verified by unit tests
/verified by E2Es
/verified later -> deferred to QE
```

Because most suites have no CI coverage yet, prefer the explicit form that names what
you ran and on which platform.

## Generated Code

This repository has no generated Go code — no deepcopy functions, CRD manifests,
clientsets, mocks, or protobuf. There is nothing to regenerate before submitting.

Vendored dependencies are the exception, and only in the CAS module:

| Path | Rule |
|------|------|
| `cas/vendor/` | Never hand-edit. Run `make -C cas vendor` (`go mod tidy && go mod vendor && go mod verify`) and commit the vendor changes in a separate commit from logic changes. |
| root module | Not vendored. Run `go mod tidy` and commit `go.mod` and `go.sum`. |

## Code Style and Conventions

- Run `make fmt` (root) before committing; it applies the `gofmt` and `goimports`
  formatters configured in `.golangci.yml`.
- Run `make lint` for the root module and `make cas-lint` for `cas/`.
- Import ordering is enforced by `goimports` with the local prefix
  `github.com/openshift/autoscale-tests`: stdlib, external, then local packages,
  separated by blank lines.
- Error strings are lowercase with no trailing punctuation; wrap with
  `fmt.Errorf("context: %w", err)`.
- Dot-imports are allowed only for `github.com/onsi/ginkgo/v2` and
  `github.com/onsi/gomega` (see the `revive` settings in `.golangci.yml`).
- Prefer `GinkgoWriter.Printf` over `fmt.Printf` in new test and framework code so
  output is attributed to the right spec.
- Use the `wait.PollUntilContextTimeout` helpers already in `pkg/framework/` rather
  than `time.Sleep`, and give every wait an explicit, generous timeout — autoscaling is
  slow and the alternative is a flake.

## Pre-Submit Checklist

Before requesting review:

1. `make lint` and `make cas-lint` — run the linters for both modules
2. `make test-unit` — run unit tests
3. `make check` — lint plus unit tests for the root module
4. Run the affected E2E suite against a real cluster; note the platform and result
5. Review your diff for secrets, kubeconfig paths, credentials, or debug code
6. Address any [CodeRabbit](https://coderabbit.ai/) review feedback — as a courtesy to
   the human reviewer who follows. Responding with an explanation of why you're not
   acting on a suggestion is fine; the goal is to resolve straightforward issues so
   human reviewers can focus on the substantive aspects.

## AI Code Review

This repository uses [CodeRabbit](https://coderabbit.ai/) for automated AI code review,
configured in [.coderabbit.yaml](.coderabbit.yaml). CodeRabbit posts review comments
automatically. You do not need to accept every suggestion; explaining why a comment
does not apply is a perfectly good resolution.
