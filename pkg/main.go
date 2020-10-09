package main

import (
	"flag"
	"fmt"
	"os/signal"
	"syscall"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"

	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/toni-moreno/syncflux/pkg/agent"
	"github.com/toni-moreno/syncflux/pkg/config"
	"github.com/toni-moreno/syncflux/pkg/webui"
)

var (
	log  = logrus.New()
	quit = make(chan struct{})
	//startTime  = time.Now()
	getversion bool
	httpPort   = "0.0.0.0:4090"
	appdir     = os.Getenv("PWD")
	//homeDir    string
	pidFile string
	logDir  = filepath.Join(appdir, "log")
	logMode = "console"
	confDir = filepath.Join(appdir, "conf")
	//dataDir    = confDir
	configFile = filepath.Join(confDir, "syncflux.toml")
	//
	action       = "hamonitor"
	master       string
	slave        string
	actiondb     = ".*"
	actionrp     = ".*"
	actionmeas   = ".*"
	newdb        string
	newrp        string
	starttimestr string
	starttime    = time.Now().Add(-3600 * 24)
	endtimestr   string
	endtime      = time.Now()
	copyorder    = "reverse"
	fulltime     bool
	chunktimestr string
	//log level

	loginfo  bool
	logdebug bool
	logtrace bool
)

func writePIDFile() {
	if pidFile == "" {
		return
	}

	// Ensure the required directory structure exists.
	err := os.MkdirAll(filepath.Dir(pidFile), 0700)
	if err != nil {
		log.Fatal(3, "Failed to verify pid directory", err)
	}

	// Retrieve the PID and write it.
	pid := strconv.Itoa(os.Getpid())
	if err := ioutil.WriteFile(pidFile, []byte(pid), 0644); err != nil {
		log.Fatal(3, "Failed to write pidfile", err)
	}
}

func flags() *flag.FlagSet {
	var f flag.FlagSet
	f.BoolVar(&getversion, "version", getversion, "display the version")
	//--------------------------------------------------------------
	f.StringVar(&action, "action", action, "hamonitor(default),copy,fullcopy,replicaschema")
	f.StringVar(&master, "master", master, "choose master ID from all those in the config file where to get data (override the master-db parameter in the config file)")
	f.StringVar(&slave, "slave", slave, "choose master ID from all those in the config file where to write data (override the slave-db parameter in the config file)")
	f.StringVar(&actiondb, "db", actiondb, "set the db where to play")
	f.StringVar(&actionrp, "rp", actionrp, "set the rp where to play")
	f.StringVar(&actionmeas, "meas", actionmeas, "set the meas where to play")
	f.StringVar(&newdb, "newdb", newdb, "set the db to work on")
	f.StringVar(&newrp, "newrp", newrp, "set the rp to work on")
	f.StringVar(&chunktimestr, "chunk", chunktimestr, "set RW chuck periods as in the data-chuck-duration config param")
	f.StringVar(&starttimestr, "start", starttimestr, "set the starttime to do action (no valid in hamonitor) default now-24h")
	f.StringVar(&endtimestr, "end", endtimestr, "set the endtime do action (no valid in hamonitor) default now")
	f.StringVar(&copyorder, "copyorder", copyorder, "backward (most to least recent, default), forward (least to most recent)")
	f.BoolVar(&fulltime, "full", fulltime, "copy full database or now()- max-retention-interval if greater retention policy")
	//  -v = Info
	//  -vv =  debug
	//  -vvv = trace
	f.BoolVar(&loginfo, "v", loginfo, "set log level to Info")
	f.BoolVar(&logdebug, "vv", logdebug, "set log level to Debug")
	f.BoolVar(&logtrace, "vvv", logtrace, "set log level to Trace")

	//--------------------------------------------------------------
	f.StringVar(&configFile, "config", configFile, "config file")
	f.StringVar(&logMode, "logmode", logDir, "log mode [console/file] default console")
	f.StringVar(&logDir, "logs", logDir, "log directory (only apply if action=hamonitor and logmode=file)")
	//f.StringVar(&homeDir, "home", homeDir, "home directory")
	//f.StringVar(&dataDir, "data", dataDir, "Data directory")
	f.StringVar(&pidFile, "pidfile", pidFile, "path to pid file")
	//---------------------------------------------------------------
	f.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
		f.VisitAll(func(flag *flag.Flag) {
			format := "%10s: %s\n"
			fmt.Fprintf(os.Stderr, format, "-"+flag.Name, flag.Usage)
		})
		fmt.Fprintf(os.Stderr, "\nAll settings can be set in config file: %s\n", configFile)
		os.Exit(1)

	}
	return &f
}

