# SyncFlux 

SyncFlux is an Open Source InfluxDB  Data syncronization and replication tool with HTTP API Interface which has as main goal recover lost data from any  handmade HA influxDB 1.X cluster ( made as any simple relay  https://github.com/influxdata/influxdb-relay or our Smart Relay http://github.com/toni-moreno/influxdb-srelay )  


## Intall from precompiled packages

Debian | RedHat |Docker
-------|--------|------
[deb](http://syncflux-rel.s3.amazonaws.com/builds/syncflux_latest_amd64.deb) - [signature](http://syncflux-rel.s3.amazonaws.com/builds/syncflux_latest_amd64.deb.sha1)|[rpm](http://syncflux-rel.s3.amazonaws.com/builds/syncflux-latest-1.x86_64.rpm) - [signature](http://syncflux-rel.s3.amazonaws.com/builds/syncflux-latest-1.x86_64.rpm.sha1)| `docker run -d --name=syncflux_instance00 -p 4090:4090 -v /mylocal/conf:/opt/syncflux/conf -v /mylocal/log:/opt/syncflux/log  tonimoreno/syncflux`

## Run from master

If you want to build a package yourself, or contribute. Here is a guide for how to do that.

### Dependencies

- Go 1.11 

### Get Code

```bash
go get -d github.com/toni-moreno/syncflux/...
```

### Building the backend


```bash
cd $GOPATH/src/github.com/toni-moreno/syncflux
go run build.go build           
```

### Creating minimal package tar.gz

After building frontend and backend you will do

```bash
go run build.go pkg-min-tar
```

### Creating rpm and deb packages

you  will need previously installed the fpm/rpm and deb packaging tools.
After building frontend and backend  you will do.

```bash
go run build.go latest
```

### Running first time
To execute without any configuration you need a minimal config.toml file on the conf directory.

```bash
cp conf/sample.syncflux.toml conf/syncflux.toml
./bin/syncflux [options]
```


### Recompile backend on source change (only for developers)

To rebuild on source change (requires that you executed godep restore)
```bash
go get github.com/Unknwon/bra
bra run  
```
will init a change autodetect webserver with angular-cli (ng serve) and also a autodetect and recompile process with bra for the backend

## Basic Usage

### Set config file


````toml
# -*- toml -*-

# -------GENERAL SECTION ---------
# syncflux could work in several ways, 
# not all General config parameters works on all modes.
#  modes
#  "hamonitor" => enables syncflux as a daemon to sync 
#                2 Influx 1.X OSS db and sync data between them
#                when needed (does active monitoring )
#  "copy" => executes syncflux as a new process to copy data 
#            between master and slave databases
#  "replicashema" => executes syncflux as a new process to create 
#             the database/s and all its related retention policies 
#  "fullcopy" => does database/rp replication and after does a data copy

[General]
 # ------------------------
 # logdir ( only valid on hamonitor action) 
 #  the directory where to place logs 
 #  will place the main log "
 #  

 logdir = "./log"

 # ------------------------
 # loglevel ( valid for all actions ) 
 #  set the log level , valid values are:
 #  fatal,error,warn,info,debug,trace

 loglevel = "debug"

 # -----------------------------
 # sync-mode (only valid on hamonitor action)
 #  NOTE: rigth now only  "onlyslave" (one way sync ) is valied
 #  (planned sync in two ways in the future)

 sync-mode = "onlyslave"

 # ---------------------------
 # master-db choose one of the configured InfluxDB as a SlaveDB
 # this parameter will be override by the command line -master parameter
 
 master-db = "influxdb01"

 # ---------------------------
 # slave-db choose one of the configured InfluxDB as a SlaveDB
 # this parameter will be override by the command line -slave parameter
 
 slave-db = "influxdb02"

 # ------------------------------
 # check-interval
 # the inteval for health cheking for both master and slave databases
 
 check-interval = "10s"

 # ------------------------------
 # min-sync-interval
 # the inteval in which HA monitor will check both are ok and change
 # the state of the cluster if not, making all needed recovery actions

 min-sync-interval = "20s"
 
 # ---------------------------------------------
 # initial-replication
 # tells syncflux if needed some type of replication 
 # on slave database from master database on initialize 
 # (only valid on hamonitor action)
 #
 # none:  no replication
 # schema: database and retention policies will be recreated on the slave database
 # data: data for all retention policies will be replicated 
 #      be carefull: this full data copy could take hours,days.
 # all:  will replicate first the schema and them the full data 

 initial-replication = "none"

 # 
 # monitor-retry-durtion 
 #
 # syncflux only can begin work when master and slave database are both up, 
 # if some of them is down syncflux will retry infinitely each monitor-retry-duration to work.
 monitor-retry-interval = "1m"

 # 
 # data-chuck-duration
 #
 # duration for each small, read  from master -> write to slave, chuck of data
 # smaller chunks of data will use less memory on the syncflux process
 # and also less resources on both master and slave databases
 # greater chunks of data will improve sync speed 

 data-chuck-duration = "60m"

 # 
 #  max-retention-interval
 #
 # for infinite ( or bigger ) retention policies full replication should begin somewhere in the time
 # this parameter set the max retention.
 
 max-retention-interval = "8760h" # 1 year
 

# ---- HTTP API SECTION (Only valid on hamonitor action)
# Enables an HTTP API endpoint to check the cluster health

[http]
 name = "example-http-influxdb"
 bind-addr = "127.0.0.1:4090"
 admin-user = "admin"
 admin-passwd = "admin"
 cookie-id = "mysupercokie"

# ---- INFLUXDB  SECTION
# Sets a list of available DB's that can be used 

````

### Run as a Database replication Tool

Available actions:

- Replicate Schema
- Copy data
- Full copy (replicate schema + copy data)


#### Replicate schema

Allows the user to copy DB schemas from DB1 to DB2. DB schema are DBs and RPs.


___Syntax___
 
```
./bin/syncflux -action replicaschema [-master <master_id>] [-slave <slave_id>] [-db <db_regex_selector>] [-newdb <newdb_name>] [-rp <rp_regex_selector>] [-newrp <newrp_name>] [-meas <meas_regex_selector>]
```

___Description of syntax___

If no `master` or `slave` are provided it takes the default from config file. The db selector allows to filter with regex expression on all dbs.
If the `slave` schema must be different than the `master`, the new schema can be set using `newdb` and `newrp` flags


___Limitations___

- Only the default RP can be renamed

___Examples___

*Example 1*: Copy schema from Influx01 to Influx02

```bash
Influx01 schema
----------------

  |-- db1
    |-- rp1*
    |-- rp2
  |-- db2
    |-- rp1*
    |-- rp2
```

```bash
./bin/syncflux -action "replicaschema" -master "influx01" -slave "influx02"
```

The result will be that the schema of Influx01 will be replicated on Influx02

```bash
Influx02 schema
----------------
  |-- db1
    |-- rp1*
    |-- rp2
  |-- db2
    |-- rp1*
    |-- rp2
```

*Example 2*: Copy schema from Influx01-DB1 to Influx02

```bash
Influx01 schema
----------------

  |-- db1
    |-- rp1*
    |-- rp2
  |-- db2
    |-- rp1*
    |-- rp2
```

```bash
./bin/syncflux -action "replicaschema" -master "influx01" -slave "influx02" -db "^db1$"
```

The result will be that the schema of Influx01 will be replicated on Influx02

```bash
Influx02 schema
----------------
  |-- db1
    |-- rp1*
    |-- rp2
```


*Example 3*: Copy schema from Influx01-DB1 to Influx02-DB3 (new db called DB3) and only from rp1

```
Influx01 schema
----------------

  |-- db1
    |-- rp1*
    |-- rp2
  |-- db2
    |-- rp1*
    |-- rp2
```

```bash
./bin/syncflux -action "replicaschema" -master "influx01" -slave "influx02" -db "^db1$" -newdb "db3" -rp "^rp1$"
```

The result will be that the schema of Influx01 will be replicated on Influx02

```bash
Influx02 schema
----------------
  |-- db3
    |-- rp1*
```

*Example 4*: Copy schema from Influx01-DB1 to Influx02-DB3 (new db called DB3) and set the defaultrp to rp3

```bash
Influx01 schema
----------------

  |-- db1
    |-- rp1*
    |-- rp2
  |-- db2
    |-- rp1*
    |-- rp2
```

```bash
./bin/syncflux -action "replicaschema" -master "influx01" -slave "influx02" -db "^db1$" -newdb "db3" -newrp "rp3"
```

The result will be that the schema of Influx01 will be replicated on Influx02

```bash
Influx02 schema
----------------
  |-- db3
    |-- rp3*
    |-- rp2
```

*Example 5*: Copy data and schema from Influx01-DB1 to Influx02-DB3 (new db called DB3) and only from meas "cpu.*"

```bash
Influx01 schema
----------------

  |-- db1
    |-- rp1*
      |-- cpu
      |-- mem
      |-- swap
      |-- ...
    |-- rp2
  |-- db2
    |-- rp1*
    |-- rp2
```

#### Copy data

Allows the user to copy DB data from master to slave. DB schema are DBs and RPs.


___Syntax___
 
```
./bin/syncflux -action copy [-master <master_id>] [-slave <slave_id>] [-db <db_regex_selector>] [-newdb <newdb_name>] [-rp <rp_regex_selector>] [-newrp <newrp_name>] [-meas <meas_regex_selector>] { [-start <start_time>] [-endtime <end_time>] , [-full] }
```

___Description of syntax___

If no `master` or `slave` are provided it takes the default from config file. The db selector allows to filter with regex expression on all dbs.
If the `slave` schema must be different than the `master`, the new schema can be set using `newdb` and `newrp` flags
The `start` end `end` allow to define a time window to copy data. If `full` is passed, the data will be copied from now to `max-retention-interval`

> Remember that with this action schema is not replicated so if the DB or RP on slave doesn't exists it will be skipped

___Limitations___

- ...

___Examples___

*Example 1*: Copy all data from Influx01 to Influx02

```bash
Influx01 schema
----------------

  |-- db1
    |-- rp1*
    |-- rp2
  |-- db2
    |-- rp1*
    |-- rp2
```

```bash
./bin/syncflux -action "coy" -master "influx01" -slave "influx02"
```

The command above will copy data from all dbs from Influxdb01 into Influx02

```bash
Influx02 schema
----------------

  |-- db1
    |-- rp1*
    |-- rp2
  |-- db2
    |-- rp1*
    |-- rp2
```


*Example 2*: Copy data from Influx01-DB1 to Influx02 on a time window and only from rp1

```bash
Influx01 schema
----------------

  |-- db1
    |-- rp1*
    |-- rp2
  |-- db2
    |-- rp1*
    |-- rp2
```

```bash
./bin/syncflux -action "copy" -master "influx01" -slave "influx02" -db "^db1$" -rp "^rp1$" -start -10h end -5h
```

The command above will repicate all data from Influx01 to InfluxDB but only from db1.rp1 and with a time window from -10h to -5h

```bash
Influx02 schema
----------------
  |-- db1
    |-- rp1*
    |-- rp2
```

*Example 3*: Copy data from Influx01-DB1 to Influx02-DB3 (existing db called DB3)

```
Influx01 schema
----------------

  |-- db1
    |-- rp1*
    |-- rp2
  |-- db2
    |-- rp1*
    |-- rp2
```

```bash
./bin/syncflux -action "copy" -master "influx01" -slave "influx02" -db "^db1$" -newdb "db3"
```

The command above will replicate all data from  Influx01-db1 to InfluxDB on a new DB called 'db3'

```bash
Influx02 schema
----------------
  |-- db3
    |-- rp1*
    |-- rp2
```

*Example 4*: Copy data from Influx01-DB1 to Influx02-DB3 (existing db called DB3) and set the defaultrp to existing rp3

```bash
Influx01 schema
----------------

  |-- db1
    |-- rp1*
    |-- rp2
  |-- db2
    |-- rp1*
    |-- rp2
```

```bash
./bin/syncflux -action "copy" -master "influx01" -slave "influx02" -db "^db1$" -newdb "db3"
```


The command above will replicate all data from Influx01-db1 to InfluxDB on a new DB called 'db3' and a new defaultrp called rp3

```bash
Influx02 schema
----------------
  |-- db3
    |-- rp3*
    |-- rp2
```

*Example 5*: Copy data from Influx01-DB1 to Influx02-DB3 (new db called DB3) and only from meas "cpu.*"

```bash
Influx01 schema
----------------

  |-- db1
    |-- rp1*
      |-- cpu
      |-- mem
      |-- swap
      |-- ...
    |-- rp2
  |-- db2
    |-- rp1*
    |-- rp2
```


```bash
./bin/syncflux -action "copy" -master "influx01" -slave "influx02" -db "^db1$" -newdb "db3" -mes "cpu.*"
```

The command above will replicate all data from Influx01-db1 to InfluxDB on a new DB called 'db3' and a new defaultrp called rp3

```bash
Influx02 schema
----------------
  |-- db3
    |-- rp3*
      |-- cpu
    |-- rp2
```

#### Copy data + schema

Allows the user to copy DB data from master to slave. DB schema are DBs and RPs.


___Syntax___
 
```
./bin/syncflux -action fullcopy [-master <master_id>] [-slave <slave_id>] [-db <db_regex_selector>] [-newdb <newdb_name>] [-rp <rp_regex_selector>] [-newrp <newrp_name>] [-meas <meas_regex_selector>] { [-start <start_time>] [-endtime <end_time>] , [-full] }
```

___Description of syntax___

If no `master` or `slave` are provided it takes the default from config file. The db selector allows to filter with regex expression on all dbs.
If the `slave` schema must be different than the `master`, the new schema can be set using `newdb` and `newrp` flags
The `start` end `end` allow to define a time window to copy data. If `full` is passed, the data will be copied from now to `max-retention-interval`

> Remember that with this action schema is not replicated so if the DB or RP on slave doesn't exists it will be skipped

___Limitations___

- Only the default RP can be renamed


___Examples___

*Example 1*: Copy all data and schema from Influx01 to Influx02

```bash
Influx01 schema
----------------

  |-- db1
    |-- rp1*
    |-- rp2
  |-- db2
    |-- rp1*
    |-- rp2
```

```bash
./bin/syncflux -action "coy" -master "influx01" -slave "influx02"
```

The command above will create the schema and will copy data from all dbs from Influxdb01 into Influx02

```bash
Influx02 schema
----------------

  |-- db1
    |-- rp1*
    |-- rp2
  |-- db2
    |-- rp1*
    |-- rp2
```


*Example 2*: Copy data and schema from Influx01-DB1 to Influx02 on a time window

```bash
Influx01 schema
----------------

  |-- db1
    |-- rp1*
    |-- rp2
  |-- db2
    |-- rp1*
    |-- rp2
```

```bash
./bin/syncflux -action "copy" -master "influx01" -slave "influx02" -db "^db1$" -start -10h end -5h
```

The command above will create the schema and will repicate all data from Influx01 to InfluxDB but only from db1 and with a time window from -10h to -5h

```bash
Influx02 schema
----------------
  |-- db1
    |-- rp1*
    |-- rp2
```

*Example 3*: Copy data from Influx01-DB1 to Influx02-DB3 (new db called DB3)

```
Influx01 schema
----------------

  |-- db1
    |-- rp1*
    |-- rp2
  |-- db2
    |-- rp1*
    |-- rp2
```

```bash
./bin/syncflux -action "copy" -master "influx01" -slave "influx02" -db "^db1$" -newdb "db3"
```

The command above will create the schema and will replicate all data from  Influx01-db1 to InfluxDB on a new DB called 'db3'

```bash
Influx02 schema
----------------
  |-- db3
    |-- rp1*
    |-- rp2
```

*Example 4*: Copy data and schema from Influx01-DB1 to Influx02-DB3 (new db called DB3) and set the defaultrp to rp3

```bash
Influx01 schema
----------------

  |-- db1
    |-- rp1*
    |-- rp2
  |-- db2
    |-- rp1*
    |-- rp2
```


```bash
./bin/syncflux -action "copy" -master "influx01" -slave "influx02" -db "^db1$" -newdb "db3"
```


The command above will create the schema and will replicate all data from Influx01-db1 to InfluxDB on a new DB called 'db3' and a new defaultrp called rp3

```bash
Influx02 schema
----------------
  |-- db3
    |-- rp3*
    |-- rp2
```

*Example 5*: Copy data and schema from Influx01-DB1 to Influx02-DB3 (new db called DB3) and only from meas "cpu.*"

```bash
Influx01 schema
----------------

  |-- db1
    |-- rp1*
      |-- cpu
      |-- mem
      |-- swap
      |-- ...
    |-- rp2
  |-- db2
    |-- rp1*
    |-- rp2
```


```bash
./bin/syncflux -action "copy" -master "influx01" -slave "influx02" -db "^db1$" -newdb "db3" -mes "cpu.*"
```

The command above will create the schema and will replicate all data from Influx01-db1 to InfluxDB on a new DB called 'db3' and a new defaultrp called rp3

```bash
Influx02 schema
----------------
  |-- db3
    |-- rp3*
      |-- cpu
    |-- rp2
```

### Run as a HA Cluster monitor

```bash
./bin/syncflux -config ./conf/syncflux.conf -action hamonitor 
```
 syncflux by default search a file syncflux.conf in the `CWD/conf/` and syncflux has hamonitor action by default so this last is equivalent to this one

```bash
./bin/syncflux  
```

you can check the cluster state with any HTTP client, posibles values are:

* OK: both nodes are ok
* CHECK_SLAVE_DOWN: current slave is down
* RECOVERING: both databases are working but slave leaks some data and syncflux is recovering them

````bash
 % curl http://localhost:4090/api/health
{
  "ClusterState": "CHECK_SLAVE_DOWN",
  "ClusterNumRecovers": 0,
  "ClusterLastRecoverDuration": 0,
  "MasterState": true,
  "MasterLastOK": "2019-04-06T09:45:05.461897766+02:00",
  "SlaveState": false,
  "SlaveLastOK": "2019-04-06T09:44:55.465393243+02:00"
}

% curl http://localhost:4090/api/health
{
  "ClusterState": "RECOVERING",
  "ClusterNumRecovers": 0,
  "ClusterLastRecoverDuration": 0,
  "MasterState": true,
  "MasterLastOK": "2019-04-06T10:28:25.459701432+02:00",
  "SlaveState": true,
  "SlaveLastOK": "2019-04-06T10:28:25.55500823+02:00"
}


% curl http://localhost:4090/api/health
{
  "ClusterState": "OK",
  "ClusterNumRecovers": 1,
  "ClusterLastRecoverDuration": 2473620691,
  "MasterState": true,
  "MasterLastOK": "2019-04-06T10:28:25.459701432+02:00",
  "SlaveState": true,
  "SlaveLastOK": "2019-04-06T10:28:25.55500823+02:00"
}
````
