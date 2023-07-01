package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"
	policy "k8s.io/api/policy/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	_ "k8s.io/klog"
	metrics "k8s.io/metrics/pkg/client/clientset/versioned"
)

const (
	EvictionKind                   = "Eviction"
	PodLabelSelector               = "pre-oom-killer=enabled"
	TargetContainerName            = "pre-oom-killer.v1alpha1.k8s.io/target-container-name"
	MemoryUsageThresholdAnnotation = "pre-oom-killer.v1alpha1.k8s.io/memory-usage-threshold" // メモリ使用率のしきい値（1 ~ 100）
)

type Controller struct {
	context          context.Context
	clientset        kubernetes.Interface
	metricsClientset *metrics.Clientset
	interval         time.Duration
}

func NewController(context context.Context, clientset kubernetes.Interface, metricsClientset *metrics.Clientset, interval time.Duration) *Controller {
	return &Controller{
		context:          context,
		clientset:        clientset,
		metricsClientset: metricsClientset,
		interval:         interval,
	}
}

func evictPod(ctx context.Context, client kubernetes.Interface, podName, podNamespace, policyGroupVersion string, dryRun bool) (bool, error) {
	if dryRun {
		return true, nil
	}

	deleteOptions := &metav1.DeleteOptions{}
	eviction := &policy.Eviction{
		TypeMeta: metav1.TypeMeta{
			APIVersion: policyGroupVersion,
			Kind:       EvictionKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: podNamespace,
		},
		DeleteOptions: deleteOptions,
	}
	err := client.PolicyV1beta1().Evictions(eviction.Namespace).Evict(ctx, eviction)

	if err == nil {
		return true, nil
	} else if apierrors.IsTooManyRequests(err) {
		return false, fmt.Errorf("error when evicting pod (ignoring) %q: %v", podName, err)
	} else if apierrors.IsNotFound(err) {
		return true, fmt.Errorf("pod not found when evicting %q: %v", podName, err)
	} else {
		return false, err
	}
}

func (c *Controller) Evict() error {
	evictionCount := 0

	// Pod一覧の取得
	podList, err := c.clientset.CoreV1().Pods("").List(
		c.context,
		metav1.ListOptions{
			LabelSelector: PodLabelSelector,
		},
	)
	if err != nil {
		log.Errorf("PodListError for label selector %s: %s", PodLabelSelector, err)
		return err
	}

	for _, pod := range podList.Items {
		podName, podNamespace := pod.ObjectMeta.Name, pod.ObjectMeta.Namespace
		podTargetContainer, ok := pod.ObjectMeta.Annotations[TargetContainerName]
		if !ok {
			log.WithFields(log.Fields{
				"pod":       podName,
				"namespace": podNamespace,
			}).Errorf("PodTargetContainerNameFetchError: %s", err)
			continue
		}

		podMemoryUsageThreshold, err := resource.ParseQuantity(pod.ObjectMeta.Annotations[MemoryUsageThresholdAnnotation])
		if err != nil {
			log.WithFields(log.Fields{
				"pod":       podName,
				"namespace": podNamespace,
			}).Errorf("PodMemoryUsageThresholdFetchError: %s", err)
			continue
		}

		// 指定したContainerのlimits.memoryを取得する
		var containerLimitsMemory *resource.Quantity
		for _, container := range pod.Spec.Containers {
			if container.Name != podTargetContainer {
				continue
			}

			// Memo: resources.limits.memoryを取得しようと思うとPodではなくPod内のContainer単位で取得しなければいけない
			containerLimitsMemory = container.Resources.Limits.Memory()
		}
		if containerLimitsMemory == nil {
			continue
		}

		// 指定したContainerのmemory.usageを取得する
		containerMemoryUsage := &resource.Quantity{}
		podMetrics, err := c.metricsClientset.MetricsV1beta1().PodMetricses(podNamespace).Get(c.context, podName, metav1.GetOptions{})
		if err != nil {
			log.WithFields(log.Fields{
				"pod":                  podName,
				"namespace":            podNamespace,
				"targetContainer":      podTargetContainer,
				"memoryUsageThreshold": podMemoryUsageThreshold.String(),
			}).Errorf("PodMetricsFetchError: %s", err)
			return err
		}
		for _, containerMetrics := range podMetrics.Containers {
			if containerMetrics.Name != podTargetContainer {
				continue
			}

			containerMemoryUsage = containerMetrics.Usage.Memory()
		}

		// メモリ使用率がしきい値を超えているかどうか
		containerMemoryUsagePercentage := (float64(containerMemoryUsage.Value()) / float64(containerLimitsMemory.Value())) * 100
		if containerMemoryUsagePercentage > float64(podMemoryUsageThreshold.Value()) {
			_, err := evictPod(c.context, c.clientset, podName, podNamespace, "v1", false)
			if err != nil {
				log.WithFields(log.Fields{
					"pod":                  podName,
					"namespace":            podNamespace,
					"targetContainer":      podTargetContainer,
					"memoryUsageThreshold": podMemoryUsageThreshold.String(),
				}).Errorf("PodEvictionError: %v", err)
			} else {
				evictionCount += 1
				log.WithFields(log.Fields{
					"pod":                  podName,
					"namespace":            podNamespace,
					"targetContainer":      podTargetContainer,
					"memoryUsageThreshold": podMemoryUsageThreshold.String(),
				}).Infof("PodEvicted with container memory usage percentage: %v", containerMemoryUsagePercentage)
			}
		}
	}

	log.Infof("%d pods evicted during this run", evictionCount)
	return nil
}

