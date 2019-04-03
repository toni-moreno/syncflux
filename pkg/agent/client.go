package agent

import (
	"fmt"
	"strconv"

	"encoding/json"
	client "github.com/influxdata/influxdb1-client"
	"net/url"
	"time"
)

func DBclient(location string, user string, pass string) (*client.Client, error) {

	//connect to database
	u, err := url.Parse(location)
	if err != nil {
		log.Printf("Fail to parse host and port of database %s, error: %s\n", location, err)
		return nil, err
	}

	info := client.Config{
		URL:       *u,
		Username:  user,
		Password:  pass,
		UserAgent: "syncflux-agent",
	}

	con, err2 := client.NewClient(info)
	if err2 != nil {
		log.Printf("Fail to build newclient to database %s, error: %s\n", location, err2)
		return nil, err2
	}

	dur, ver, err3 := con.Ping()
	if err != nil {
		log.Printf("Fail to build newclient to database %s, error: %s\n", location, err2)
		return nil, err3
	}

	q := client.Query{
		Command:  "show databases",
		Database: "",
	}
	response, err4 := con.Query(q)
	if err4 == nil && response.Error() == nil {
		log.Println(response.Results)
		return con, nil
	} else {
		if err4 != nil {
			return nil, err4
		}
		if response.Error() != nil {
			return nil, response.Error()
		}

	}
	log.Printf("Happy as a hippo! %v, %s", dur, ver)

	return con, nil
}

type RetPol struct {
	Name               string
	Duration           time.Duration
	ShardGroupDuration time.Duration
	NReplicas          int64
	Def                bool
}

func CreateDB(con *client.Client, db string, rp *RetPol) error {

	if db == "_internal" {
		return nil
	}

	cmd := "CREATE DATABASE " + db + " WITH DURATION " + rp.Duration.String() + " REPLICATION " + strconv.FormatInt(rp.NReplicas, 10) + " SHARD DURATION " + rp.ShardGroupDuration.String() + " NAME " + rp.Name

	q := client.Query{
		Command: cmd,
	}
	response, err4 := con.Query(q)
	if err4 == nil && response.Error() == nil {
		log.Debugf("Database Creation response %#+v", response)
		return nil
	}
	if err4 != nil {
		return err4
	}
	if response.Error() != nil {
		return response.Error()
	}

	return nil
}

func CreateRP(con *client.Client, db string, rp *RetPol) error {

	cmd := "CREATE RETENTION POLICY " + rp.Name + " ON " + db + " DURATION " + rp.Duration.String() + " REPLICATION " + strconv.FormatInt(rp.NReplicas, 10) + " SHARD DURATION " + rp.ShardGroupDuration.String()
	if rp.Def {
		cmd += " DEFAULT"
	}
	log.Debugf("Influx QUERY: %s", cmd)
	q := client.Query{
		Command: cmd,
	}
	response, err4 := con.Query(q)
	if err4 == nil && response.Error() == nil {
		log.Debugf("Retention Policy Creation response %#+v", response)
		return nil
	}
	if err4 != nil {
		return err4
	}
	if response.Error() != nil {
		return response.Error()
	}

	return nil
}

func GetDataBases(con *client.Client) ([]string, error) {
	databases := []string{}
	q := client.Query{
		Command:  "show databases",
		Database: "",
	}
	response, err4 := con.Query(q)
	if err4 == nil && response.Error() == nil {
		for j, k := range response.Results[0].Series[0].Values {
			log.Debugf("discovered database %d: %s", j, k)
			db := k[0]
			if db.(string) != "_internal" {
				databases = append(databases, db.(string))
			}
		}
		return databases, nil
	} else {
		if err4 != nil {
			return nil, err4
		}
		if response.Error() != nil {
			return nil, response.Error()
		}
	}
	return nil, nil
}

