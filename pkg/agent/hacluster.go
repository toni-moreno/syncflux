package agent

import (
	"regexp"
	"sync"
	"time"
)

type InfluxSchDb struct {
	Name     string
	NewName  string
	DefRp    string
	NewDefRp string
	Rps      []*RetPol
}

type MeasurementSch struct {
	Name   string
	Fields map[string]*FieldSch
}

type FieldSch struct {
	Name string
	Type string
}

type HACluster struct {
	Master                     *InfluxMonitor
	Slave                      *InfluxMonitor
	CheckInterval              time.Duration
	ClusterState               string
	SlaveStateOK               bool
	SlaveLastOK                time.Time
	SlaveCheckDuration         time.Duration
	MasterStateOK              bool
	MasterLastOK               time.Time
	MasterCheckDuration        time.Duration
	ClusterNumRecovers         int
	ClusterLastRecoverDuration time.Duration
	statsData                  sync.RWMutex
	Schema                     []*InfluxSchDb
	ChunkDuration              time.Duration
	MaxRetentionInterval       time.Duration
}

type ClusterStatus struct {
	ClusterState               string
	ClusterNumRecovers         int
	ClusterLastRecoverDuration time.Duration
	MID                        string
	SID                        string
	MasterState                bool
	MasterLastOK               time.Time
	SlaveState                 bool
	SlaveLastOK                time.Time
}

func (hac *HACluster) GetStatus() *ClusterStatus {
	hac.statsData.RLock()
	defer hac.statsData.RUnlock()
	return &ClusterStatus{
		ClusterState:               hac.ClusterState,
		MID:                        hac.Master.cfg.Name,
		SID:                        hac.Slave.cfg.Name,
		ClusterNumRecovers:         hac.ClusterNumRecovers,
		ClusterLastRecoverDuration: hac.ClusterLastRecoverDuration,
		MasterState:                hac.MasterStateOK,
		MasterLastOK:               hac.MasterLastOK,
		SlaveState:                 hac.SlaveStateOK,
		SlaveLastOK:                hac.SlaveLastOK,
	}
}

// From Master to Slave
func (hac *HACluster) GetSchema(dbfilter string, rpfilter string, measfilter string) ([]*InfluxSchDb, error) {

	schema := []*InfluxSchDb{}

	var filterdb *regexp.Regexp
	var filterrp *regexp.Regexp
	var filtermeas *regexp.Regexp

	var err error

	if len(dbfilter) > 0 {
		filterdb, err = regexp.Compile(dbfilter)
		if err != nil {
			return nil, err
		}
	}

	srcDBs, _ := GetDataBases(hac.Master.cli)

	for _, db := range srcDBs {

		if len(dbfilter) > 0 && !filterdb.MatchString(db) {
			log.Debugf("Database %s not match to regex %s:  skipping.. ", db, dbfilter)
			continue
		}

		// Get Retention policies
		rps, err := GetRetentionPolicies(hac.Master.cli, db)
		if err != nil {
			log.Errorf("Error on get Retention Policies on Database %s MasterDB %s : Error: %s", db, hac.Master.cfg.Name, err)
			continue
		}

		if len(rpfilter) > 0 {
			filterrp, err = regexp.Compile(rpfilter)
			if err != nil {
				return nil, err
			}
		}

		//check for default RP
		var defaultRp *RetPol

		for _, rp := range rps {
			if len(rpfilter) > 0 && !filterrp.MatchString(rp.Name) {
				log.Debugf("Retention policy %s not match to regex %s:  skipping.. ", rp.Name, rpfilter)
				continue
			}
			if rp.Def {
				defaultRp = rp
			}

			meas := GetMeasurements(hac.Master.cli, db, rp.Name, measfilter)

			if len(measfilter) > 0 {
				filtermeas, err = regexp.Compile(measfilter)
				if err != nil {
					return nil, err
				}
			}

			mf := make(map[string]*MeasurementSch, len(meas))

			for _, m := range meas {

				if len(measfilter) > 0 && !filtermeas.MatchString(m.Name) {
					log.Debugf("Measurement %s not match to regex %s:  skipping.. ", m.Name, measfilter)
					continue
				}

				log.Debugf("discovered measurement  %s on DB: %s-RP:%s", m, db, rp.Name)
				mf[m.Name] = m
				mf[m.Name].Fields = GetFields(hac.Master.cli, db, m.Name, rp.Name)
			}
			rp.Measurements = mf

		}

		// Check if default RP is valid
		if defaultRp == nil {
			log.Errorf("Error on Create DB  %s on SlaveDB %s : Database has not default Retention Policy ", db, hac.Slave.cfg.Name)
			continue
		}
		schema = append(schema, &InfluxSchDb{Name: db, NewName: db, DefRp: defaultRp.Name, NewDefRp: defaultRp.Name, Rps: rps})
	}
	hac.Schema = schema
	return schema, nil
}