func (c *Controller) Run() {
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	for {
		err := c.Evict()
		if err != nil {
			log.Error(err)
		}
		select {
		case <-ticker.C:
		case <-c.context.Done():
			log.Info("Stop loop")
			return
		}
	}
}

func main() {
	var kubeconfig string
	var masterUrl string
	var logLevel string
	var logFormat string
	var interval int

	flag.StringVar(&kubeconfig, "kubeconfig", "", "absolute path to the kubeconfig file")
	flag.StringVar(&masterUrl, "masterUrl", "", "masterUrl url")
	flag.IntVar(&interval, "interval", 300, "Interval (in seconds)")
	flag.StringVar(&logLevel, "logLevel", "info", "Log level, one of debug, info, warn, error")
	flag.StringVar(&logFormat, "logFormat", "text", "Log format, one of json, text")
	flag.Parse()

	log.SetOutput(os.Stdout)

	switch logFormat {
	case "json":
		log.SetFormatter(&log.JSONFormatter{})
	case "text":
	default:
		log.SetFormatter(&log.TextFormatter{})

	}

	switch logLevel {
	case "debug":
		log.SetLevel(log.DebugLevel)
	case "warn":
		log.SetLevel(log.WarnLevel)
	case "error":
		log.SetLevel(log.ErrorLevel)
	case "info":
	default:
		log.SetLevel(log.InfoLevel)
	}

	config, err := clientcmd.BuildConfigFromFlags(masterUrl, kubeconfig)
	if err != nil {
		log.Fatal(err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatal(err)
	}

	metricsClientset, err := metrics.NewForConfig(config)
	if err != nil {
		log.Fatal(err)
	}

	ctx := SetupSignalHandler()
	controller := NewController(ctx, clientset, metricsClientset, time.Duration(interval)*time.Second)
	controller.Run()
}

var onlyOneSignalHandler = make(chan struct{})
var shutdownSignals = []os.Signal{os.Interrupt, syscall.SIGTERM}

// SetupSignalHandler : SIGTERMとSIGINTをハンドリングする
// どちらかのシグナルが発生するとキャンセルされるコンテキストが返される
// 2回目のシグナルがキャッチされた場合、プログラムは終了コード1で終了する（ = force shutdown)
func SetupSignalHandler() context.Context {
	// 多重呼び出しの防止
	close(onlyOneSignalHandler)

	c := make(chan os.Signal, 2)
	ctx, cancel := context.WithCancel(context.Background())
	signal.Notify(c, shutdownSignals...)
	go func() {
		<-c
		cancel()
		<-c
		os.Exit(1)
	}()

	return ctx
}
