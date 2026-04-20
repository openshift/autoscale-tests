package hpa

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	autoscalingv2 "k8s.io/api/autoscaling/v2"

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

	Context("HPA Functionality - Memory-based scaling", func() {
		var testNamespace string

		BeforeEach(func() {
			var err error
			testNamespace, err = f.CreateTestNamespace(f.Ctx, "hpa-memory-test")
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			if testNamespace != "" {
				err := f.CleanupTestNamespace(f.Ctx, testNamespace)
				Expect(err).NotTo(HaveOccurred())
			}
		})

		It("should create memory-based HPA from template", func() {
			By("Creating test deployment")
			deployCfg := framework.DefaultDeploymentConfig("hpa-test-app", testNamespace)
			GinkgoWriter.Printf("[Test] Creating deployment %q in namespace %q\n", deployCfg.Name, testNamespace)
			_, err := f.CreateDeployment(f.Ctx, deployCfg)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for deployment to be ready")
			GinkgoWriter.Printf("[Test] Waiting for deployment to be ready (timeout: 3m)...\n")
			err = f.WaitForDeploymentReady(f.Ctx, deployCfg.Name, testNamespace, 3*time.Minute)
			Expect(err).NotTo(HaveOccurred())
			GinkgoWriter.Printf("[Test] Deployment %q is ready\n", deployCfg.Name)

			By("Rendering HPA template")
			hpaYAML, err := framework.RenderHPATemplate(framework.HPATemplateConfig{
				ResourceName:               "hpa-memory",
				Namespace:                  testNamespace,
				DeploymentName:             deployCfg.Name,
				MinReplicas:                1,
				MaxReplicas:                5,
				Metrics:                    []framework.HPAMetric{{Name: "memory", AverageValue: "40Mi"}},
				StabilizationWindowSeconds: 20,
			})
			Expect(err).NotTo(HaveOccurred())
			GinkgoWriter.Printf("[Test] Rendered HPA template:\n%s\n", hpaYAML)

			By("Creating HPA from framework")
			GinkgoWriter.Printf("[Test] Creating memory-based HPA: minReplicas=1, maxReplicas=5, memoryTarget=40Mi\n")
			hpaCfg := framework.HPAConfig{
				Name:               "hpa-memory",
				Namespace:          testNamespace,
				TargetDeployment:   deployCfg.Name,
				MinReplicas:        1,
				MaxReplicas:        5,
				MemoryAverageValue: "40Mi",
			}
			hpa, err := f.CreateHPA(f.Ctx, hpaCfg)
			Expect(err).NotTo(HaveOccurred())

			By("Verifying HPA was created")
			fetchedHPA, err := f.GetHPA(f.Ctx, hpa.Name, testNamespace)
			Expect(err).NotTo(HaveOccurred())
			Expect(*fetchedHPA.Spec.MinReplicas).To(Equal(int32(1)))
			Expect(fetchedHPA.Spec.MaxReplicas).To(Equal(int32(5)))
			GinkgoWriter.Printf("[Test] HPA %q created successfully: MinReplicas=%d, MaxReplicas=%d, Metrics=%d\n",
				fetchedHPA.Name,
				*fetchedHPA.Spec.MinReplicas,
				fetchedHPA.Spec.MaxReplicas,
				len(fetchedHPA.Spec.Metrics),
			)
		})
	})

	Context("HPA Functionality - CPU-based scaling", func() {
		var testNamespace string

		BeforeEach(func() {
			var err error
			testNamespace, err = f.CreateTestNamespace(f.Ctx, "hpa-cpu-test")
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			if testNamespace != "" {
				err := f.CleanupTestNamespace(f.Ctx, testNamespace)
				Expect(err).NotTo(HaveOccurred())
			}
		})

		It("should create CPU-based HPA from template", func() {
			By("Creating test deployment")
			deployCfg := framework.DefaultDeploymentConfig("hpa-cpu-app", testNamespace)
			deployCfg.CPURequest = "100m"
			GinkgoWriter.Printf("[Test] Creating deployment %q in namespace %q (cpuRequest: %s)\n",
				deployCfg.Name, testNamespace, deployCfg.CPURequest)
			_, err := f.CreateDeployment(f.Ctx, deployCfg)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for deployment to be ready")
			GinkgoWriter.Printf("[Test] Waiting for deployment to be ready (timeout: 3m)...\n")
			err = f.WaitForDeploymentReady(f.Ctx, deployCfg.Name, testNamespace, 3*time.Minute)
			Expect(err).NotTo(HaveOccurred())
			GinkgoWriter.Printf("[Test] Deployment %q is ready\n", deployCfg.Name)

			By("Rendering HPA template")
			hpaYAML, err := framework.RenderHPATemplate(framework.HPATemplateConfig{
				ResourceName:   "hpa-cpu",
				Namespace:      testNamespace,
				DeploymentName: deployCfg.Name,
				MinReplicas:    1,
				MaxReplicas:    5,
				Metrics:        []framework.HPAMetric{{Name: "cpu", AverageValue: "10m"}},
			})
			Expect(err).NotTo(HaveOccurred())
			GinkgoWriter.Printf("[Test] Rendered HPA template:\n%s\n", hpaYAML)

			By("Creating CPU-based HPA")
			cpuTarget := int32(50)
			GinkgoWriter.Printf("[Test] Creating CPU-based HPA: minReplicas=1, maxReplicas=5, cpuTarget=%d%%\n", cpuTarget)
			hpaCfg := framework.HPAConfig{
				Name:                 "hpa-cpu",
				Namespace:            testNamespace,
				TargetDeployment:     deployCfg.Name,
				MinReplicas:          1,
				MaxReplicas:          5,
				CPUTargetUtilization: &cpuTarget,
			}
			hpa, err := f.CreateHPA(f.Ctx, hpaCfg)
			Expect(err).NotTo(HaveOccurred())

			By("Verifying HPA was created")
			var fetchedHPA autoscalingv2.HorizontalPodAutoscaler
			err = f.Client.Get(f.Ctx, framework.ObjectKey{Name: hpa.Name, Namespace: testNamespace}, &fetchedHPA)
			Expect(err).NotTo(HaveOccurred())
			Expect(*fetchedHPA.Spec.MinReplicas).To(Equal(int32(1)))
			Expect(fetchedHPA.Spec.MaxReplicas).To(Equal(int32(5)))
			GinkgoWriter.Printf("[Test] HPA %q created successfully: MinReplicas=%d, MaxReplicas=%d, Metrics=%d\n",
				fetchedHPA.Name,
				*fetchedHPA.Spec.MinReplicas,
				fetchedHPA.Spec.MaxReplicas,
				len(fetchedHPA.Spec.Metrics),
			)
		})
	})
})
