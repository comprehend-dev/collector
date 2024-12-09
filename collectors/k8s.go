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
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

type KubernetesCollector struct {
	Collector
	namespace string
	clientSet *kubernetes.Clientset
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

	collector := KubernetesCollector{nil, namespace, clientSet}
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

func (c KubernetesCollector) Collect() (models.Model, error) {
	deployments, err := c.clientSet.AppsV1().Deployments(c.namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	//TODO: pagination
	d := make([]models.Deployment, len(deployments.Items))
	for i, deployment := range deployments.Items {
		c := make([]models.Container, len(deployment.Spec.Template.Spec.Containers))
		for i, container := range deployment.Spec.Template.Spec.Containers {
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
			c[i] = models.Container{
				Name: container.Name,
				Image: container.Image,
				Ports: p,
			}
		}

		d[i] = models.Deployment{
			Namespace: deployment.Namespace,
			Name: deployment.Name,
			Replicas: *deployment.Spec.Replicas,
			Selector: models.Selector{
				MatchLabels: deployment.Spec.Selector.MatchLabels,
			},
			Template: models.Template{
				Labels: deployment.Spec.Template.Labels,
				Containers: c,
			},
		}
	}

	return models.KubernetesModel{Deployments: d}, nil
}

var registeredK8s = RegisterCollector(KubernetesCollector{})
