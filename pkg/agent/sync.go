package agent

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/gammazero/workerpool"
)

type ChunkReport struct {
	Num             int64
	Total           int64
	TimeExec        time.Time
	TimeStart       int64
	TimeEnd         int64
	ReadErrors      uint64
	WriteErrors     uint64
	Errors          []string
	ProcessedPoints int64
	TimeTaken       time.Duration
}

func (cr *ChunkReport) Log() {
	percent := 100 * (cr.Num + 1) / cr.Total
	log.Infof("Processed Chunk [%d/%d](%d%%) from [%d][%s] to [%d][%s] (%d) Points Took [%s] ERRORS[R:%d|W:%d]",
		cr.Num,
		cr.Total,
		percent,
		cr.TimeStart,
		time.Unix(cr.TimeStart, 0).String(),
		cr.TimeEnd,
		time.Unix(cr.TimeEnd, 0).String(),
		cr.ProcessedPoints,
		cr.TimeTaken.String(),
		cr.ReadErrors,
		cr.WriteErrors)
}

type SyncReport struct {
	SrcSrv       string
	DstSrv       string
	SrcDB        string
	DstDB        string
	SrcRP        string
	DstRP        string
	TotalPoints  int64
	TotalElapsed time.Duration
	Start        time.Time
	End          time.Time
	ChunkReport  []*ChunkReport
	BadChunks    []*ChunkReport
}

func (sr *SyncReport) Log() {

	log.Printf("Processed DB data from %s[%s|%s] to %s[%s|%s] has done  #Points (%d)  Took [%s] ERRORS [%d]!\n",
		sr.SrcSrv,
		sr.SrcDB,
		sr.SrcRP,
		sr.DstSrv,
		sr.DstDB,
		sr.DstRP,
		sr.TotalPoints,
		sr.TotalElapsed.String(),
		len(sr.BadChunks))
}

func Sync(src *InfluxMonitor, dst *InfluxMonitor, sdb string, ddb string, srp *RetPol, drp *RetPol, sEpoch time.Time, eEpoch time.Time, dbschema *InfluxSchDb, chunk time.Duration, maxret time.Duration) (*SyncReport, error) {

	if dbschema == nil {
		err := fmt.Errorf("DBSChema for DB %s is null", sdb)
		log.Errorf("%s", err.Error())
		return nil, err
	}

	Report := &SyncReport{
		SrcSrv: src.cfg.Name,
		DstSrv: dst.cfg.Name,
		SrcDB:  sdb,
		DstDB:  ddb,
		SrcRP:  srp.Name,
		DstRP:  drp.Name,
		Start:  sEpoch,
		End:    eEpoch,
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

	chuckReport := make([]*ChunkReport, hLength)
	badChunkReport := make([]*ChunkReport, 0)

	log.Debugf("SYNC-DB-RP[%s|%s] From:%s To:%s | Duration: %s || #chunks: %d  | chunk Duration %s ", sdb, srp.Name, sEpoch.String(), eEpoch.String(), duration.String(), hLength, chunk.String())
	log.Tracef("SYNC-DB-RP Schema: %s  ", srp)

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
		log.Debugf("Detected %d measurements on %s|%s", len(srp.Measurements), sdb, srp.Name)
		//--------
		var readErrors uint64
		var writeErrors uint64
		//--------
		for m, sch := range srp.Measurements {
			m := m
			sch := sch

			//add to the worker pool
			wp.Submit(func() {
				log.Tracef("Processing measurement %s with schema #%+v", m, sch)
				log.Debugf("processing Database %s Measurement %s from %d to %d", sdb, m, startsec, endsec)
				getvalues := fmt.Sprintf("select * from  \"%v\" where time  > %vs and time < %vs group by *", m, startsec, endsec)
				batchpoints, np, rerr := ReadDB(src.cli, sdb, srp.Name, ddb, drp.Name, getvalues, sch.Fields)
				if rerr != nil {
					atomic.AddUint64(&readErrors, 1)
					log.Errorf("error in read DB %s | Measurement %s | ERR: %s", sdb, m, rerr)
					return
					//return err
				}
				atomic.AddInt64(&totalpoints, np)
				//totalpoints += np
				log.Debugf("processed %d points", np)
				werr := WriteDB(dst.cli, batchpoints)
				if werr != nil {
					atomic.AddUint64(&writeErrors, 1)
					log.Errorf("error in write DB %s | Measurement %s | ERR: %s", ddb, m, werr)
					return
					//return err
				}
			})
			//write datas of every hour
		}
		wp.StopWait()
		chunkElapsed := time.Since(chs)
		dbpoints += totalpoints
		chrep := &ChunkReport{
			Num:             i + 1,
			Total:           hLength,
			TimeExec:        time.Now(),
			TimeStart:       startsec,
			TimeEnd:         endsec,
			ReadErrors:      readErrors,
			WriteErrors:     writeErrors,
			ProcessedPoints: totalpoints,
			TimeTaken:       chunkElapsed,
		}

		chrep.Log()
		chuckReport = append(chuckReport, chrep)
		if readErrors+writeErrors > 0 {
			badChunkReport = append(badChunkReport, chrep)
		}
		//log.Infof("Processed Chunk [%d/%d](%d%%) from [%d][%s] to [%d][%s] (%d) Points Took [%s] ERRORS[R:%d|W:%d]",i+1, hLength, 100*(i+1)/hLength, startsec, time.Unix(startsec, 0).String(), endsec, time.Unix(endsec, 0).String(), totalpoints, chunkElapsed.String(), readErrors, writeErrors)

	}
	Report.TotalElapsed = time.Since(dbs)
	Report.TotalPoints = dbpoints
	Report.ChunkReport = chuckReport
	Report.BadChunks = badChunkReport
	Report.Log()
	//log.Printf("Processed DB data from %s[%s|%s] to %s[%s|%s] has done  #Points (%d)  Took [%s] !\n", src.cfg.Name, sdb, srp.Name, dst.cfg.Name, ddb, drp.Name, dbpoints, dbElapsed.String())

	return Report, nil
}

func SyncDBRP(src *InfluxMonitor, dst *InfluxMonitor, sdb string, ddb string, srp *RetPol, drp *RetPol, sEpoch time.Time, eEpoch time.Time, dbschema *InfluxSchDb, chunk time.Duration, maxret time.Duration) error {

	_, err := Sync(src, dst, sdb, ddb, srp, drp, sEpoch, eEpoch, dbschema, chunk, maxret)
	return err
}
