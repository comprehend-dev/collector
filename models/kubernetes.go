package models

import (
	"encoding/json"
	"fmt"
)

type KubernetesModel struct {
	Deployments []Deployment `json:"deployments"`
	Jobs []Job `json:"jobs"`
	CronJobs []CronJob `json:"cronjobs"`
}

func (m KubernetesModel) ToJSON() ([]byte, error) {
	return json.Marshal(m)
}

type Deployment struct {
	Namespace	string		`json:"namespace"`
	Name		string		`json:"name"`
	Replicas	int32		`json:"replicas"`
	Paused		bool		`json:"paused"`
	Selector	Selector 	`json:"selector"`
	Template	PodTemplate	`json:"template"`
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
