package collectors

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"os"
	"strings"
	"sync"
	"github.com/comprehend-dev/comprehend.dev/agent/models"
	"github.com/go-ini/ini"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"k8s.io/metrics/pkg/client/clientset/versioned"
)

var warnedOnce sync.Map

// warnPermissionOnce logs a warning the first time a forbidden/unauthorized error is seen
// for a given resource. Returns true if the error was a permissions error (caller may skip
// returning the error and continue with empty data instead).
func warnPermissionOnce(resource string, err error) bool {
	if apierrors.IsForbidden(err) || apierrors.IsUnauthorized(err) {
		if _, alreadyLogged := warnedOnce.LoadOrStore(resource, struct{}{}); !alreadyLogged {
			log.Printf("WARNING: k8s %s collection disabled — insufficient RBAC permissions. "+
				"Ensure the agent ServiceAccount has a Role with get/list/watch on %s. Error: %v",
				resource, resource, err)
		}
		return true
	}
	return false
}

type KubernetesCollector struct {
	Collector
	namespace string
	clusterHost string
	clientSet *kubernetes.Clientset
	metricsClientSet *versioned.Clientset
}

/* For kubernetes the agent runs either inside a pod in the cluster, in which
   case it picks up the appropriate environment variables automatically, or
   outside the cluster. In the latter case it can use the kubectl configuration
   files which deal with authentication. */
func (c KubernetesCollector) Initialize(arg string) (Collector, error) {
	delim := strings.LastIndex(arg, " ");
	var config *rest.Config
	var err error
	var namespace string

	if delim > -1 {
		// We got an explicit config file path and namespace separated by space
		var path string
		if arg[0] == '~' {
			path = filepath.Join(homedir.HomeDir(), arg[1:delim])
		} else {
			path = arg[0:delim]
		}
		namespace = arg[delim + 1 : len(arg)]
		config, err = clientcmd.BuildConfigFromFlags("", path)
	} else if os.Getenv("KUBERNETES_SERVICE_HOST") != "" && os.Getenv("KUBERNETES_SERVICE_PORT") != "" {
		// We're running in a pod in the cluster and got a namespace
		namespace = arg
		config, err = rest.InClusterConfig()
	} else {
		// We're outside the cluster and only got a namespace - fall back to default config path
		path := filepath.Join(homedir.HomeDir(), ".kube", "config")
		namespace = arg
		config, err = clientcmd.BuildConfigFromFlags("", path)
	}
	if err != nil {
		return nil, fmt.Errorf("Failed to build the k8s config. Error - %s", err)
	}

	clientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("Failed to create the k8s client set. Error - %s", err)
	}
	metricsClientSet, err := versioned.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("Failed to create the k8s metrics client set. Error - %s", err)
	}

	collector := KubernetesCollector{nil, namespace, config.Host, clientSet, metricsClientSet}
	return collector, nil
}

func (c KubernetesCollector) InitializeFromConfig(section *ini.Section) (Collector, error) {
	var config string = ""
	var namespace string = ""
	if section.HasKey("config") {
		key, _ := section.GetKey("config")
		config = key.String()
	}
	if section.HasKey("namespace") {
		key, _ := section.GetKey("namespace")
		namespace = key.String()
	}
	var arg string
	if config != "" {
		arg = config + " " + namespace
	} else if namespace != "" {
		arg = namespace
	}
	return c.Initialize(arg)
}

func (c KubernetesCollector) InitializeDefault() (Collector, error) {
	return c.Initialize("")
}

func (c KubernetesCollector) URISchema() (string) {
	return "k8s"
}

func (c KubernetesCollector) Description() (string) {
	return "Kubernetes connection string/URI"
}

func (c KubernetesCollector) HostInfo() *HostInfo {
	return nil
}

