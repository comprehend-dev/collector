package models

import (
	"encoding/json"
	"fmt"
	"time"
)

type KubernetesModel struct {
	ClusterHost string `json:"clusterHost"`
	Deployments []Deployment `json:"deployments"`
	Jobs []Job `json:"jobs"`
	CronJobs []CronJob `json:"cronjobs"`
	Pods []Pod `json:"pods"`
	Nodes []Node `json:"nodes"`
}

func (m KubernetesModel) ToJSON() ([]byte, error) {
	return json.Marshal(m)
}

type Deployment struct {
	Namespace		string		`json:"namespace"`
	Name			string		`json:"name"`
	Replicas		int32		`json:"replicas"`
	Paused			bool		`json:"paused"`
	ExistingReplicas	int32		`json:"existingreplicas"`
	AvailableReplicas	int32		`json:"availablereplicas"`
	Selector		Selector 	`json:"selector"`
	Template		PodTemplate	`json:"template"`
}

type Job struct {
	Namespace	string		`json:"namespace"`
	Name		string		`json:"name"`
	Parallelism	int32		`json:"parallelism"`
	Completions	int32		`json:"completions"`
	Suspend		bool		`json:"suspend"`
	Selector	Selector 	`json:"selector"`
	Template	PodTemplate	`json:"template"`
}

type CronJob struct {
	Namespace	string		`json:"namespace"`
	Name		string		`json:"name"`
	Schedule	string		`json:"schedule"`
	TimeZone	string		`json:"timezone"`
	Suspend		bool		`json:"suspend"`
	JobTemplate	Job		`json:"jobtemplate"`
}

type Selector struct {
	MatchLabels	map[string]string	`json:"matchLabels"`
}

type PodTemplate struct {
	Labels		map[string]string	`json:"labels"`
	Containers	[]Container		`json:"containers"`
}

type Pod struct {
	Namespace		string			`json:"namespace"`
	Name			string			`json:"name"`
	Labels			map[string]string	`json:"labels"`
	Phase			string			`json:"phase"`
	Message			string			`json:"message"`
	Reason			string			`json:"reason"`
	HostIP			string			`json:"hostip"`
	NodeName		string			`json:"nodeName"`
	StartTime		time.Time		`json:"startTime"`
	CurrentCPUUsage		float64			`json:"currentCPUUsage"`
	CurrentMemoryUsage	float64			`json:"currentMemoryUsage"`
}

type Container struct {
	Name	string		`json:"name"`
	Image	string		`json:"image"`
	Ports	[]ContainerPort	`json:"ports"`
}

type ContainerPort struct {
	ContainerPort	int32		`json:"containerPort"`
	HostPort	int32		`json:"hostPort"`
	HostIP		string		`json:"hostIP"`
	Name		string		`json:"name"`
	Protocol	Protocol	`json:"protocol"`
}

type Node struct {
	Name 			string	`json:"name"`
	CurrentCPUUsage		float64	`json:"currentCPUUsage"`
	CurrentMemoryUsage 	float64	`json:"currentMemoryUsage"`
}

type Protocol string

const (
	UDP Protocol  = "UDP"
	TCP Protocol  = "TCP"
	SCTP Protocol = "SCTP"
)

func ParseProtocol(p string)(Protocol, error) {
	if p == "UDP" {
		return UDP, nil
	} else if p == "TCP" {
		return TCP, nil
	} else if p == "SCTP" {
		return SCTP, nil
	}
	return TCP, fmt.Errorf("Unknown protocol %s", p)
}
