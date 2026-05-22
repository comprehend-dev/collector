package collectors

import (
	"context"
	"fmt"
	"path/filepath"
	"os"
	"strings"
	"github.com/comprehend-dev/comprehend.dev/agent/models"
	"github.com/go-ini/ini"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"k8s.io/metrics/pkg/client/clientset/versioned"
)

type KubernetesCollector struct {
	Collector
	namespace string
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

	collector := KubernetesCollector{nil, namespace, clientSet, metricsClientSet}
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
	nodes := make([]models.Node, len(nodeMetrics.Items))
	if err == nil {
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
		return nil, err
	}
	p := make([]models.Pod, len(pods.Items))
	for i, pod := range pods.Items {
		var cpu = 0.0
		var mem = 0.0 // Actually should be int, but Javascript only has floats anyway and this avoids having to deal with big ints
		if string(pod.Status.Phase) == "Running" {
			podMetrics, err := c.metricsClientSet.MetricsV1beta1().PodMetricses(c.namespace).Get(context.TODO(), pod.Name, metav1.GetOptions{})
			if err == nil {
				for _, container := range podMetrics.Containers {
					cpuUsage := container.Usage["cpu"]
					cpu += cpuUsage.AsFloat64Slow()

					memUsage := container.Usage["memory"]
					mem += memUsage.AsFloat64Slow()
				}
			}
		}

		p[i] = models.Pod{
			Namespace: pod.Namespace,
			Name: pod.Name,
			Labels: pod.Labels,
			Phase: string(pod.Status.Phase),
			Message: pod.Status.Message,
			Reason: pod.Status.Reason,
			HostIP: pod.Status.HostIP,
			NodeName: pod.Spec.NodeName,
			StartTime: pod.Status.StartTime.Time,
			CurrentCPUUsage: cpu,
			CurrentMemoryUsage: mem,
		}
	}

	deployments, err := c.clientSet.AppsV1().Deployments(c.namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
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
		return nil, err
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
		return nil, err
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

	return models.KubernetesModel{
		Deployments: d,
		Jobs: j,
		CronJobs: cj,
		Pods: p,
		Nodes: nodes,
	}, nil
}

var registeredK8s = RegisterCollector(KubernetesCollector{})