func GetRetentionPolicies(con *client.Client, db string) ([]*RetPol, error) {
	rparray := []*RetPol{}
	q := client.Query{
		Command:  "show retention policies on " + db,
		Database: "",
	}
	response, err4 := con.Query(q)
	if err4 == nil && response.Error() == nil {
		for j, k := range response.Results[0].Series[0].Values {
			log.Debugf("discovered retention Policies %d:  %d : %#+v", j, len(k), k)
			var d, sgd time.Duration
			var err error

			d, err = time.ParseDuration(k[1].(string))
			if err != nil {
				log.Errorf("Error on parsing Duration: Err %s", err)
				continue
			}

			sgd, err = time.ParseDuration(k[2].(string))
			if err != nil {
				log.Errorf("Error on parsing Duration: Err %s", err)
				continue
			}
			nr, err2 := k[3].(json.Number).Int64()
			if err != nil {
				log.Errorf("Error on parse num replicas :%s", err2)
			}
			rp := &RetPol{
				Name:               k[0].(string),
				Duration:           d,
				ShardGroupDuration: sgd,
				NReplicas:          nr,
				Def:                k[4].(bool),
			}
			rparray = append(rparray, rp)

		}
		return rparray, nil
	} else {
		if err4 != nil {
			return nil, err4
		}
		if response.Error() != nil {
			return nil, response.Error()
		}
	}
	return rparray, nil
}

func GetMeasurements(c *client.Client, sdb string) []string {

	cmd := "show measurements"
	//get measurements from database
	q := client.Query{
		Command:  cmd,
		Database: sdb,
	}

	var measurements []string

	response, err := c.Query(q)
	if err != nil {
		log.Printf("Fail to get response from database, get measurements error: %s\n", err.Error())
	}

	//log.Debugf("%s: %+v", cmd, response)

	res := response.Results

	if len(res[0].Series) == 0 {
		log.Printf("The response of database is null, get measurements error!\n")
	} else {

		values := res[0].Series[0].Values

		//show progress of getting measurements

		for _, row := range values {
			measurement := fmt.Sprintf("%v", row[0])
			measurements = append(measurements, measurement)
			time.Sleep(3 * time.Millisecond)
		}

	}
	return measurements

}

func ReadDB(c *client.Client, sdb, srp, ddb, drp, cmd string) client.BatchPoints {

	q := client.Query{
		Command:         cmd,
		Database:        sdb,
		RetentionPolicy: srp,
	}

	//get type client.BatchPoints
	var batchpoints client.BatchPoints

	response, err := c.Query(q)
	if err != nil {
		log.Printf("Fail to get response from database, read database error: %s\n", err.Error())
	}

	res := response.Results
	if len(res) == 0 {
		log.Printf("The response of database is null, read database error!\n")
	} else {

		res_length := len(res)
		for k := 0; k < res_length; k++ {

			//show progress of reading series

			for _, ser := range res[k].Series {

				//get type client.Point
				var point client.Point

				point.Measurement = ser.Name
				point.Tags = ser.Tags
				for _, v := range ser.Values {
					point.Time, _ = time.Parse(time.RFC3339, v[0].(string))

					field := make(map[string]interface{})
					l := len(v)
					for i := 1; i < l; i++ {
						if v[i] != nil {
							field[ser.Columns[i]] = v[i]
						}
					}
					point.Fields = field
					point.Precision = "s"
					batchpoints.Points = append(batchpoints.Points, point)
				}
				time.Sleep(3 * time.Millisecond)
			}
		}
		batchpoints.Database = ddb
		batchpoints.RetentionPolicy = drp
	}
	return batchpoints
}

func WriteDB(c *client.Client, b client.BatchPoints) {

	_, err := c.Write(b)
	if err != nil {
		log.Printf("Fail to write to database, error: %s\n", err.Error())
	}
}

