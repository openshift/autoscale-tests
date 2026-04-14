package framework

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Framework struct {
	Client     client.Client
	Clientset  *kubernetes.Clientset
	RestConfig *rest.Config
	Ctx        context.Context
	Namespace  string
}

func NewFramework() (*Framework, error) {
	config, err := getConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get kubeconfig: %w", err)
	}

	scheme := newScheme()
	c, err := client.New(config, client.Options{Scheme: scheme})
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create clientset: %w", err)
	}

	return &Framework{
		Client:     c,
		Clientset:  clientset,
		RestConfig: config,
		Ctx:        context.Background(),
		Namespace:  getNamespace(),
	}, nil
}

func newScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(appsv1.AddToScheme(scheme))
	utilruntime.Must(autoscalingv1.AddToScheme(scheme))
	utilruntime.Must(autoscalingv2.AddToScheme(scheme))
	return scheme
}

func getConfig() (*rest.Config, error) {
	config, err := rest.InClusterConfig()
	if err == nil {
		return config, nil
	}

	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		kubeconfig = os.Getenv("HOME") + "/.kube/config"
	}

	return clientcmd.BuildConfigFromFlags("", kubeconfig)
}

func getNamespace() string {
	if ns := os.Getenv("TEST_NAMESPACE"); ns != "" {
		return ns
	}
	return "default"
}

func (f *Framework) WithTimeout(timeout time.Duration) context.Context {
	ctx, _ := context.WithTimeout(f.Ctx, timeout)
	return ctx
}

// ==================== Pod Operations ====================

func (f *Framework) GetPod(ctx context.Context, name, namespace string) (*corev1.Pod, error) {
	pod := &corev1.Pod{}
	err := f.Client.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, pod)
	return pod, err
}

func (f *Framework) ListPods(ctx context.Context, namespace string, labelSelector map[string]string) (*corev1.PodList, error) {
	podList := &corev1.PodList{}
	opts := &client.ListOptions{
		Namespace:     namespace,
		LabelSelector: labels.SelectorFromSet(labelSelector),
	}
	err := f.Client.List(ctx, podList, opts)
	return podList, err
}

func (f *Framework) WaitForPodReady(ctx context.Context, name, namespace string, timeout time.Duration) error {
	return wait.PollUntilContextTimeout(ctx, 2*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		pod, err := f.GetPod(ctx, name, namespace)
		if err != nil {
			return false, nil
		}
		return isPodReady(pod), nil
	})
}

func (f *Framework) WaitForPodsWithLabel(ctx context.Context, namespace string, labelSelector map[string]string, minReady int, timeout time.Duration) error {
	return wait.PollUntilContextTimeout(ctx, 2*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		pods, err := f.ListPods(ctx, namespace, labelSelector)
		if err != nil {
			return false, nil
		}

		readyCount := 0
		for _, pod := range pods.Items {
			if isPodReady(&pod) {
				readyCount++
			}
		}
		return readyCount >= minReady, nil
	})
}

func (f *Framework) ExecInPod(ctx context.Context, namespace, podName, containerName string, command []string) (string, string, error) {
	req := f.Clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: containerName,
			Command:   command,
			Stdin:     false,
			Stdout:    true,
			Stderr:    true,
			TTY:       false,
		}, clientgoscheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(f.RestConfig, "POST", req.URL())
	if err != nil {
		return "", "", fmt.Errorf("failed to create executor: %w", err)
	}

	var stdout, stderr bytes.Buffer
	err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdout: &stdout,
		Stderr: &stderr,
	})

	return stdout.String(), stderr.String(), err
}

func (f *Framework) GetPodLogs(ctx context.Context, namespace, podName, containerName string, tailLines int64) (string, error) {
	opts := &corev1.PodLogOptions{
		Container: containerName,
		TailLines: &tailLines,
	}

	req := f.Clientset.CoreV1().Pods(namespace).GetLogs(podName, opts)
	stream, err := req.Stream(ctx)
	if err != nil {
		return "", err
	}
	defer stream.Close()

	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, stream)
	return buf.String(), err
}

func (f *Framework) DeletePod(ctx context.Context, name, namespace string) error {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	return f.Client.Delete(ctx, pod)
}

func isPodReady(pod *corev1.Pod) bool {
	if pod.Status.Phase != corev1.PodRunning {
		return false
	}
	for _, cond := range pod.Status.Conditions {
		if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

// ==================== Namespace Operations ====================

func (f *Framework) NamespaceExists(ctx context.Context, name string) (bool, error) {
	ns := &corev1.Namespace{}
	err := f.Client.Get(ctx, client.ObjectKey{Name: name}, ns)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (f *Framework) CreateNamespace(ctx context.Context, name string) (*corev1.Namespace, error) {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"test-suite": "autoscale-tests",
			},
		},
	}

	err := f.Client.Create(ctx, ns)
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			return ns, nil
		}
		return nil, fmt.Errorf("failed to create namespace: %w", err)
	}

	return ns, nil
}

func (f *Framework) DeleteNamespace(ctx context.Context, name string) error {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	return client.IgnoreNotFound(f.Client.Delete(ctx, ns))
}
