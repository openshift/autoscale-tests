package hpa

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/openshift/autoscale-tests/pkg/framework"
)

var f *framework.Framework

var _ = BeforeSuite(func() {
	var err error
	f, err = framework.NewFramework()
	Expect(err).NotTo(HaveOccurred(), "Failed to create framework")
})

var _ = Describe("HPA / Cluster Autoscaler Operator", func() {

	Context("Installation verification", func() {

		It("should have the operator namespace", func() {
			exists, err := f.NamespaceExists(f.Ctx, framework.HPANamespace)
			Expect(err).NotTo(HaveOccurred())
			Expect(exists).To(BeTrue(), "HPA namespace %s should exist", framework.HPANamespace)
		})

		It("should have running operator pods", func() {
			pods, err := f.GetOperatorPods(f.Ctx, "hpa")
			Expect(err).NotTo(HaveOccurred())
			Expect(len(pods.Items)).To(BeNumerically(">", 0), 
				"Should have at least one HPA operator pod in namespace %s", framework.HPANamespace)
			
			By("Listing found pods")
			for _, pod := range pods.Items {
				GinkgoWriter.Printf("  - Pod: %s, Status: %s\n", pod.Name, pod.Status.Phase)
			}
		})

		It("should have all pods in Ready state", func() {
			err := f.CheckOperatorHealth(f.Ctx, "hpa")
			Expect(err).NotTo(HaveOccurred(), "All HPA operator pods should be healthy")
		})
	})
})
