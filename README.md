# Victron MQTT bridge 

This work is based on 2 repos.

* Python implementation of [Fabian Lauer](https://github.com/fabian-lauer/dbus-shelly-3em-smartmeter)
* Golang implementation of [stormmurdoc](https://github.com/stormmurdoc/victron_sdm630_bridge)

The goal was to make a MQTT bridge which supports all sorts of EM meters as long long as they store their data in an MQTT server and add a YAML file for configuring it.

## Features
* Golang executeable which should be way faster and easier to setup
* Reactive rather then proactive
* Use any MQTT topic as long as the subtopics are no JSON objects.
* Will work with one phase only. L2 and L3 will just be left with default values which is still enough for the Victron.


![Victron Overview](./.media/meter.png)

![logfile](./.media/log.png)

# Configuration

You need to change the default values in the `victron-mqtt-bridge.yaml` file:
```yaml

updates: 0 #updates to the DBUS > 0 = live on power changes, otherwhise in miliseconds
dryrun: false #disables dbus connection, for testing only
name: "victron-3em-bridge"
CheckForUpdates: true #kills itself if no MQTT updates during the during logging interval apear

logging:
  level: debug #loglevels are: "info,warn,debug,trace", remove to disable logging
  interval: 300 #time in secods to write periodic logs. default: 3600

mqtt:
  broker: 192.168.12.200
  port: 1883
  user: 
  password: 
  topic: shellies/3em/emeter/#

#Victron needs im/exported totals in kWh but for me f.e. those are in Wh 
#these values are multiplied with the aproriate value.
#default 1
factors:
  imported: 0.001  #multiply imported with this value
  exported: 0.001  #multiply exported with this value

phases:
  - name: L1
    #default values if some topic is missing
    voltage: 230.0
    current: 0.0
    power: 0.0
    imported: 0.0
    exported: 0.0
    #relative from topic
    topics:
      Power: 0/power 
      Voltage: 0/voltage
      Current: 0/current
      Imported: 0/total
      Exported: 0/total_returned
  ...
```

# Installing

1. [Download](https://github.com/achmed20/victron_energymeter_mqtt/releases) and extract the latest release and extract it into `/data` or execute this script!
```sh
wget -c https://github.com/achmed20/victron_energymeter_mqtt/releases/latest/download/release.tgz -O - | tar -xz -C /data
```
2. modify the config
```
nano /victron-mqtt-bridge.yaml
```
3. Test it by starting it it manualy. If you dont get errors, and the GX UI is showing values, just reboot
```sh
/data/victron-mqtt-bridge
```

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
Search the output for `path not found, creating dummy`! If you  have those, its likely that either your main `topic` or the topics of `L1-L3` are wrong

