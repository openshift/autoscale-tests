package vpa

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

	By("Checking if VPA operator is installed")
	installed, err := f.IsOperatorSubscribed(f.Ctx, "vertical-pod-autoscaler", framework.VPANamespace)
	Expect(err).NotTo(HaveOccurred())

	if !installed {
		By("Installing VPA operator from catalog")
		err = f.InstallOperatorByKey(f.Ctx, "vpa")
		Expect(err).NotTo(HaveOccurred(), "Failed to install VPA operator")

		By("Waiting for VPA operator CSV to be ready")
		err = f.WaitForOperatorCSVReady(f.Ctx, framework.VPANamespace, 5*time.Minute)
		Expect(err).NotTo(HaveOccurred(), "VPA operator CSV did not become ready")

		By("Waiting for VPA operator pods to be ready")
		err = f.WaitForOperatorReady(f.Ctx, "vpa", 3*time.Minute)
		Expect(err).NotTo(HaveOccurred(), "VPA operator pods did not become ready")
	}
})

var _ = AfterSuite(func() {
	if f != nil {
		By("Uninstalling VPA operator")
		err := f.UninstallOperatorByKey(f.Ctx, "vpa")
		Expect(err).NotTo(HaveOccurred(), "Failed to uninstall VPA operator")
	}
})

var _ = Describe("VPA Operator", func() {

	Context("Installation verification", func() {

		It("should have the VPA namespace", func() {
			exists, err := f.NamespaceExists(f.Ctx, framework.VPANamespace)
			Expect(err).NotTo(HaveOccurred())
			Expect(exists).To(BeTrue(), "VPA namespace %s should exist", framework.VPANamespace)
		})

		It("should have running operator pods", func() {
			pods, err := f.GetOperatorPods(f.Ctx, "vpa")
			Expect(err).NotTo(HaveOccurred())
			Expect(len(pods.Items)).To(BeNumerically(">", 0),
				"Should have at least one VPA operator pod in namespace %s", framework.VPANamespace)

			By("Listing found pods")
			for _, pod := range pods.Items {
				GinkgoWriter.Printf("  - Pod: %s, Status: %s\n", pod.Name, pod.Status.Phase)
			}
		})

		It("should have all pods in Ready state", func() {
			err := f.CheckOperatorHealth(f.Ctx, "vpa")
			Expect(err).NotTo(HaveOccurred(), "All VPA operator pods should be healthy")
		})
	})
})
