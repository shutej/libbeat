package outputs

import (
	"time"

	"github.com/elastic/libbeat/common"
)

type Config struct {
	Enabled           bool
	SaveTopology      bool
	Host              string
	Port              int
	Hosts             []string
	Protocol          string
	Username          string
	Password          string
	Index             string
	Path              string
	Db                int
	DbTopology        int
	Timeout           int
	ReconnectInterval int
	Filename          string
	RotateEveryKb     int
	NumberOfFiles     int
	DataType          string
	FlushInterval     *int
	BulkSize          *int
	MaxRetries        *int
}

// Functions to be exported by a output plugin
type Interface interface {
	// Initialize the output plugin
	Init(config Config, topologyExpire int) error

	// Register the agent name and its IPs to the topology map
	PublishIPs(name string, localAddrs []string) error

	// Get the agent name with a specific IP from the topology map
	GetNameByIP(ip string) string

	// Publish event
	PublishEvent(ts time.Time, event common.MapStr) error
}

// Output identifier
type OutputPlugin uint16

// Output constants
const (
	UnknownOutput OutputPlugin = iota
	RedisOutput
	ElasticsearchOutput
	FileOutput
)

// Output names
var OutputNames = []string{
	"unknown",
	"redis",
	"elasticsearch",
	"file",
}

func (o OutputPlugin) String() string {
	if int(o) >= len(OutputNames) {
		return "impossible"
	}
	return OutputNames[o]
}
