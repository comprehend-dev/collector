package collectors

import (
	"github.com/go-ini/ini"
	"github.com/comprehend-dev/comprehend.dev/agent/models"
)

type Collector interface {
	URISchema() (string)
	Description() (string)
	Initialize(arg string) (Collector, error)
	InitializeFromConfig(section *ini.Section) (Collector, error)
	InitializeDefault() (Collector, error)
	Collect() (models.Model, error)
}

var Collectors map[string]Collector = make(map[string]Collector);

type CollectorRegistration struct {
}

func RegisterCollector(collector Collector) (CollectorRegistration) {
	name := collector.URISchema()
	Collectors[name] = collector
	return CollectorRegistration{}
}
