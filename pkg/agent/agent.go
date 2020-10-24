package agent

import (
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/toni-moreno/syncflux/pkg/config"
)

var (
	// Version is the app X.Y.Z version
	Version string
	// Commit is the git commit sha1
	Commit string
	// Branch is the git branch
	Branch string
	// BuildStamp is the build timestamp
	BuildStamp string
)

// RInfo contains the agent's release and version information.
type RInfo struct {
	InstanceID string
	Version    string
	Commit     string
	Branch     string
	BuildStamp string
}

// GetRInfo returns the agent release information.
func GetRInfo() *RInfo {
	info := &RInfo{
		InstanceID: MainConfig.General.InstanceID,
		Version:    Version,
		Commit:     Commit,
		Branch:     Branch,
		BuildStamp: BuildStamp,
	}
	return info
}

var (

	// MainConfig contains the global configuration
	MainConfig config.Config

	log *logrus.Logger
	// reloadMutex guards the reloadProcess flag
	reloadMutex   sync.Mutex
	reloadProcess bool
	// mutex guards the runtime devices map access
	mutex sync.RWMutex

	processWg sync.WaitGroup

	Cluster *HACluster

	MaxWorkers int
)

// SetLogger sets the current log output.
func SetLogger(l *logrus.Logger) {
	log = l
}

func initCluster(master string, slave string) *HACluster {

	if len(master) == 0 {
		master = MainConfig.General.MasterDB
	}
	if len(slave) == 0 {
		slave = MainConfig.General.SlaveDB
	}

	log.Infof("Initializing cluster")

	var MDB *InfluxMonitor
	var SDB *InfluxMonitor

	for {
		slaveFound := false
		masterAlive := true
		masterFound := false
		slaveAlive := true

		for _, idb := range MainConfig.InfluxArray {
			if idb.Name == master {
				masterFound = true
				log.Infof("Found MasterDB[%s] in config File %+v", master, idb)
				MDB = &InfluxMonitor{cfg: idb, CheckInterval: MainConfig.General.CheckInterval}

				cli, _, _, err := MDB.InitPing()
				if err != nil {
					masterAlive = false
					log.Errorf("MasterDB[%s] has  problems :%s", master, err)
				}
				MDB.SetCli(cli)
			}
			if idb.Name == slave {
				slaveFound = true
				log.Infof("Found SlaveDB[%s] in config File %+v", slave, idb)
				SDB = &InfluxMonitor{cfg: idb, CheckInterval: MainConfig.General.CheckInterval}

				cli, _, _, err := SDB.InitPing()
				if err != nil {
					slaveAlive = false
					log.Errorf("SlaveDB[%s] has  problems :%s", slave, err)
				}
				SDB.SetCli(cli)
			}

		}

		if slaveFound && masterFound && masterAlive && slaveAlive {
			return &HACluster{
				Master:               MDB,
				Slave:                SDB,
				CheckInterval:        MainConfig.General.MinSyncInterval,
				ClusterState:         "OK",
				SlaveStateOK:         true,
				SlaveLastOK:          time.Now(),
				MasterStateOK:        true,
				MasterLastOK:         time.Now(),
				MaxRetentionInterval: MainConfig.General.MaxRetentionInterval,
				ChunkDuration:        MainConfig.General.DataChunkDuration,
			}

		} else {
			if !slaveFound {
				log.Errorf("No Slave DB  found, please check config and restart the process")
			}
			if !masterFound {
				log.Errorf("No Master DB found, please check config and restart the process")
			}
			if !masterAlive {
				log.Errorf("Master DB is not runing I should wait until both up to begin to chek sync status")
			}
			if !slaveAlive {
				log.Errorf("Slave DB is not runing I should wait until both up to begin to chek sync status")
			}
		}
		time.Sleep(MainConfig.General.MonitorRetryInterval)
	}
}

