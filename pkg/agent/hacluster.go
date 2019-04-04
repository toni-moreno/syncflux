package agent

import (
	"sync"
	"time"
)

type InfluxSchDb struct {
	Name   string
	DefRp  string
	Rps    []*RetPol
	Ftypes map[string]map[string]string
}

type HACluster struct {
	Master        *InfluxMonitor
	Slave         *InfluxMonitor
	CheckInterval time.Duration
	ClusterState  string
	SlaveStateOK  bool
	SlaveLastOK   time.Time
	MasterStateOK bool
	MasterLastOK  time.Time
	statsData     sync.RWMutex
	Schema        []*InfluxSchDb
}

type ClusterStatus struct {
	ClusterState string
	MasterState  bool
	MasterLastOK time.Time
	SlaveState   bool
	SlaveLastOK  time.Time
}

func (hac *HACluster) GetStatus() *ClusterStatus {
	hac.statsData.RLock()
	defer hac.statsData.RUnlock()
	return &ClusterStatus{
		ClusterState: hac.ClusterState,
		MasterState:  hac.MasterStateOK,
		MasterLastOK: hac.MasterLastOK,
		SlaveState:   hac.SlaveStateOK,
		SlaveLastOK:  hac.SlaveLastOK,
	}
}

// From Master to Slave
func (hac *HACluster) GetSchema() ([]*InfluxSchDb, error) {

	schema := []*InfluxSchDb{}

	srcDBs, _ := GetDataBases(hac.Master.cli)

	for _, db := range srcDBs {

		// Get Retention policies
		rps, err := GetRetentionPolicies(hac.Master.cli, db)
		if err != nil {
			log.Errorf("Error on get Retention Policies on Database %s MasterDB %s : Error: %s", db, hac.Master.cfg.Name, err)
			continue
		}

		//check for default RP
		var defaultRp *RetPol
		for _, rp := range rps {
			if rp.Def {
				defaultRp = rp
				break
			}
		}

		// Check if default RP is valid
		if defaultRp == nil {
			log.Errorf("Error on Create DB  %s on SlaveDB %s : Database has not default Retention Policy ", db, hac.Slave.cfg.Name)
			continue
		}

		meas := GetMeasurements(hac.Master.cli, db)

		mf := make(map[string]map[string]string, len(meas))

		for _, m := range meas {
			log.Debugf("discovered measurement  %s", m)
			mf[m] = GetFields(hac.Master.cli, db, m)
		}
		//
		schema = append(schema, &InfluxSchDb{Name: db, DefRp: defaultRp.Name, Rps: rps, Ftypes: mf})

		log.Infof("Replication Schema: DB %s OK", db)
	}
	hac.Schema = schema
	return schema, nil
}

// From Master to Slave
func (hac *HACluster) ReplicateSchema(schema []*InfluxSchDb) error {

	for _, db := range schema {
		//check for default RP
		var defaultRp *RetPol
		for _, rp := range db.Rps {
			if rp.Def {
				defaultRp = rp
				break
			}
		}
		crdberr := CreateDB(hac.Slave.cli, db.Name, defaultRp)
		if crdberr != nil {
			log.Errorf("Error on Create DB  %s on SlaveDB %s : Error: %s", db, hac.Slave.cfg.Name, crdberr)
			continue
		}
		for _, rp := range db.Rps {
			if rp.Def {
				// default has been previously created
				continue
			}
			log.Infof("Creating Extra Retention Policy %s on database %s ", rp.Name, db)
			crrperr := CreateRP(hac.Slave.cli, db.Name, rp)
			if crrperr != nil {
				log.Errorf("Error on Create Retention Policies on Database %s SlaveDB %s : Error: %s", db, hac.Slave.cfg.Name, crrperr)
				continue
			}
			log.Infof("Replication Schema: DB %s OK", db)

		}
	}
	return nil
}

