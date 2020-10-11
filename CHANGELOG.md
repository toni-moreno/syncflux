# v 0.6.8 (unreleased)

## New features

* Support for uint64 columns (#44)

# v 0.6.7 (2020-05-03)

## New features

* Adds `-logmode` option (default console if not set). Related to https://github.com/toni-moreno/influxdb-srelay/issues/26 issue
* Updated base libraries
* Migrated dependancy tool from dep to gomodules

## fixes

* fix #33

# v 0.6.6 (2020-04-21)

## fixes

* fix #26,#31 (thanks to @maxadamo )


# v 0.6.5 (2019-08-19)

## fixes

* fix #22

# v 0.6.4 (2019-06-27)

## fixes

* fix #21

# v 0.6.3 (2019-06-07)

## New features

* added max-points-on-single-write to limit write queries
* added 1  recovery level with bad chunks 

# v 0.6.2 (2019-06-06)

## fixes

* fixes for #18

# v 0.6.1 (2019-06-05)

## fixes

* fixes for #16

# v 0.6.0 (2019-05-08)

## New features

* added command line parameters -rp and -meas to filter rps and measurements (implement #3)

## fixes

* enhanced logic to solve rp name limitations

# v 0.5.0 (2019-04-27)

* added command line parameter -newdb to change database name and -newrp to change Retention policy name.

# v 0.4.0 (2019-04-14)

## New features

* added command line parameters -v, -vv, -vvv for info,debug,trace  log levels 
* added command line -chunk as replacement/override for  data-chuck-duration parameter in the config file (implement #10)
* added config  rw-max-retries,  rw-retry-delay  to fix minor errors (timeouts, latency, net glich)
* added config  num-workers to add concurrent read/write on the same db and chuck time ( several measurements at timel )

## fixes

* fixes for #6,#11,#9,#8

# v 0.3.0 (2019-04-11)

## New features

* added -full option to the -action copy/fullcopy execution modes

## fixes

* fixes for #1,#4,#5,#7

# v 0.2.0 (2019-04-11)

## New features

* added replicashema and fullcopy execution mode.
* added /queryactive endpoint available to external tools.
* added syncronization tunning params data-chuck-duration , max-retention-interval  


# v 0.1.0 (2019-04-07)

## New features

* first release with  slavei db  syncronization with master
* Added initial db schema replication 
* Added initial db data replication
* Added action copy