func ReplSch(master string, slave string, dbs string, newdb string, rps string, newrp string, meas string) {

	Cluster = initCluster(master, slave)

	schema, err := Cluster.GetSchema(dbs, rps, meas)
	if err != nil {
		log.Errorf("Can not copy data , error on get Schema: %s", err)
		return
	}

	if len(newdb) > 0 && len(schema) > 0 {
		for p := range schema {
			schema[p].NewName = newdb
		}
	}

	if len(newrp) > 0 && len(schema) > 0 {
		for p := range schema {
			schema[p].NewDefRp = newrp
		}
	}

	s := time.Now()
	Cluster.ReplicateSchema(schema)
	elapsed := time.Since(s)
	log.Infof("Replicate Schame take: %s", elapsed.String())

}

func SchCopy(master string, slave string, dbs string, newdb string, rps string, newrp string, meas string, start time.Time, end time.Time, full bool, copyorder string) {

	Cluster = initCluster(master, slave)

	schema, err := Cluster.GetSchema(dbs, rps, meas)
	if err != nil {
		log.Errorf("Can not copy data , error on get Schema: %s", err)
		return
	}

	if len(newdb) > 0 && len(schema) > 0 {
		for p := range schema {
			schema[p].NewName = newdb
		}
	}

	if len(newrp) > 0 && len(schema) > 0 {
		for p := range schema {
			schema[p].NewDefRp = newrp
		}
	}

	s := time.Now()
	Cluster.ReplicateSchema(schema)
	if full {
		Cluster.ReplicateDataFull(schema, copyorder)
	} else {
		Cluster.ReplicateData(schema, start, end, copyorder)
	}
	elapsed := time.Since(s)
	log.Infof("Copy take: %s", elapsed.String())

}

func Copy(master string, slave string, dbs string, newdb string, rps string, newrp string, meas string, start time.Time, end time.Time, full bool, copyorder string) {

	Cluster = initCluster(master, slave)

	schema, err := Cluster.GetSchema(dbs, rps, meas)
	if err != nil {
		log.Errorf("Can not copy data , error on get Schema: %s", err)
		return
	}

	if len(newdb) > 0 && len(schema) > 0 {
		for p := range schema {
			schema[p].NewName = newdb
		}
	}
	if len(newrp) > 0 && len(schema) > 0 {
		for p := range schema {
			schema[p].NewDefRp = newrp
		}
	}

	s := time.Now()
	if full {
		Cluster.ReplicateDataFull(schema, copyorder)
	} else {
		Cluster.ReplicateData(schema, start, end, copyorder)
	}
	elapsed := time.Since(s)
	log.Infof("Copy take: %s", elapsed.String())

}

func HAMonitorStart(master string, slave string, copyorder string) {

	Cluster = initCluster(master, slave)

	schema, _ := Cluster.GetSchema("", "", "")

	switch MainConfig.General.InitialReplication {
	case "schema":
		log.Info("Replicating DB Schema from Master to Slave")
		Cluster.ReplicateSchema(schema)
	case "data":
		log.Info("Replicating DATA Schema from Master to Slave")
		Cluster.ReplicateDataFull(schema, copyorder)
	case "both":
		log.Info("Replicating DB Schema from Master to Slave")
		Cluster.ReplicateSchema(schema)
		log.Info("Replicating DATA Schema from Master to Slave")
		Cluster.ReplicateDataFull(schema, copyorder)
	case "none":
		log.Info("No replication done")
	default:
		log.Errorf("Unknown replication config %s", MainConfig.General.InitialReplication)
	}

	Cluster.Master.StartMonitor(&processWg)
	Cluster.Slave.StartMonitor(&processWg)
	time.Sleep(MainConfig.General.CheckInterval)
	Cluster.SuperVisor(&processWg, copyorder)

}

// End stops all devices polling.
func End() (time.Duration, error) {

	start := time.Now()
	//nothing to do
	return time.Since(start), nil
}

// ReloadConf stops the polling, reloads all configuration and restart the polling.
func ReloadConf() (time.Duration, error) {
	start := time.Now()
	//nothing to do yet
	return time.Since(start), nil
}
