# Cluster Autoscaler tests

This directory contains the end-to-end tests for the cluster autoscaler.

## Running the tests locally

1. create an openshft cluster and set KUBECONFIG
2. build the tests with `make build-e2e`
3. run the tests with `./bin/autoscale-tests-cas-e2e`
