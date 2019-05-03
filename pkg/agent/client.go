package agent

import (
	"fmt"
	"strconv"

	"encoding/json"
	"sync/atomic"
	"time"

	"github.com/gammazero/workerpool"
	"github.com/influxdata/influxdb1-client/v2"
	"github.com/toni-moreno/syncflux/pkg/agent/try"
)

type RetPol struct {
	Name               string
	Duration           time.Duration
	ShardGroupDuration time.Duration
	NReplicas          int64
	Def                bool
	Measurements       map[string]*MeasurementSch
}

func (rp *RetPol) GetFirstLastTime(max time.Duration) (time.Time, time.Time) {
	if rp.Duration == 0 {
		last := time.Now()
		return last.Add(-max), last
	}
	last := time.Now()
	return last.Add(-rp.Duration), last
}

func (rp *RetPol) GetFirstTime(max time.Duration) time.Time {
	if rp.Duration == 0 {
		return time.Now().Add(-max)
	}
	return time.Now().Add(-rp.Duration)
}

func DBclient(location string, user string, pass string) (client.Client, error) {

	info := client.HTTPConfig{
		Addr:      location,
		Username:  user,
		Password:  pass,
		UserAgent: "syncflux-agent",
		Timeout:   120 * time.Second,
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
	log.Debugf("Happy as a hippo! %v, %s", dur, ver)

	return con, nil
}

func CreateDB(con client.Client, db string, rp *RetPol) error {

	if db == "_internal" {
		return nil
	}

	cmd := "CREATE DATABASE " + db + " WITH DURATION " + rp.Duration.String() + " REPLICATION " + strconv.FormatInt(rp.NReplicas, 10) + " SHARD DURATION " + rp.ShardGroupDuration.String() + " NAME " + "\"" + rp.Name + "\""

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

	cmd := "CREATE RETENTION POLICY \"" + rp.Name + "\" ON " + db + " DURATION " + rp.Duration.String() + " REPLICATION " + strconv.FormatInt(rp.NReplicas, 10) + " SHARD DURATION " + rp.ShardGroupDuration.String()
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

func SetDefaultRP(con client.Client, db string, rp *RetPol) error {

	cmd := "ALTER RETENTION POLICY \"" + rp.Name + "\" ON " + db + " DEFAULT"

	log.Debugf("Influx QUERY: %s", cmd)
	q := client.Query{
		Command: cmd,
	}
	response, err4 := con.Query(q)
	if err4 == nil && response.Error() == nil {
		log.Debugf("Altern Policy Creation response %#+v", response)
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

func GetFields(c client.Client, sdb string, meas string, rp string) map[string]*FieldSch {

	fields := make(map[string]*FieldSch)

	cmd := "show field keys from " + meas
	//get measurements from database
	q := client.Query{
		Command:         cmd,
		Database:        sdb,
		RetentionPolicy: rp,
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
			fields[fieldname] = &FieldSch{Name: fieldname, Type: fieldtype}
			log.Debugf("Detected Field [%s] type [%s] on measurement [%s]", fieldname, fieldtype, meas)
		}

	}
	return fields
}

func GetMeasurements(c client.Client, sdb string, rp string, mesafilter string) []*MeasurementSch {

	cmd := "show measurements"
	//get measurements from database
	q := client.Query{
		Command:         cmd,
		Database:        sdb,
		RetentionPolicy: rp,
	}

	var measurements []*MeasurementSch

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
			measurements = append(measurements, &MeasurementSch{Name: measurement, Fields: nil})

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

func ReadDB(c client.Client, sdb, srp, ddb, drp, cmd string, fieldmap map[string]*FieldSch) (client.BatchPoints, int64, error) {
	var totalpoints int64
	RWMaxRetries := MainConfig.General.RWMaxRetries
	RWRetryDelay := MainConfig.General.RWRetryDelay
	totalpoints = 0

	//param := make(map[string]interface{})
	//param["wait_for_leader"] = "2000s"

	q := client.Query{
		Command:         cmd,
		Database:        sdb,
		RetentionPolicy: srp,
		Precision:       "ns",
		Chunked:         true,
		ChunkSize:       10000,
		//Parameters:      param,
	}

	bpcfg := client.BatchPointsConfig{
		Database:        ddb,
		RetentionPolicy: drp,
		Precision:       "ns",
	}

	batchpoints, err := client.NewBatchPoints(bpcfg)
	if err != nil {
		log.Error("Error on create BatchPoints: %s", err)
		return batchpoints, 0, err
	}
	var response *client.Response

	//Retry query if some error happens

	err = try.Do(func(attempt int) (bool, error) {
		var qerr error
		s := time.Now()
		response, qerr = c.Query(q)
		elapsed := time.Since(s)
		log.Debugf("Query [%s] took %s ", cmd, elapsed.String())
		if qerr != nil {
			log.Warnf("Fail to get response from query in attempt %d / read database error: %s", attempt, qerr)
			log.Warnf("Trying again... in %s sec", RWRetryDelay.String())
			time.Sleep(RWRetryDelay) // wait a minute
		}

		return attempt < RWMaxRetries, qerr

	})
	if err != nil {
		log.Errorf("Max Retries (%d) exceeded on read Data: Last error %s ", RWMaxRetries, err)
		return nil, 0, err
	}

	res := response.Results
	if len(res) == 0 {
		log.Warnf("The response of query [%s] is void, read database error!\n", cmd)
	} else {
		resLength := len(res)
		for k := 0; k < resLength; k++ {

			//show progress of reading series
			log.Tracef("Reading %d Series for db %s", len(res[k].Series), sdb)
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
								switch tp.Type {
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
	return batchpoints, totalpoints, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func BpSplit(bp client.BatchPoints, splitnum int) []client.BatchPoints {

	points := bp.Points()
	len := len(points)
	lim := (len / splitnum) + 1
	ret := make([]client.BatchPoints, 0, lim)

	if len < splitnum {
		ret = append(ret, bp)
		return ret
	}

	for i := 0; i < lim; i++ {

		bpcfg := client.BatchPointsConfig{
			Database:        bp.Database(),
			RetentionPolicy: bp.RetentionPolicy(),
			Precision:       bp.Precision(),
		}

		newbp, err := client.NewBatchPoints(bpcfg)
		if err != nil {
			log.Error("Error on create BatchPoints: %s", err)
			return nil
		}
		pointchunk := make([]*client.Point, splitnum)
		init := i * splitnum
		end := min((i+1)*splitnum, len)
		log.Debugf("Splitting %s batchpoints into 50000  points chunks from %d to %d ", len, init, end)
		copy(points[init:end], pointchunk)
		newbp.AddPoints(pointchunk)

		ret = append(ret, newbp)

	}
	return ret
}

func WriteDB(c client.Client, bp client.BatchPoints) {

	RWMaxRetries := MainConfig.General.RWMaxRetries
	RWRetryDelay := MainConfig.General.RWRetryDelay

	//spliting writtes max of 50k points
	sbp := BpSplit(bp, 50000)

	for k, b := range sbp {
		err := try.Do(func(attempt int) (bool, error) {
			s := time.Now()
			err := c.Write(b)
			elapsed := time.Since(s)
			log.Debugf("Write attempt [%d] took %s ", attempt, elapsed.String())
			if err != nil {
				log.Warnf("Fail to write batchpoints to write database error Trying again... in %s : Error %s  ", RWRetryDelay.String(), err)
				time.Sleep(RWRetryDelay) // wait a minute
			}
			return attempt < RWMaxRetries, err
		})
		if err != nil {
			log.Errorf("Max Retries  ( %d ) exceeded on  write to database in write chunk %d, Last error: %s", RWMaxRetries, k, err)
		}
	}

}

func SyncDBRP(src *InfluxMonitor, dst *InfluxMonitor, sdb string, ddb string, srp *RetPol, drp *RetPol, sEpoch time.Time, eEpoch time.Time, dbschema *InfluxSchDb, chunk time.Duration, maxret time.Duration) error {

	if dbschema == nil {
		err := fmt.Errorf("DBSChema for DB %s is null", sdb)
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

	log.Debugf("SYNC-DB-RP[%s|%s] From:%s To:%s | Duration: %s || #chunks: %d  | chunk Duration %s ", sdb, srp, sEpoch.String(), eEpoch.String(), duration.String(), hLength, chunk.String())

	var i int64
	var dbpoints int64
	dbs := time.Now()

	for i = 0; i < hLength; i++ {
		wp := workerpool.New(MainConfig.General.NumWorkers)
		defer wp.Stop()
		chs := time.Now()
		//sync from newer to older data
		endsec := eEpoch.Unix() - (i * chunkSecond)
		startsec := eEpoch.Unix() - ((i + 1) * chunkSecond)
		var totalpoints int64
		totalpoints = 0

		for m, sch := range srp.Measurements {
			m := m
			sch := sch

			//add to the worker pool
			wp.Submit(func() {
				log.Tracef("Processing measurement %s with schema #%+v", m, sch)
				log.Debugf("processing Database %s Measurement %s from %d to %d", sdb, m, startsec, endsec)
				getvalues := fmt.Sprintf("select * from  \"%v\" where time  > %vs and time < %vs group by *", m, startsec, endsec)
				batchpoints, np, err := ReadDB(src.cli, sdb, srp.Name, ddb, drp.Name, getvalues, sch.Fields)
				if err != nil {
					log.Errorf("error in read %s", err)
					return
					//return err
				}
				atomic.AddInt64(&totalpoints, np)
				//totalpoints += np
				log.Debugf("processed %d points", np)
				WriteDB(dst.cli, batchpoints)
			})
			//write datas of every hour
		}
		wp.StopWait()
		chunkElapsed := time.Since(chs)
		dbpoints += totalpoints
		log.Infof("Processed Chunk [%d/%d](%d%%) from [%d][%s] to [%d][%s] (%d) Points Took [%s]", i+1, hLength, 100*(i+1)/hLength, startsec, time.Unix(startsec, 0).String(), endsec, time.Unix(endsec, 0).String(), totalpoints, chunkElapsed.String())

	}
	dbElapsed := time.Since(dbs)
	log.Printf("Processed DB data from %s[%s|%s] to %s[%s|%s] has done  #Points (%d)  Took [%s] !\n", src.cfg.Name, sdb, srp.Name, dst.cfg.Name, ddb, drp.Name, dbpoints, dbElapsed.String())

	return nil
}
