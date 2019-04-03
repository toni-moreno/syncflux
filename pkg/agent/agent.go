package agent

import (
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
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
)

// SetLogger sets the current log output.
func SetLogger(l *logrus.Logger) {
	log = l
}

// Reload Mutex Related Methods.

// CheckAndSetReloadProcess sets the reloadProcess flag.
// Returns its previous value.

func init() {

}

func Start() {
	log.Infof("Beginning Agent")

	var MDB *InfluxMonitor
	var SDB *InfluxMonitor

	slaveFound := false
	masterAlive := true
	masterFound := false
	slaveAlive := true

	for _, idb := range MainConfig.InfluxArray {
		if idb.Name == MainConfig.General.MasterDB {
			masterFound = true
			log.Infof("Found MasterDB in config File %+v", idb)
			MDB = &InfluxMonitor{cfg: idb, CheckInterval: MainConfig.General.CheckInterval}
			// cound
			_, _, _, err := MDB.InitPing()
			if err != nil {
				masterAlive = false
				log.Errorf("MasterDB has  problems :%s", err)
			}

		}
		if idb.Name == MainConfig.General.SlaveDB {
			slaveFound = true
			log.Infof("Found SlaveDB in config File %+v", idb)
			SDB = &InfluxMonitor{cfg: idb, CheckInterval: MainConfig.General.CheckInterval}

			_, _, _, err := SDB.InitPing()
			if err != nil {
				slaveAlive = false
				log.Errorf("SlaveDB has  problems :%s", err)
			}
		}

	}

	if slaveFound && masterFound && masterAlive && slaveAlive {
		Cluster = &HACluster{
			Master:        MDB,
			Slave:         SDB,
			CheckInterval: MainConfig.General.MinSyncInterval,
			ClusterState:  "OK",
			SlaveStateOK:  true,
			SlaveLastOK:   time.Now(),
			MasterStateOK: true,
			MasterLastOK:  time.Now(),
		}
		schema, _ := Cluster.GetSchema()

		switch MainConfig.General.InitialReplication {
		case "schema":
			Cluster.ReplicateSchema(schema)
		case "data":
			Cluster.ReplicateData(schema)
		case "both":
			Cluster.ReplicateSchema(schema)
			Cluster.ReplicateData(schema)
		default:
			log.Errorf("Unknown replication config %s", MainConfig.General.InitialReplication)
		}

		MDB.StartMonitor(&processWg)
		SDB.StartMonitor(&processWg)
		time.Sleep(MainConfig.General.CheckInterval)
		Cluster.SuperVisor(&processWg)
	} else {
		if !slaveFound {
			log.Errorf("No Slave DB  found")
		}
		if !masterFound {
			log.Errorf("No Master DB found")
		}
	}
}

// End stops all devices polling.
func End() (time.Duration, error) {

	start := time.Now()
	log.Infof("END: begin device Gather processes stop... at %s", start.String())
	// stop all device processes
	log.Info("END: begin selfmon Gather processes stop...")

	// wait until Done
	processWg.Wait()

	log.Infof("END: Finished from %s to %s [Duration : %s]", start.String(), time.Now().String(), time.Since(start).String())
	return time.Since(start), nil
}

// ReloadConf stops the polling, reloads all configuration and restart the polling.
func ReloadConf() (time.Duration, error) {
	start := time.Now()
	log.Infof("RELOADCONF INIT: begin device Gather processes stop... at %s", start.String())
	End()

	log.Info("RELOADCONF: loading configuration Again...")

	log.Info("RELOADCONF: Starting all device processes again...")
	// Initialize Devices in Runtime map

	log.Infof("RELOADCONF END: Finished from %s to %s [Duration : %s]", start.String(), time.Now().String(), time.Since(start).String())

	return time.Since(start), nil
}
