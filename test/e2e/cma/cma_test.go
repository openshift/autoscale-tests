package cma

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/openshift/autoscale-tests/pkg/framework"
)

var f *framework.Framework

var _ = BeforeSuite(func() {
	var err error
	f, err = framework.NewFramework()
	Expect(err).NotTo(HaveOccurred(), "Failed to create framework")

	By("Checking if CMA operator is installed")
	installed, err := f.IsOperatorSubscribed(f.Ctx, "openshift-custom-metrics-autoscaler-operator", framework.CMANamespace)
	Expect(err).NotTo(HaveOccurred())

	if !installed {
		By("Installing CMA operator from catalog")
		err = f.InstallOperatorByKey(f.Ctx, "cma")
		Expect(err).NotTo(HaveOccurred(), "Failed to install CMA operator")

		By("Waiting for CMA operator CSV to be ready")
		err = f.WaitForOperatorCSVReady(f.Ctx, framework.CMANamespace, 5*time.Minute)
		Expect(err).NotTo(HaveOccurred(), "CMA operator CSV did not become ready")

		By("Waiting for CMA operator pods to be ready")
		err = f.WaitForOperatorReady(f.Ctx, "cma", 3*time.Minute)
		Expect(err).NotTo(HaveOccurred(), "CMA operator pods did not become ready")
	}
})

var _ = AfterSuite(func() {
	if f != nil {
		By("Uninstalling CMA operator")
		err := f.UninstallOperatorByKey(f.Ctx, "cma")
		Expect(err).NotTo(HaveOccurred(), "Failed to uninstall CMA operator")
	}
})

var _ = Describe("Custom Metrics Autoscaler Operator", func() {

	Context("Installation verification", func() {

		It("should have the CMA namespace", func() {
			exists, err := f.NamespaceExists(f.Ctx, framework.CMANamespace)
			Expect(err).NotTo(HaveOccurred())
			Expect(exists).To(BeTrue(), "CMA namespace %s should exist", framework.CMANamespace)
		})

		It("should have running operator pods", func() {
			pods, err := f.GetOperatorPods(f.Ctx, "cma")
			Expect(err).NotTo(HaveOccurred())
			Expect(len(pods.Items)).To(BeNumerically(">", 0),
				"Should have at least one CMA operator pod in namespace %s", framework.CMANamespace)

			By("Listing found pods")
			for _, pod := range pods.Items {
				GinkgoWriter.Printf("  - Pod: %s, Status: %s\n", pod.Name, pod.Status.Phase)
			}
		})

		It("should have all pods in Ready state", func() {
			err := f.CheckOperatorHealth(f.Ctx, "cma")
			Expect(err).NotTo(HaveOccurred(), "All CMA operator pods should be healthy")
		})
	})
})