/*/ From Master to Slave
func (hac *HACluster) ReplicateSchema() ([]*InfluxSchDb, error) {

	schema := []*InfluxSchDb{}

	srcDBs, _ := GetDataBases(hac.Master.cli)

	for _, db := range srcDBs {

		// Get Retention policies
		rps, err := GetRetentionPolicies(hac.Master.cli, db)
		if err != nil {
			log.Errorf("Error on get Retention Policies on Database %s MasterDB %s : Error: %s", db, hac.Master.cfg.Name, err)
			continue
		}

		//check for default RP
		var defaultRp *RetPol
		for _, rp := range rps {
			if rp.Def {
				defaultRp = rp
				break
			}
		}

		// Check if default RP is valid
		if defaultRp == nil {
			log.Errorf("Error on Create DB  %s on SlaveDB %s : Database has not default Retention Policy ", db, hac.Slave.cfg.Name)
			continue
		}
		//
		schema = append(schema, &InfluxSchDb{Name: db, DefRp: defaultRp.Name, Rps: rps})

		//======================================
		crdberr := CreateDB(hac.Slave.cli, db, defaultRp)
		if crdberr != nil {
			log.Errorf("Error on Create DB  %s on SlaveDB %s : Error: %s", db, hac.Slave.cfg.Name, crdberr)
			continue
		}
		log.Infof("Replication Schema: DB %s OK", db)

		for _, rp := range rps {
			if rp.Def {
				// default has been previously created
				continue
			}
			log.Infof("Creating Extra Retention Policy %s on database %s ", rp.Name, db)
			crrperr := CreateRP(hac.Slave.cli, db, rp)
			if crrperr != nil {
				log.Errorf("Error on Create Retention Policies on Database %s SlaveDB %s : Error: %s", db, hac.Slave.cfg.Name, crrperr)
				continue
			}
		}
	}
	return schema, nil
}*/

func (hac *HACluster) ReplicateData(schema []*InfluxSchDb) error {
	for _, db := range schema {
		var dbSch *InfluxSchDb
		for _, sch := range schema {
			if sch.Name == db.Name {
				dbSch = sch
			}
			break
		}
		for _, rp := range db.Rps {
			log.Infof("Replicating Data from DB %s RP %s....", db.Name, rp.Name)
			SyncDBFull(hac.Master, hac.Slave, db.Name, rp, dbSch)
		}
	}
	return nil
}

// ScltartMonitor Main GoRutine method to begin snmp data collecting
func (hac *HACluster) SuperVisor(wg *sync.WaitGroup) {
	wg.Add(1)
	go hac.startSupervisorGo(wg)
}

// OK -> CHECK_SLAVE_DOWN -> RECOVERING -> OK

func (hac *HACluster) checkCluster() {
	hac.statsData.Lock()
	defer hac.statsData.Unlock()
	//check Master

	hac.MasterStateOK, hac.MasterLastOK, _ = hac.Master.GetState()

	log.Info("HACluster check....")
	if hac.ClusterState == "RECOVERING" {
		log.Infof("HACluster: Database Still recovering")
		return
	}

	lastSlave, lastOK, duration := hac.Slave.GetState()
	if hac.ClusterState == "CHECK_SLAVE_DOWN" && lastSlave == true {
		log.Infof("HACLuster: detected UP Last(%s) Duratio OK (%s) RECOVERING", lastOK.String(), duration.String())
		// service has been recovered is time to sincronize
		hac.ClusterState = "RECOVERING"
		startTime := hac.SlaveLastOK.Add(-hac.CheckInterval)
		endTime := lastOK
		SyncDBs(hac.Master, hac.Slave, startTime, endTime, hac.Schema)
		hac.ClusterState = "OK"
		hac.SlaveLastOK = lastOK
	}
	if hac.SlaveStateOK && lastSlave != true {
		log.Infof("HACLuster: detected DOWN Last(%s) Duratio OK (%s)", lastOK.String(), duration.String())
		hac.ClusterState = "CHECK_SLAVE_DOWN"
	}
	hac.SlaveLastOK = lastOK
	hac.SlaveStateOK = lastSlave

}

func (hac *HACluster) startSupervisorGo(wg *sync.WaitGroup) {
	defer wg.Done()

	log.Infof("Beginning Supervision process  process each %s ", hac.CheckInterval.String())
	hac.MasterStateOK, hac.MasterLastOK, _ = hac.Master.GetState()
	hac.SlaveStateOK, hac.SlaveLastOK, _ = hac.Slave.GetState()

	t := time.NewTicker(hac.CheckInterval)
	for {
		hac.checkCluster()
	LOOP:
		for {
			select {
			case <-t.C:
				break LOOP
			}
		}
	}
}