// From Master to Slave
func (hac *HACluster) ReplicateSchema(schema []*InfluxSchDb) error {

	for _, db := range schema {
		//check for default RP
		var defaultRp RetPol

		for _, rp := range db.Rps {
			if rp.Def {
				defaultRp = *rp
				defaultRp.Name = db.NewDefRp
				break
			}
		}

		crdberr := CreateDB(hac.Slave.cli, db.NewName, &defaultRp)

		if crdberr != nil {
			log.Errorf("Error on Create DB  %s on SlaveDB %s : Error: %s", db, hac.Slave.cfg.Name, crdberr)
			//continue
		}
		for _, rp := range db.Rps {
			//If its default, ensure that it exist and assign it as default R
			if rp.Def {
				// default has been previously created
				// Ensure its default
				crrperr := CreateRP(hac.Slave.cli, db.NewName, &defaultRp)
				if crrperr != nil {
					log.Errorf("Error on Create Retention Policies on Database %s SlaveDB %s : Error: %s", db, hac.Slave.cfg.Name, crrperr)
				}
				alrperr := SetDefaultRP(hac.Slave.cli, db.NewName, &defaultRp)
				if alrperr != nil {
					log.Errorf("Error on Altern Retention Policies on Database %s SlaveDB %s : Error: %s", db, hac.Slave.cfg.Name, alrperr)
				}
				continue
			}
			//For other cases, creates the RP
			log.Infof("Creating Extra Retention Policy %s on database %s ", rp.Name, db)
			crrperr := CreateRP(hac.Slave.cli, db.NewName, rp)
			if crrperr != nil {
				log.Errorf("Error on Create Retention Policies on Database %s SlaveDB %s : Error: %s", db, hac.Slave.cfg.Name, crrperr)
				continue
			}
			log.Infof("Replication Schema: DB %s OK", db)
		}
	}
	return nil
}

func (hac *HACluster) ReplicateData(schema []*InfluxSchDb, start time.Time, end time.Time, copyorder string) error {
	for _, db := range schema {
		for _, rp := range db.Rps {
			log.Infof("Replicating Data from DB %s RP %s...", db.Name, rp.Name)
			//Need to check if the rp is the default, in that case must provide other name
			rn := *rp
			if rp.Def {
				rn.Name = db.NewDefRp
			}
			//log.Debugf("%s RP %s... SCHEMA %#+v.", db.Name, rp.Name, db)
			report := SyncDBRP(hac.Master, hac.Slave, db.Name, db.NewName, rp, &rn, start, end, db, hac.ChunkDuration, hac.MaxRetentionInterval, copyorder)
			if report == nil {
				log.Errorf("Data Replication error in DB [%s] RP [%s] ", db, rn.Name)
			}
			if len(report.BadChunks) > 0 {
				r, w, t := report.RWErrors()
				log.Errorf("Data Replication error in DB [%s] RP [%s] | Registered %d Read %d Write | %d Total Errors", db, r, w, t)
			}
		}
	}
	return nil
}

func (hac *HACluster) ReplicateDataFull(schema []*InfluxSchDb, copyorder string) error {
	for _, db := range schema {
		for _, rp := range db.Rps {
			log.Infof("Replicating Data from DB %s RP %s....", db.Name, rp.Name)
			start, end := rp.GetFirstLastTime(hac.MaxRetentionInterval)
			rn := *rp
			if rn.Def {
				rn.Name = db.NewDefRp
			}
			report := SyncDBRP(hac.Master, hac.Slave, db.Name, db.NewName, rp, &rn, start, end, db, hac.ChunkDuration, hac.MaxRetentionInterval, copyorder)
			if report == nil {
				log.Errorf("Data Replication error in DB [%s] RP [%s] ", db, rn.Name)
			}
			if len(report.BadChunks) > 0 {
				r, w, t := report.RWErrors()
				log.Errorf("Data Replication error in DB [%s] RP [%s] | Registered %d Read %d Write | %d Total Errors", db, r, w, t)
			}
		}
	}
	return nil
}

// ScltartMonitor Main GoRutine method to begin snmp data collecting
func (hac *HACluster) SuperVisor(wg *sync.WaitGroup, copyorder string) {
	wg.Add(1)
	go hac.startSupervisorGo(wg, copyorder)
}

// OK -> CHECK_SLAVE_DOWN -> RECOVERING -> OK

