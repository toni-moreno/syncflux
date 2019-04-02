package agent

import (
	"net/url"
	"sync"
	"time"

	client "github.com/influxdata/influxdb1-client"
	"github.com/toni-moreno/syncflux/pkg/config"
)

type InfluxMonitor struct {
	cfg               *config.InfluxDB
	CheckInterval     time.Duration
	lastOK            time.Time
	lastStateDuration time.Duration
	statusOK          bool
	Version           string
	PingDuration      time.Duration
	statsData         sync.RWMutex
	cli               *client.Client
}

func (im *InfluxMonitor) setStatError() {
	im.statsData.Lock()
	defer im.statsData.Unlock()
	im.statusOK = false

}

func (im *InfluxMonitor) setStatOK(t time.Duration, version string) {
	im.statsData.Lock()
	defer im.statsData.Unlock()
	im.lastOK = time.Now()
	im.statusOK = true
	im.Version = version
	im.PingDuration = t
}

func (im *InfluxMonitor) GetState() (bool, time.Time, time.Duration) {
	im.statsData.RLock()
	defer im.statsData.RUnlock()
	return im.statusOK, im.lastOK, time.Since(im.lastOK)
}

func (im *InfluxMonitor) InitPing() (*client.Client, time.Duration, string, error) {

	//connect to database

	u, err := url.Parse(im.cfg.Location)
	if err != nil {
		log.Errorf("Fail to parse host and port of database %s, error: %s\n", im.cfg.Location, err)
		return nil, 0, "", err
	}

	info := client.Config{
		URL:      *u,
		Username: im.cfg.AdminUser,
		Password: im.cfg.AdminPasswd,
		Timeout:  im.cfg.Timeout,
	}

	con, err2 := client.NewClient(info)
	if err2 != nil {
		log.Errorf("Fail to build newclient to database %s, error: %s\n", im.cfg.Location, err2)
		return nil, 0, "", err2
	}

	dur, ver, err3 := con.Ping()
	if err != nil {
		log.Errorf("Fail to build newclient to database %s, error: %s\n", im.cfg.Location, err3)
		return nil, 0, "", err3
	}
	//log.Debugf("%s", dur.String())

	q := client.Query{
		Command:  "show databases",
		Database: "",
	}

	response, err4 := con.Query(q)
	if err4 == nil && response.Error() == nil {
		log.Debugf("SHOW DATABASES: %+v", response.Results)
		im.cli = con
		return con, dur, ver, nil
	} else {
		if err4 != nil {
			//			log.Warnf(" ERR4: %s", err4)
			return nil, dur, ver, err4
		}
		if response.Error() != nil {
			//			log.Warnf("Response Error not null: ERR %s", response.Error())
			return nil, dur, ver, response.Error()
		}

	}
	log.Debugf("SHOW DATABASES: %+v", response.Results)
	im.cli = con
	return con, dur, ver, nil
}

func (im *InfluxMonitor) GetStat() {
	_, dur, ver, err := im.InitPing()
	if err != nil {
		log.Warnf("InfluxDB : %s  NO OK (Error :%s )", im.cfg.Name, err)
		im.setStatError()
	} else {
		log.Infof("InfluxDB : %s  OK (Version  %s : Duration %s )", im.cfg.Name, ver, dur.String())
		im.setStatOK(dur, ver)
	}
}

// StartMonitor Main GoRutine method to begin snmp data collecting
func (im *InfluxMonitor) StartMonitor(wg *sync.WaitGroup) {
	wg.Add(1)
	go im.startMonitorGo(wg)
}

func (im *InfluxMonitor) startMonitorGo(wg *sync.WaitGroup) {
	defer wg.Done()

	log.Infof("Beginning Monitoring process  process for influxdb %s | %s", im.cfg.Name, im.cfg.Location)

	t := time.NewTicker(im.CheckInterval)
	for {

		im.GetStat()

	LOOP:
		for {
			select {
			case <-t.C:
				break LOOP
			}
		}
	}
}
