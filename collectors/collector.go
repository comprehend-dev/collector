package collectors

import (
	"github.com/go-ini/ini"
	"github.com/comprehend-dev/collector/models"
)

type HostInfo struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

type Collector interface {
	URISchema() (string)
	Description() (string)
	Initialize(arg string) (Collector, error)
	InitializeFromConfig(section *ini.Section) (Collector, error)
	InitializeDefault() (Collector, error)
	Collect() (models.Model, error)
	HostInfo() *HostInfo
}

var Collectors map[string]Collector = make(map[string]Collector);

type CollectorRegistration struct {
}

func RegisterCollector(collector Collector) (CollectorRegistration) {
	name := collector.URISchema()
	Collectors[name] = collector
	return CollectorRegistration{}
}
