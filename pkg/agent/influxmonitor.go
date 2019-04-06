package agent

import (
	"fmt"
	"sync"
	"time"

	"github.com/influxdata/influxdb1-client/v2"
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
	cli               client.Client
	lastcli           client.Client
	climutex          sync.RWMutex
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

func (im *InfluxMonitor) InitPing() (client.Client, time.Duration, string, error) {

	info := client.HTTPConfig{
		Addr:     im.cfg.Location,
		Username: im.cfg.AdminUser,
		Password: im.cfg.AdminPasswd,
		Timeout:  im.cfg.Timeout,
	}

	con, err2 := client.NewHTTPClient(info)
	if err2 != nil {
		log.Errorf("Fail to build newclient to database %s, error: %s\n", im.cfg.Location, err2)
		return nil, 0, "", err2
	}

	dur, ver, err3 := con.Ping(im.cfg.Timeout)
	if err3 != nil {
		log.Errorf("Fail to build newclient to database %s, error: %s\n", im.cfg.Location, err3)
		return nil, 0, "", err3
	}

	q := client.Query{
		Command:  "show databases",
		Database: "",
	}

	response, err4 := con.Query(q)
	if err4 == nil && response.Error() == nil {
		log.Debugf("SHOW DATABASES: %+v", response.Results)
		im.lastcli = con
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
	im.lastcli = con
	return con, dur, ver, nil
}

func (im *InfluxMonitor) SetCli(cli client.Client) {
	im.climutex.Lock()
	defer im.climutex.Unlock()
	im.cli = cli
}

func (im *InfluxMonitor) GetCli() client.Client {
	im.climutex.RLock()
	defer im.climutex.RUnlock()
	return im.cli
}

func (im *InfluxMonitor) UpdateCli() client.Client {
	im.climutex.RLock()
	defer im.climutex.RUnlock()
	im.cli = im.lastcli
	return im.cli
}

func (im *InfluxMonitor) Ping() (time.Duration, string, error) {

	cli := im.GetCli()
	if cli == nil {
		return 0, "", fmt.Errorf("can not ping database, the client is not initialized")
	}

	dur, ver, err3 := cli.Ping(im.cfg.Timeout)
	if err3 != nil {
		log.Errorf("Fail to build newclient to database %s, error: %s\n", im.cfg.Location, err3)
		return 0, "", err3
	}

	q := client.Query{
		Command:  "show databases",
		Database: "",
	}

	response, err4 := cli.Query(q)
	if err4 == nil && response.Error() == nil {
		log.Debugf("SHOW DATABASES: %+v", response.Results)
		return dur, ver, nil
	} else {
		if err4 != nil {
			//			log.Warnf(" ERR4: %s", err4)
			return dur, ver, err4
		}
		if response.Error() != nil {
			//			log.Warnf("Response Error not null: ERR %s", response.Error())
			return dur, ver, response.Error()
		}

	}
	log.Debugf("SHOW DATABASES: %+v", response.Results)
	return dur, ver, nil
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
