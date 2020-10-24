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

func (cr *ChunkReport) format() string {
	percent := 100 * cr.Num / cr.Total
	return fmt.Sprintf("[%d/%d](%d%%) from [%d][%s] to [%d][%s] (%d) Points Took [%s] ERRORS[R:%d|W:%d]",
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

func (cr *ChunkReport) Log(prefix string) {

	log.Infof("%s %s", prefix, cr.format())
}

func (cr *ChunkReport) Warn(prefix string) {

	log.Warnf("%s %s", prefix, cr.format())
}

func (cr *ChunkReport) Error(prefix string) {

	log.Warnf("%s %s", prefix, cr.format())
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

func (sr *SyncReport) Log(prefix string) {

	log.Printf("%s data from %s[%s|%s] to %s[%s|%s] has done  #Points (%d)  Took [%s] ERRORS [%d]!\n",
		prefix,
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

func (sr *SyncReport) RWErrors() (uint64, uint64, uint64) {
	var readErrors, writeErrors uint64
	for _, b := range sr.BadChunks {
		readErrors += b.ReadErrors
		writeErrors += b.WriteErrors

	}
	return readErrors, writeErrors, readErrors + writeErrors
}

func Sync(src *InfluxMonitor, dst *InfluxMonitor, sdb string, ddb string, srp *RetPol, drp *RetPol, sEpoch time.Time, eEpoch time.Time, dbschema *InfluxSchDb, chunk time.Duration, maxret time.Duration, copyorder string) *SyncReport {

	if dbschema == nil {
		err := fmt.Errorf("DBSChema for DB %s is null", sdb)
		log.Errorf("%s", err.Error())
		return nil
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
	var startsec, endsec int64
	dbs := time.Now()

	for i = 0; i < hLength; i++ {
		wp := workerpool.New(MainConfig.General.NumWorkers)
		defer wp.Stop()
		chs := time.Now()

		if copyorder == "forward" {
			//sync from older to newer data
			startsec = eEpoch.Unix() - ((hLength - i) * chunkSecond)
			endsec = eEpoch.Unix() - ((hLength - i - 1) * chunkSecond)
		} else {
			//sync from newer to older data
			endsec = eEpoch.Unix() - (i * chunkSecond)
			startsec = eEpoch.Unix() - ((i + 1) * chunkSecond)
		}

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

		chrep.Log("Processed Chunk")
		chuckReport = append(chuckReport, chrep)
		if readErrors+writeErrors > 0 {
			badChunkReport = append(badChunkReport, chrep)
		}

	}

	Report.TotalElapsed = time.Since(dbs)
	Report.TotalPoints = dbpoints
	Report.ChunkReport = chuckReport
	Report.BadChunks = badChunkReport
	Report.Log("Processed DB")

	return Report
}

func SyncDBRP(src *InfluxMonitor, dst *InfluxMonitor, sdb string, ddb string, srp *RetPol, drp *RetPol, sEpoch time.Time, eEpoch time.Time, dbschema *InfluxSchDb, chunk time.Duration, maxret time.Duration, copyorder string) *SyncReport {

	report := Sync(src, dst, sdb, ddb, srp, drp, sEpoch, eEpoch, dbschema, chunk, maxret, copyorder)
	if len(report.BadChunks) > 0 {
		log.Warnf("Initializing Recovery for %d chunks", len(report.BadChunks))
		newBadChunks := make([]*ChunkReport, 0)
		for _, bc := range report.BadChunks {
			bc.Warn("Recovery for Bad Chunk")
			start := time.Unix(bc.TimeStart, 0)
			end := time.Unix(bc.TimeEnd, 0)

			recoveryrep := Sync(src, dst, sdb, ddb, srp, drp, start, end, dbschema, chunk/10, maxret, copyorder)
			newBadChunks = append(newBadChunks, recoveryrep.BadChunks...)
		}
		report.BadChunks = newBadChunks
	}
	return report
}