func init() {
	//Log format
	customFormatter := new(logrus.TextFormatter)
	customFormatter.TimestampFormat = "2006-01-02 15:04:05"
	log.Formatter = customFormatter
	customFormatter.FullTimestamp = true

	// parse first time to see if config file is being specified
	f := flags()
	f.Parse(os.Args[1:])

	if getversion {
		t, _ := strconv.ParseInt(agent.BuildStamp, 10, 64)
		fmt.Printf("syncflux v%s (git: %s ) built at [%s]\n", agent.Version, agent.Commit, time.Unix(t, 0).Format("2006-01-02 15:04:05"))
		os.Exit(0)
	}

	// now load up config settings
	if _, err := os.Stat(configFile); err == nil {
		viper.SetConfigFile(configFile)
		confDir = filepath.Dir(configFile)
	} else {
		viper.SetConfigName("syncflux")
		viper.AddConfigPath("/etc/syncflux/")
		viper.AddConfigPath("/opt/syncflux/conf/")
		viper.AddConfigPath("./conf/")
		viper.AddConfigPath(".")
	}
	err := viper.ReadInConfig()
	if err != nil {
		log.Errorf("Fatal error config file: %s \n", err)
		os.Exit(1)
	}
	err = viper.Unmarshal(&agent.MainConfig)
	if err != nil {
		log.Errorf("Fatal error config file: %s \n", err)
		os.Exit(1)
	}
	cfg := &agent.MainConfig

	log.Infof("CFG :%+v", cfg)

	if len(logDir) == 0 {
		logDir = cfg.General.LogDir
		log.Infof("Set logdir %s from Command Line parameter", logDir)
	}

	//default output to console
	log.Out = os.Stdout

	if action == "hamonitor" {
		if logMode == "file" {
			os.MkdirAll(logDir, 0755)
			//Log output
			file, _ := os.OpenFile(logDir+"/syncflux.log", os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
			log.Out = file

		}
	}

	if len(master) == 0 {
		master = cfg.General.MasterDB
		log.Infof("Set Master DB %s from Command Line parameters", master)
	}

	if len(slave) == 0 {
		slave = cfg.General.SlaveDB
		log.Infof("Set Slave DB %s from Command Line parameters", slave)
	}

	if len(cfg.General.LogLevel) > 0 && action == "hamonitor" {
		l, _ := logrus.ParseLevel(cfg.General.LogLevel)
		log.Level = l
		log.Infof("Set log level to  %s from Config File", cfg.General.LogLevel)
	}
	if action != "hamonitor" {
		switch {
		case loginfo:
			log.Level = logrus.InfoLevel
		case logdebug:
			log.Level = logrus.DebugLevel
		case logtrace:
			log.Level = logrus.TraceLevel
		default:
			log.Level = logrus.WarnLevel

		}
	}
	if cfg.General.RWMaxRetries == 0 {
		cfg.General.RWMaxRetries = 5
	}

	if cfg.General.RWRetryDelay == 0 {
		cfg.General.RWRetryDelay = 10 * time.Second
	}
	if cfg.General.MaxPointsOnSingleWrite == 0 {
		cfg.General.MaxPointsOnSingleWrite = 10000
	}

	//needed to create SQLDB when SQLite and debug log
	config.SetLogger(log)
	config.SetLogDir(logDir)
	//config.SetDirs(dataDir, logDir, confDir)

	webui.SetLogger(log)
	webui.SetLogDir(logDir)
	webui.SetConfDir(confDir)
	agent.SetLogger(log)

	//
	log.Infof("Set Default directories : \n   - Exec: %s\n   - Config: %s\n   -Logs: %s\n", appdir, confDir, logDir)
}

func main() {

	defer func() {
		//errorLog.Close()
	}()
	writePIDFile()
	//Init BD config
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	go func() {
		select {
		case sig := <-c:
			switch sig {
			case syscall.SIGTERM:
				log.Infof("Received TERM signal")
				agent.End()
				log.Infof("Exiting for requested user SIGTERM")
				os.Exit(1)
			case syscall.SIGINT:
				log.Infof("Received INT signal")
				agent.End()
				log.Infof("Exiting for requested user SIGINT")
				os.Exit(1)
			case syscall.SIGHUP:
				log.Infof("Received HUP signal")
				agent.ReloadConf()
			}

		}
	}()

	var err error

	//parse input data

	if len(endtimestr) > 0 {
		endtime, err = parseInputTime(endtimestr)
		if err != nil {
			fmt.Printf("ERROR in Parse End Time (%s) : Error %s", endtimestr, err)
			os.Exit(1)
		}
	}

	if len(starttimestr) > 0 {
		starttime, err = parseInputTime(starttimestr)
		if err != nil {
			fmt.Printf("ERROR in Parse End Time (%s) : Error %s", starttimestr, err)
			os.Exit(1)
		}
	}
	if len(chunktimestr) > 0 {
		dur, err := time.ParseDuration(chunktimestr)
		if err != nil {
			fmt.Printf("ERROR in Parse Chunk Duration (%s) :  Error %s", chunktimestr, err)
			os.Exit(1)
		}
		agent.MainConfig.General.DataChunkDuration = dur
	}

	switch action {
	case "hamonitor":
		agent.HAMonitorStart(master, slave, copyorder)
		webui.WebServer("", httpPort, &agent.MainConfig.HTTP, agent.MainConfig.General.InstanceID)
	case "copy":
		agent.Copy(master, slave, actiondb, newdb, actionrp, newrp, actionmeas, starttime, endtime, fulltime, copyorder)
	case "move":
	case "replicaschema":
		agent.ReplSch(master, slave, actiondb, newdb, actionrp, newrp, actionmeas)
	case "fullcopy":
		agent.SchCopy(master, slave, actiondb, newdb, actionrp, newrp, actionmeas, starttime, endtime, fulltime, copyorder)
	default:
		fmt.Printf("Unknown action: %s", action)
	}

}