func SyncDBFull(src *InfluxMonitor, dst *InfluxMonitor, db string, rp *RetPol) error {

	var eEpoch, sEpoch time.Time
	var hLength int64

	eEpoch = time.Now()

	if rp.Duration != 0 {

		sEpoch = eEpoch.Add(-rp.Duration)

		duration := time.Since(sEpoch)

		hLength = int64(duration.Hours()) + 1

		//The datas which can be inputed is less than a year
		if hLength > 8760 {
			hLength = 8760
		}
	} else {
		hLength = 8760
	}

	measurements := GetMeasurements(src.cli, db)
	var i int64
	for i = 0; i < hLength; i++ {

		//sync from newer to older data
		endsec := eEpoch.Unix() - (i * 3600)
		startsec := eEpoch.Unix() - ((i + 1) * 3600)
		totalpoints := 0
		for _, m := range measurements {
			log.Infof("Processing measurement %s", m)

			//write datas of every hour

			log.Debugf("processing Database %s Measurement %s from %d to %d", db, m, startsec, endsec)
			getvalues := fmt.Sprintf("select * from  \"%v\" where time  > %vs and time < %vs group by *", m, startsec, endsec)
			batchpoints := ReadDB(src.cli, db, rp.Name, db, rp.Name, getvalues)
			np := len(batchpoints.Points)
			totalpoints += np
			log.Debugf("processed %d points", np)
			WriteDB(dst.cli, batchpoints)

			time.Sleep(time.Millisecond)
		}
		log.Infof("Processed Chunk [%d] from [%d][%s] to [%d][%s] (%d) Points", i, startsec, time.Unix(startsec, 0).String(), endsec, time.Unix(endsec, 0).String(), totalpoints)

		time.Sleep(time.Millisecond)
	}

	log.Printf("Move data from %s to %s has done!\n", src.cfg.Name, dst.cfg.Name)
	return nil
}

func SyncDBs(src *InfluxMonitor, dst *InfluxMonitor, stime time.Time, etime time.Time) error {

	scon, err1 := DBclient(src.cfg.Location, src.cfg.AdminUser, src.cfg.AdminPasswd)
	if err1 != nil {
		log.Errorf("%s", err1)
		return err1
	}
	dcon, err2 := DBclient(dst.cfg.Location, dst.cfg.AdminUser, dst.cfg.AdminPasswd)
	if err2 != nil {
		log.Errorf("%s", err2)
		return err2
	}

	template := "2006-01-02 15:04:05"

	since_time, err_sin := time.Parse(template, "1970-01-01 00:00:00")
	if err_sin != nil {
		log.Println("Fail to parse since_time")
	}

	s_epoch := stime.Sub(since_time)
	e_epoch := etime.Sub(since_time)

	h_length := int(e_epoch.Hours()-s_epoch.Hours()) + 1

	//The datas which can be inputed is less than a year
	if h_length > 8760 {
		h_length = 8760
	}
	dbarray, _ := GetDataBases(scon)

	for _, db := range dbarray {
		log.Infof("Processing Database %s", db)

		// Get Retention policies
		rps, err := GetRetentionPolicies(scon, db)
		if err != nil {
			log.Errorf("Error on get Retention Policies on Database %s (%) : Error: %s", db, src.cfg.Name, src.cfg.Location, err)
			continue
		}

		for _, rp := range rps {

			measurements := GetMeasurements(scon, db)

			for i := 0; i < h_length; i++ {

				startsec := int(s_epoch.Seconds() + float64(i*3600))
				endsec := int(s_epoch.Seconds() + float64((i+1)*3600))
				log.Infof("Processed Chunk [%d] from [%d][%s] to [%d][%s] (%d) Points", i, startsec, time.Unix(startsec, 0).String(), endsec, time.Unix(endsec, 0).String(), totalpoints)

				for _, m := range measurements {
					log.Infof("Processing measurement %s", m)

					//write datas of every hour

					log.Debugf("processing Database %s Measurement %s from %d to %d", db, m, startsec, endsec)
					getvalues := fmt.Sprintf("select * from  \"%v\" where time  > %vs and time < %vs group by *", m, startsec, endsec)
					batchpoints := ReadDB(scon, db, rp.Name, db, rp.Name, getvalues)
					log.Debugf("processed %d points", len(batchpoints.Points))
					WriteDB(dcon, batchpoints)

					time.Sleep(time.Millisecond)
				}
				time.Sleep(time.Millisecond)
			}

		}

	}

	log.Printf("Move data from %s to %s has done!\n", src.cfg.Name, dst.cfg.Name)
	return nil
}
