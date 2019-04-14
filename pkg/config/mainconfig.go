package config

import (
	"time"
)

// GeneralConfig has miscellaneous configuration options
type GeneralConfig struct {
	InstanceID           string        `mapstructure:"instanceID"`
	LogDir               string        `mapstructure:"logdir"`
	HomeDir              string        `mapstructure:"homedir"`
	DataDir              string        `mapstructure:"datadir"`
	LogLevel             string        `mapstructure:"loglevel"`
	SyncMode             string        `mapstructure:"sync-mode"`
	CheckInterval        time.Duration `mapstructure:"check-interval"`
	MinSyncInterval      time.Duration `mapstructure:"min-sync-interval"`
	MasterDB             string        `mapstructure:"master-db"`
	SlaveDB              string        `mapstructure:"slave-db"`
	InitialReplication   string        `mapstructure:"initial-replication"`
	MonitorRetryInterval time.Duration `mapstructure:"monitor-retry-interval"`
	DataChunkDuration    time.Duration `mapstructure:"data-chuck-duration"`
	MaxRetentionInterval time.Duration `mapstructure:"max-retention-interval"`
	RWMaxRetries         int           `mapstructure:"rw-max-retries"`
	RWRetryDelay         time.Duration `mapstructure:"rw-retry-delay"`
	NumWorkers           int           `mapstructure:"num-workers"`
}

//SelfMonConfig configuration for self monitoring
/*type SelfMonConfig struct {
	Enabled           bool     `mapstructure:"enabled"`
	Freq              int      `mapstructure:"freq"`
	Prefix            string   `mapstructure:"prefix"`
	InheritDeviceTags bool     `mapstructure:"inheritdevicetags"`
	ExtraTags         []string `mapstructure:"extra-tags"`
}*/

//HTTPConfig has webserver config options
type HTTPConfig struct {
	BindAddr      string `mapstructure:"bind-addr"`
	AdminUser     string `mapstructure:"admin-user"`
	AdminPassword string `mapstructure:"admin-passwd"`
	CookieID      string `mapstructure:"cookie-id"`
}

type InfluxDB struct {
	Release     string        `mapstructure:"release"`
	Name        string        `mapstructure:"name"`
	Location    string        `mapstructure:"location"`
	AdminUser   string        `mapstructure:"admin-user"`
	AdminPasswd string        `mapstructure:"admin-passwd"`
	Timeout     time.Duration `mapstructure:"timeout"`
}

//Config Main Configuration struct
type Config struct {
	General GeneralConfig
	//Database DatabaseCfg
	//Selfmon  SelfMonConfig
	HTTP        HTTPConfig
	InfluxArray []*InfluxDB `mapstructure:"influxdb"`
}

//var MainConfig Config
