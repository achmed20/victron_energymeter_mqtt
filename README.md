# Victron MQTT bridge 

This work is based on 2 repos.

* Python implementation of [Fabian Lauer](https://github.com/fabian-lauer/dbus-shelly-3em-smartmeter)
* Golang implementation of [stormmurdoc](https://github.com/stormmurdoc/victron_sdm630_bridge)

The goal was to make a MQTT bridge which supports all sorts of EM meters as long long as they store their data in an MQTT server and add a YAML file for configuring it.

# Configuration

## Change Default Configuration

You need to change the default values in the `victron-mqtt-bridge.yaml` file:
```yaml
loglevel: trace                         #loglevels are: "info,warn,debug,trace", remove to disable logging
#dryrun: true                           #disables dbus connection, for testing only
client_id: "victron-3em-bridge"         #Name inside Victron

#mqtt config, most likley the only thing you need to change
broker: 192.168.12.200
port: 1883
user: 
password: 
topic: shellies/3em/emeter/#            #base topic. the "#" will subscribe to ALL topics beneath it

phases: 3                               #amount of phases, required

L1:
  #default values in case some topic is missing
  voltage: 230.0
  current: 0.0
  power: 0.0
  imported: 0.0
  exported: 0.0
  #relative path from topic, just write "something" if it's unassigned
  topic:
    Power: 0/power                      #will result in 'shellies/3em/emeter/0/power'   
    Voltage: 0/voltage                  #...
    Current: 0/current
    Imported: 0/total
    Exported: 0/total_returned

L2:
  voltage: 230.0
  current: 0.0
  power: 0.0
  imported: 0.0
  exported: 0.0
  topic:
    Power: 1/power
    Voltage: 1/voltage
    Current: 1/current
    Imported: 1/total
    Exported: 1/total_returned

L3:
  voltage: 230.0
  current: 0.0
  power: 0.0
  imported: 0.0
  exported: 0.0
  topic:
    Power: 2/power
    Voltage: 2/voltage
    Current: 2/current
    Imported: 2/total
    Exported: 2/total_returned
```

## Compiling from source

To compile this for the Venus GX (an Arm 7 processor), you can easily cross-compile with the following:
```sh
make build
```
This will create a `.build` fodler with all the filey you need

After Compiling, make sure to to change the config under `.build/victron-mqtt-bridge.yaml`

## Installing

copy contents of `.build` over to your device by hand (into the `/data` folder)

or

if you Venus device happens to be named `einstein`, you can use this command to copy the files over
```sh
make copy
```
