package agent

import (
	"fmt"
	"strconv"

	"encoding/json"
	//client "github.com/influxdata/influxdb1-client/v2"
	"github.com/influxdata/influxdb1-client/v2"
	//"net/url"
	"time"
)

func DBclient(location string, user string, pass string) (client.Client, error) {

	//connect to database
	/*u, err := url.Parse(location)
	if err != nil {
		log.Printf("Fail to parse host and port of database %s, error: %s\n", location, err)
		return nil, err
	}*/

	info := client.HTTPConfig{
		Addr:      location,
		Username:  user,
		Password:  pass,
		UserAgent: "syncflux-agent",
	}

	con, err2 := client.NewHTTPClient(info)
	if err2 != nil {
		log.Printf("Fail to build newclient to database %s, error: %s\n", location, err2)
		return nil, err2
	}

	dur, ver, err3 := con.Ping(time.Duration(10) * time.Second)
	if err3 != nil {
		log.Printf("Fail to build newclient to database %s, error: %s\n", location, err3)
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

func CreateDB(con client.Client, db string, rp *RetPol) error {

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

func CreateRP(con client.Client, db string, rp *RetPol) error {

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

func GetDataBases(con client.Client) ([]string, error) {
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

func GetRetentionPolicies(con client.Client, db string) ([]*RetPol, error) {
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

func GetFields(c client.Client, sdb string, meas string) map[string]string {
	ret := make(map[string]string)

	cmd := "show field keys from " + meas
	//get measurements from database
	q := client.Query{
		Command:  cmd,
		Database: sdb,
	}

	response, err := c.Query(q)
	if err != nil {
		log.Printf("Fail to get response from database, get measurements error: %s\n", err.Error())
	}

	res := response.Results

	if len(res[0].Series) == 0 {
		log.Warnf("The response for Query is null, get Fields from  DB %s Measurement %s error!\n", sdb, meas)
	} else {

		values := res[0].Series[0].Values
		//show progress of getting measurements
		for _, row := range values {
			fieldname := row[0].(string)
			fieldtype := row[1].(string)
			ret[fieldname] = fieldtype
			log.Debugf("Detected Field [%s] type [%s] on measurement [%s]", fieldname, fieldtype, meas)
		}

	}

	return ret
}

func GetMeasurements(c client.Client, sdb string) []string {

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
		log.Warnf(" Response for query is void, no measurements on DB %s", sdb)
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

func UnixNano2Time(tstamp int64) (time.Time, error) {
	sec := tstamp / 1000000000
	nsec := tstamp % 1000000000
	return time.Unix(sec, nsec), nil
}

func StrUnixNano2Time(tstamp string) (time.Time, error) {
	i, err := strconv.ParseInt(tstamp, 10, 64)
	if err != nil {
		log.Errorf("Error on parse time [%s]", tstamp, err)
		return time.Now(), err
	}
	sec := i / 1000000000
	nsec := i % 1000000000
	return time.Unix(sec, nsec), nil
}

func ReadDB(c client.Client, sdb, srp, ddb, drp, cmd string, fieldmap map[string]string) (client.BatchPoints, int64) {
	var totalpoints int64

	totalpoints = 0

	q := client.Query{
		Command:         cmd,
		Database:        sdb,
		RetentionPolicy: srp,
		Precision:       "ns",
	}

	bpcfg := client.BatchPointsConfig{
		Database:        ddb,
		RetentionPolicy: drp,
		Precision:       "ns",
	}

	batchpoints, err := client.NewBatchPoints(bpcfg)
	if err != nil {
		log.Error("Error on create BatchPoints: %s", err)
		return batchpoints, 0
	}

	response, err := c.Query(q)
	if err != nil {
		log.Warnf("Fail to get response from query %s, read database error: %s", cmd, err.Error())
	}

	res := response.Results
	if len(res) == 0 {
		log.Warnf("The response of query [%s] is void, read database error!\n", cmd)
	} else {

		resLength := len(res)
		for k := 0; k < resLength; k++ {

			//show progress of reading series

			for _, ser := range res[k].Series {
				log.Tracef("ROW Result [%d] [%#+v]", k, ser)

				for _, v := range ser.Values {

					var timestamp time.Time
					var terr error

					switch t := v[0].(type) {
					case string:
						timestamp, terr = StrUnixNano2Time(t)
						if terr != nil {
							log.Errorf("Error processing timestamp skipping point %d for measurements %s", k, ser.Name)
							continue
						}
					case int64:
						timestamp, terr = UnixNano2Time(t)
						if terr != nil {
							log.Errorf("Error processing timestamp skipping point %d for measurements %s", k, ser.Name)
							continue
						}
					case json.Number:
						i, _ := t.Int64()
						timestamp, terr = UnixNano2Time(i)
						if terr != nil {
							log.Errorf("Error processing timestamp skipping point %d for measurements %s", k, ser.Name)
							continue
						}
					default:
						log.Warnf("Timestamp type is %T [%#+v]", t, t)
						continue
					}

					field := make(map[string]interface{})
					l := len(v)
					for i := 1; i < l; i++ {
						val := v[i]
						if val != nil {
							switch vt := val.(type) {
							case json.Number:
								tp := fieldmap[ser.Columns[i]]
								switch tp {
								case "float":
									conv, err := vt.Float64()
									if err != nil {
										log.Errorf("Error on parse field %s data %#+v %T :%s", ser.Columns[i], val, vt, err)
									}
									field[ser.Columns[i]] = conv
								case "integer":
									conv, err := vt.Int64()
									if err != nil {
										log.Errorf("Error on parse field %s data %#+v %T :%s", ser.Columns[i], val, vt, err)
									}
									field[ser.Columns[i]] = conv
								case "boolean":
									fallthrough
								case "string":
									conv := vt.String()
									field[ser.Columns[i]] = conv
								default:
									log.Warnf("Unhandled type %s in field %s measuerment %s", tp, ser.Columns[i], ser.Name)
								}
							case string, bool, int64, float64:
								field[ser.Columns[i]] = v[i]
							default:
								//Supposed to be ok
								log.Warnf("Error unknown type %T on field %s don't know about type %T! value %#+v \n", vt, ser.Columns[i], vt)
								field[ser.Columns[i]] = v[i]
							}

						}
					}
					log.Tracef("POINT TIME  [%s] - NOW[%s] | MEAS: %s | TAGS: %#+v | FIELDS: %#+v| ", timestamp.String(), time.Now().String(), ser.Name, ser.Tags, field)
					point, err := client.NewPoint(ser.Name, ser.Tags, field, timestamp)
					if err != nil {
						log.Errorf("Error in set point %s", err)
						continue
					}
					batchpoints.AddPoint(point)
					totalpoints++
				}

			}
		}

	}
	return batchpoints, totalpoints
}

func WriteDB(c client.Client, b client.BatchPoints) {

	err := c.Write(b)
	if err != nil {
		log.Printf("Fail to write to database, error: %s\n", err.Error())
	}
}

func SyncDBFull(src *InfluxMonitor, dst *InfluxMonitor, db string, rp *RetPol, dbschema *InfluxSchDb, chunk time.Duration, maxret time.Duration) error {

	var eEpoch, sEpoch time.Time

	var hLength int64
	var MaxLength int64
	var chunkSecond int64

	MaxLength = int64(maxret/chunk) + 1

	chunkSecond = int64(chunk.Seconds())

	eEpoch = time.Now()

	if rp.Duration != 0 {

		sEpoch = eEpoch.Add(-rp.Duration)

		duration := time.Since(sEpoch)

		hLength = int64(duration/chunk) + 1

		if hLength > MaxLength {
			hLength = MaxLength
		}

	} else {
		hLength = MaxLength
	}

	var i int64
	for i = 0; i < hLength; i++ {
		s := time.Now()
		//sync from newer to older data
		endsec := eEpoch.Unix() - (i * chunkSecond)
		startsec := eEpoch.Unix() - ((i + 1) * chunkSecond)
		var totalpoints int64
		totalpoints = 0
		for m, sch := range dbschema.Ftypes {

			log.Debugf("Processing measurement %s with schema #%+v", m, sch)

			//write datas of every hour

			log.Debugf("processing Database %s Measurement %s from %d to %d", db, m, startsec, endsec)
			getvalues := fmt.Sprintf("select * from  \"%v\" where time  > %vs and time < %vs group by *", m, startsec, endsec)
			batchpoints, np := ReadDB(src.cli, db, rp.Name, db, rp.Name, getvalues, sch)
			totalpoints += np
			log.Debugf("processed %d points", np)
			WriteDB(dst.cli, batchpoints)

		}
		elapsed := time.Since(s)
		log.Infof("Processed Chunk [%d] from [%d][%s] to [%d][%s] (%d) Points Took [%s]", i, startsec, time.Unix(startsec, 0).String(), endsec, time.Unix(endsec, 0).String(), totalpoints, elapsed.String())

	}

	log.Printf("Copy data from %s[%s|%s] to %s[%s|%s] has done!\n", src.cfg.Name, db, rp.Name, dst.cfg.Name, db, rp.Name)
	return nil
}

func SyncDBRP(src *InfluxMonitor, dst *InfluxMonitor, db string, rp *RetPol, sEpoch time.Time, eEpoch time.Time, dbschema *InfluxSchDb, chunk time.Duration, maxret time.Duration) error {

	if dbschema == nil {
		err := fmt.Errorf("DBSChema for DB %s is null", db)
		log.Errorf("%s", err.Error())
		return err
	}

	var hLength int64
	var MaxLength int64
	var chunkSecond int64

	duration := eEpoch.Sub(sEpoch)

	hLength = int64(duration/chunk) + 1

	MaxLength = int64(maxret/chunk) + 1

	if hLength > MaxLength {
		hLength = MaxLength
	}

	chunkSecond = int64(chunk.Seconds())

	var i int64
	for i = 0; i < hLength; i++ {
		s := time.Now()
		//sync from newer to older data
		endsec := eEpoch.Unix() - (i * chunkSecond)
		startsec := eEpoch.Unix() - ((i + 1) * chunkSecond)
		var totalpoints int64
		totalpoints = 0
		for m, sch := range dbschema.Ftypes {
			log.Debugf("Processing measurement %s from DB  %s", m, db)
			log.Tracef("Processing measurement %s with schema #%+v", m, sch)

			//write datas of every hour

			log.Debugf("processing Database %s Measurement %s from %d to %d", db, m, startsec, endsec)
			getvalues := fmt.Sprintf("select * from  \"%v\" where time  > %vs and time < %vs group by *", m, startsec, endsec)
			batchpoints, np := ReadDB(src.cli, db, rp.Name, db, rp.Name, getvalues, sch)
			totalpoints += np
			log.Debugf("processed %d points", np)
			WriteDB(dst.cli, batchpoints)

		}

		elapsed := time.Since(s)
		log.Infof("Processed Chunk [%d] from [%d][%s] to [%d][%s] (%d) Points Took [%s]", i, startsec, time.Unix(startsec, 0).String(), endsec, time.Unix(endsec, 0).String(), totalpoints, elapsed.String())

	}

	log.Printf("Copy data from %s[%s|%s] to %s[%s|%s] has done!\n", src.cfg.Name, db, rp.Name, dst.cfg.Name, db, rp.Name)

	return nil
}

/*
func SyncDBs(src *InfluxMonitor, dst *InfluxMonitor, stime time.Time, etime time.Time, schema []*InfluxSchDb) error {

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

	sinceTime, errSin := time.Parse(template, "1970-01-01 00:00:00")
	if errSin != nil {
		log.Println("Fail to parse sinceTime")
	}

	sEpoch := stime.Sub(sinceTime)
	eEpoch := etime.Sub(sinceTime)

	hLength := int64(eEpoch.Hours()-sEpoch.Hours()) + 1

	//The datas which can be inputed is less than a year
	if hLength > 8760 {
		hLength = 8760
	}
	dbarray, _ := GetDataBases(scon)

	for _, db := range dbarray {
		log.Infof("Processing Database %s", db)

		var dbschema *InfluxSchDb

		for _, k := range schema {
			if k.Name == db {
				dbschema = k
			}
		}

		// Get Retention policies
		rps, err := GetRetentionPolicies(scon, db)
		if err != nil {
			log.Errorf("Error on get Retention Policies on Database %s (%s) %s : Error: %s", db, src.cfg.Name, src.cfg.Location, err)
			continue
		}

		for _, rp := range rps {

			measurements := GetMeasurements(scon, db)

			var i int64
			for i = 0; i < hLength; i++ {

				startsec := int64(sEpoch.Seconds() + float64(i*3600))
				endsec := int64(sEpoch.Seconds() + float64((i+1)*3600))
				var totalpoints int64
				totalpoints = 0
				for _, m := range measurements {

					fieldmap := dbschema.Ftypes[m]

					log.Debugf("Processing measurement %s with schema #%+v", m, fieldmap)

					//write datas of every hour

					log.Debugf("processing Database %s Measurement %s from %d to %d", db, m, startsec, endsec)
					getvalues := fmt.Sprintf("select * from  \"%v\" where time  > %vs and time < %vs group by *", m, startsec, endsec)
					batchpoints, np := ReadDB(scon, db, rp.Name, db, rp.Name, getvalues, fieldmap)
					totalpoints += np
					log.Debugf("processed %d points", np)
					WriteDB(dcon, batchpoints)

					time.Sleep(time.Millisecond)
				}
				log.Infof("Processed Chunk [%d] from [%d][%s] to [%d][%s] (%d) Points", i, startsec, time.Unix(startsec, 0).String(), endsec, time.Unix(endsec, 0).String(), totalpoints)

				time.Sleep(time.Millisecond)

			}
			log.Printf("Copy data from %s[%s|%s] to %s[%s|%s] has done!\n", src.cfg.Name, db, rp.Name, dst.cfg.Name, db, rp.Name)

		}

	}

	log.Printf("Copy data from %s to %s has done!\n", src.cfg.Name, dst.cfg.Name)
	return nil
}*/
