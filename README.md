# SyncFlux 

SyncFlux is an Open Source InfluxDB  Data syncronization and replication tool with HTTP API Interface which has as main goal recover lost data from any  handmade HA influxDB 1.X cluster ( made as any simple relay  https://github.com/influxdata/influxdb-relay )  

For complete information on installation from binary package and configuration you could read the [syncflux wiki](https://github.com/toni-moreno/syncflux/wiki).

If you wish to compile from source code you can follow the next steps

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
go run build.go setup            # only needed once to install godep
godep restore                    # will pull down all golang lib dependencies in your current GOPATH
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
cp conf/sample.synflux.toml conf/syncflux.toml
./bin/syncflux
```

This will create a default user with username *admin* and password *admin* (don't forget to change them!).

### Recompile backend on source change (only for developers)

To rebuild on source change (requires that you executed godep restore)
```bash
go get github.com/Unknwon/bra
bra run  
```
will init a change autodetect webserver with angular-cli (ng serve) and also a autodetect and recompile process with bra for the backend


#### Online config

Now you will be able to test agent and admin it at  http://localhost:4090 