func (hac *HACluster) checkCluster(copyorder string) {

	//check Master

	lastMaster, lastmaOK, durationM := hac.Master.GetState()
	lastSlave, lastslOK, durationS := hac.Slave.GetState()

	// STATES
	// ALL OK
	// DETECTED SLAVE DONW
	// STILL DOWN
	// DETECTED UP
	// RECOVERING

	log.Info("HACluster check....")

	switch {
	//detected Down
	case hac.SlaveStateOK && lastSlave != true:
		log.Infof("HACLuster: detected DOWN Last(%s) Duratio OK (%s)", lastslOK.String(), durationS.String())
		hac.statsData.Lock()
		hac.ClusterState = "CHECK_SLAVE_DOWN"
		hac.MasterStateOK = lastMaster
		hac.MasterLastOK = lastmaOK
		hac.MasterCheckDuration = durationM
		hac.SlaveLastOK = lastslOK
		hac.SlaveStateOK = lastSlave
		hac.SlaveCheckDuration = durationS
		hac.statsData.Unlock()
		return
	//still Down
	case hac.ClusterState == "CHECK_SLAVE_DOWN" && lastSlave == false:
		hac.statsData.Lock()
		hac.MasterStateOK = lastMaster
		hac.MasterLastOK = lastmaOK
		hac.MasterCheckDuration = durationM
		hac.statsData.Unlock()
		return
	// Detected UP
	case hac.ClusterState == "CHECK_SLAVE_DOWN" && lastSlave == true:
		log.Infof("HACLuster: detected UP Last(%s) Duratio OK (%s) RECOVERING", lastslOK.String(), durationS.String())
		// service has been recovered is time to sincronize

		hac.statsData.Lock()
		startTime := hac.SlaveLastOK.Add(-hac.CheckInterval)

		hac.ClusterState = "RECOVERING"
		hac.MasterStateOK = lastMaster
		hac.MasterLastOK = lastmaOK
		hac.MasterCheckDuration = durationM
		hac.SlaveLastOK = lastslOK
		hac.SlaveStateOK = lastSlave
		hac.SlaveCheckDuration = durationS
		hac.statsData.Unlock()

		endTime := lastslOK

		// after conection recover with de database the
		// the client should be updated before any connection test
		hac.Slave.UpdateCli()
		// begin recover
		log.Infof("HACLUSTER: INIT RECOVERY : FROM [ %s ] TO [ %s ]", startTime.String(), endTime.String())
		start := time.Now()
		//refresh schema
		log.Infof("HACLUSTER: INIT REFRESH SCHEMA")
		hac.Schema, _ = hac.GetSchema("", "", "")
		log.Infof("HACLUSTER: INIT REPLICATION DATA PROCESS")
		hac.ReplicateData(hac.Schema, startTime, endTime, copyorder)
		elapsed := time.Since(start)
		log.Infof("HACLUSTER: DATA SYNCRONIZATION Took %s", elapsed.String())

		hac.statsData.Lock()
		hac.ClusterState = "OK"
		hac.ClusterNumRecovers++
		hac.ClusterLastRecoverDuration = elapsed
		hac.statsData.Unlock()
		return
	//Detected Recover
	case hac.ClusterState == "RECOVERING":
		log.Infof("HACluster: Database Still recovering")
		hac.statsData.Lock()
		hac.MasterStateOK = lastMaster
		hac.MasterLastOK = lastmaOK
		hac.MasterCheckDuration = durationM
		hac.SlaveLastOK = lastslOK
		hac.SlaveStateOK = lastSlave
		hac.SlaveCheckDuration = durationS
		hac.statsData.Unlock()
		return
	case hac.ClusterState == "OK" && lastSlave == true:
		hac.statsData.Lock()
		hac.MasterStateOK = lastMaster
		hac.MasterLastOK = lastmaOK
		hac.MasterCheckDuration = durationM
		hac.SlaveLastOK = lastslOK
		hac.SlaveStateOK = lastSlave
		hac.SlaveCheckDuration = durationS
		hac.statsData.Unlock()
	default:
		log.Warnf("HACLUSTER: undhanled State Last MasterOK %t %s", lastMaster, lastmaOK.String())
		log.Warnf("HACLUSTER: undhanled State Last SlaveOK %t %s", lastSlave, lastslOK.String())
		return
	}

}

func (hac *HACluster) startSupervisorGo(wg *sync.WaitGroup, copyorder string) {
	defer wg.Done()

	log.Infof("Beginning Supervision process  process each %s ", hac.CheckInterval.String())
	hac.MasterStateOK, hac.MasterLastOK, _ = hac.Master.GetState()
	hac.SlaveStateOK, hac.SlaveLastOK, _ = hac.Slave.GetState()

	t := time.NewTicker(hac.CheckInterval)
	for {
		hac.checkCluster(copyorder)
	LOOP:
		for {
			select {
			case <-t.C:
				break LOOP
			}
		}
	}
}
