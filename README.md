# Victron MQTT bridge 

This work is based on 2 repos.

* Python implementation of [Fabian Lauer](https://github.com/fabian-lauer/dbus-shelly-3em-smartmeter)
* Golang implementation of [stormmurdoc](https://github.com/stormmurdoc/victron_sdm630_bridge)

The goal was to make a MQTT bridge which supports all sorts of EM meters as long long as they store their data in an MQTT server and add a YAML file for configuring it.

## Features
* Can use any MQTT topic as long as the subtopics are no JSON objects.
* Will work with one phase only. L2 and L3 will just be left with default values which is still enough for the Victron.
* Will work with only Power as input! I had a SML reader before and its only output was power, which is enough to calculate the missing values.
  - Autogenerates `current` based on `power` and `voltage`
  - able to set default for Voltage in case its missing

# Configuration

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

# Required, if if you 
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

#Remove if you only have 1 phase to track
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

#Remove if you only have 1 phase to track
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

# Installing

1. Download and extract `build.zip`
2. Change the `victron-mqtt-bridge.yaml`. 
3. Copy contents over to your device by hand (into the `/data` folder)
4. Restart your GX device

## Compiling from source

Compile it with
```sh
make build
```
This will create a `.build` folder with all the files you need

# Troubleshooting

* make sure your MQTT server is correct
* make sure your topics are correct
* take a look at the logfile under `/data/victron-mqtt-bridge.log` for startup errors

# Basic debugging

start `/data/victron-mqtt-bridge` by hand. and look for errors

## Advanced debugging

Try changing the loglevel to trace in `/data/victron-mqtt-bridge.yaml`
```yaml
loglevel: trace
```
**make sure to set it back to `info` once your problem is solved**

### Values dont change?
Search the output for `found matching topic`! If you dont have those, its likely that either your main `topic` or the topics of `L1-L3` are wrong
```
TRAC[2023-03-27T14:44:06+02:00] Received message: 200.28 from topic: shellies/3em/emeter/2/power 
TRAC[2023-03-27T14:44:06+02:00] found matching topic for Power                payload=200.28 phase=L3 topic=2/power
```