func (c KubernetesCollector) CollectContainers(containerList []corev1.Container) ([]models.Container, error) {
	containers := make([]models.Container, len(containerList))
	for i, container := range containerList {
		p := make([]models.ContainerPort, len(container.Ports))
		for i, port := range container.Ports {
			protocol, err := models.ParseProtocol(fmt.Sprintf("%s", port.Protocol))
			if err != nil {
				return nil, err
			}
			p[i] = models.ContainerPort{
				ContainerPort: port.ContainerPort,
				HostPort: port.HostPort,
				HostIP: port.HostIP,
				Name: port.Name,
				Protocol: protocol,
			}
		}
		containers[i] = models.Container{
			Name: container.Name,
			Image: container.Image,
			Ports: p,
		}
	}
	return containers, nil
}

func (c KubernetesCollector) Collect() (models.Model, error) {
	nodeMetrics, err := c.metricsClientSet.MetricsV1beta1().NodeMetricses().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		warnPermissionOnce("node metrics (via metrics.k8s.io — requires ClusterRole)", err)
	}
	nodes := make([]models.Node, 0)
	if err == nil {
		nodes = make([]models.Node, len(nodeMetrics.Items))
		for i, node := range nodeMetrics.Items {
			cpuUsage := node.Usage["cpu"]
			memUsage := node.Usage["memory"]
			nodes[i] = models.Node{
				Name: node.Name,
				CurrentCPUUsage: cpuUsage.AsFloat64Slow(),
				CurrentMemoryUsage: memUsage.AsFloat64Slow(),
			}
		}
	}

	pods, err := c.clientSet.CoreV1().Pods(c.namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		if !warnPermissionOnce("pods", err) {
			return nil, err
		}
		pods = &corev1.PodList{}
	}
	p := make([]models.Pod, len(pods.Items))
	for i, pod := range pods.Items {
		containerMetrics := make(map[string]*models.ResourceUsage)
		var podUsage *models.ResourceUsage
		if string(pod.Status.Phase) == "Running" {
			podMetrics, err := c.metricsClientSet.MetricsV1beta1().PodMetricses(c.namespace).Get(context.TODO(), pod.Name, metav1.GetOptions{})
			if err == nil {
				var totalCPU, totalMem float64
				for _, container := range podMetrics.Containers {
					cpuQty := container.Usage["cpu"]
					memQty := container.Usage["memory"]
					cpu := cpuQty.AsFloat64Slow()
					mem := memQty.AsFloat64Slow()
					containerMetrics[container.Name] = &models.ResourceUsage{CPU: cpu, Memory: mem}
					totalCPU += cpu
					totalMem += mem
				}
				podUsage = &models.ResourceUsage{CPU: totalCPU, Memory: totalMem}
			} else {
				warnPermissionOnce("pod metrics (via metrics.k8s.io)", err)
			}
		}

		containerStatuses := make([]models.ContainerStatus, len(pod.Status.ContainerStatuses))
		for j, cs := range pod.Status.ContainerStatuses {
			waitingReason := ""
			if cs.State.Waiting != nil {
				waitingReason = cs.State.Waiting.Reason
			}
			containerStatuses[j] = models.ContainerStatus{
				Name:          cs.Name,
				RestartCount:  cs.RestartCount,
				WaitingReason: waitingReason,
				Metrics:       containerMetrics[cs.Name],
			}
		}
		p[i] = models.Pod{
			UID:              string(pod.UID),
			Namespace:        pod.Namespace,
			Name:             pod.Name,
			Labels:           pod.Labels,
			Phase:            string(pod.Status.Phase),
			Terminating:      pod.DeletionTimestamp != nil,
			Message:          pod.Status.Message,
			Reason:           pod.Status.Reason,
			HostIP:           pod.Status.HostIP,
			NodeName:         pod.Spec.NodeName,
			StartTime:        pod.Status.StartTime.Time,
			Metrics:          podUsage,
			ContainerStatuses: containerStatuses,
		}
	}

	deployments, err := c.clientSet.AppsV1().Deployments(c.namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		if !warnPermissionOnce("deployments", err) {
			return nil, err
		}
		deployments = &appsv1.DeploymentList{}
	}
	//TODO: pagination
	d := make([]models.Deployment, len(deployments.Items))
	for i, deployment := range deployments.Items {
		containers, err := c.CollectContainers(deployment.Spec.Template.Spec.Containers)
		if err != nil {
			return nil, err
		}
		d[i] = models.Deployment{
			Namespace: deployment.Namespace,
			Name: deployment.Name,
			Replicas: *deployment.Spec.Replicas,
			Paused: deployment.Spec.Paused,
			Selector: models.Selector{
				MatchLabels: deployment.Spec.Selector.MatchLabels,
			},
			Template: models.PodTemplate{
				Labels: deployment.Spec.Template.Labels,
				Containers: containers,
			},
		}
	}

	jobs, err := c.clientSet.BatchV1().Jobs(c.namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		if !warnPermissionOnce("jobs", err) {
			return nil, err
		}
		jobs = &batchv1.JobList{}
	}
	j := make([]models.Job, len(jobs.Items))
	for i, job := range jobs.Items {
		containers, err := c.CollectContainers(job.Spec.Template.Spec.Containers)
		if err != nil {
			return nil, err
		}
		j[i] = models.Job{
			Namespace: job.Namespace,
			Name: job.Name,
			Parallelism: *job.Spec.Parallelism,
			Completions: *job.Spec.Completions,
			Suspend: *job.Spec.Suspend,
			Selector: models.Selector{
				MatchLabels: job.Spec.Selector.MatchLabels,
			},
			Template: models.PodTemplate{
				Labels: job.Spec.Template.Labels,
				Containers: containers,
			},
		}
	}

	cronjobs, err := c.clientSet.BatchV1().CronJobs(c.namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		if !warnPermissionOnce("cronjobs", err) {
			return nil, err
		}
		cronjobs = &batchv1.CronJobList{}
	}
	cj := make([]models.CronJob, len(cronjobs.Items))
	for i, cronjob := range cronjobs.Items {
		job := cronjob.Spec.JobTemplate
		containers, err := c.CollectContainers(job.Spec.Template.Spec.Containers)
		if err != nil {
			return nil, err
		}
		cj[i] = models.CronJob{
			Namespace: cronjob.Namespace,
			Name: cronjob.Name,
			Schedule: cronjob.Spec.Schedule,
			TimeZone: *cronjob.Spec.TimeZone,
			Suspend: *cronjob.Spec.Suspend,
			JobTemplate: models.Job{
				Namespace: job.Namespace,
				Name: job.Name,
				Parallelism: *job.Spec.Parallelism,
				Completions: *job.Spec.Completions,
				Suspend: *job.Spec.Suspend,
				Selector: models.Selector{
					MatchLabels: job.Spec.Selector.MatchLabels,
				},
				Template: models.PodTemplate{
					Labels: job.Spec.Template.Labels,
					Containers: containers,
				},
			},
		}
	}

	events, err := c.clientSet.CoreV1().Events(c.namespace).List(context.TODO(), metav1.ListOptions{
		FieldSelector: "type=Warning",
	})
	if err != nil {
		if !warnPermissionOnce("events", err) {
			return nil, err
		}
		events = &corev1.EventList{}
	}
	ev := make([]models.KubernetesEvent, 0, len(events.Items))
	for _, event := range events.Items {
		if event.LastTimestamp.IsZero() {
			continue
		}
		ev = append(ev, models.KubernetesEvent{
			UID:                string(event.UID),
			Namespace:          event.Namespace,
			Reason:             event.Reason,
			Message:            event.Message,
			FirstTimestamp:     event.FirstTimestamp.Time,
			LastTimestamp:      event.LastTimestamp.Time,
			InvolvedObjectKind: event.InvolvedObject.Kind,
			InvolvedObjectName: event.InvolvedObject.Name,
			InvolvedObjectUID:  string(event.InvolvedObject.UID),
		})
	}

	return models.KubernetesModel{
		ClusterHost: c.clusterHost,
		Deployments: d,
		Jobs: j,
		CronJobs: cj,
		Pods: p,
		Nodes: nodes,
		Events: ev,
	}, nil
}

var registeredK8s = RegisterCollector(KubernetesCollector{})
