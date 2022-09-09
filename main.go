package main

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/godbus/dbus/introspect"
	"github.com/godbus/dbus/v5"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"main.go/phase"
)

/* Configuration */
var (
	BROKER    = "192.168.1.119"
	PORT      = 1883
	TOPIC     = "stromzaehler/#"
	CLIENT_ID = "sdm630-bridge"
	USERNAME  = "user"
	PASSWORD  = "pass"
	DevideBy  = 1
)

var P1 float64 = 0.00
var P2 float64 = 0.00
var P3 float64 = 0.00
var psum float64 = 0.00
var psum_update bool = true
var value_correction bool = false
var conn, err = dbus.SystemBus()

const intro = `
<node>
   <interface name="com.victronenergy.BusItem">
    <signal name="PropertiesChanged">
      <arg type="a{sv}" name="properties" />
    </signal>
    <method name="SetValue">
      <arg direction="in"  type="v" name="value" />
      <arg direction="out" type="i" />
    </method>
    <method name="GetText">
      <arg direction="out" type="s" />
    </method>
    <method name="GetValue">
      <arg direction="out" type="v" />
    </method>
    </interface>` + introspect.IntrospectDataString + `</node> `

type objectpath string

var victronValues = map[int]map[objectpath]dbus.Variant{
	// 0: This will be used to store the VALUE variant
	0: map[objectpath]dbus.Variant{},
	// 1: This will be used to store the STRING variant
	1: map[objectpath]dbus.Variant{},
}

func (f objectpath) GetValue() (dbus.Variant, *dbus.Error) {
	log.Debug("GetValue() called for ", f)
	log.Debug("...returning ", victronValues[0][f])
	return victronValues[0][f], nil
}
func (f objectpath) GetText() (string, *dbus.Error) {
	log.Debug("GetText() called for ", f)
	log.Debug("...returning ", victronValues[1][f])
	// Why does this end up ""SOMEVAL"" ... trim it I guess
	return strings.Trim(victronValues[1][f].String(), "\""), nil
}

func init() {
	log.SetFormatter(&log.TextFormatter{})

	viper.SetConfigName("victron-mqtt-emm") // name of config file (without extension)
	viper.SetConfigType("yaml")             // REQUIRED if the config file does not have the extension in the name
	viper.AddConfigPath("/etc")             // path to look for the config file in
	viper.AddConfigPath("/data")            // optionally look for config in the working directory
	viper.AddConfigPath(".")                // optionally look for config in the working directory
	err := viper.ReadInConfig()             // Find and read the config file
	if err != nil {                         // Handle errors reading the config file
		panic(fmt.Errorf("fatal error config file: %w", err))
	}

	viper.SetDefault("debug", false)
	viper.SetDefault("broker", "192.168.1.1")
	viper.SetDefault("port", 1883)
	viper.SetDefault("client_id", "victron-em-bridge")
	viper.SetDefault("user", "")
	viper.SetDefault("password", "")
	viper.SetDefault("topic", "stromzaehler/#")

	viper.SetDefault("phases", 3)

	BROKER = viper.GetString("broker")
	PORT = viper.GetInt("port")
	TOPIC = viper.GetString("topic")
	CLIENT_ID = viper.GetString("client_id")
	USERNAME = viper.GetString("user")
	PASSWORD = viper.GetString("password")

	// spew.Dump(viper.GetStringMap("l1"))
	// spew.Dump()
	//-----------------------------
	if viper.GetBool("debug") {
		log.SetLevel(log.DebugLevel)
	}

	// -------- setup phases -----------
	lineName := "l1"
	var lineDefaults = viper.GetStringMap(lineName)
	if len(lineDefaults) == 0 {
		log.Panic("no config for L1 found, exiting")
	}
	for i := 1; i < viper.GetInt("phases")+1; i++ {
		lineName = "l" + strconv.Itoa(i)
		var lineVals = viper.GetStringMap(lineName)
		log.Debug("getting config for " + lineName)

		if len(lineVals) == 0 {
			phase.Lines = append(phase.Lines, phase.SinglePhase{
				Name:    "L" + strconv.Itoa(i),
				Voltage: phase.Lines[0].Voltage,
			})
		} else {
			topics := lineVals["topic"].(map[string]interface{})
			phase.Lines = append(phase.Lines, phase.SinglePhase{
				Name:     "L" + strconv.Itoa(i),
				Voltage:  lineVals["voltage"].(float64),
				Current:  lineVals["current"].(float64),
				Power:    lineVals["power"].(float64),
				Imported: lineVals["imported"].(float64),
				Exported: lineVals["exported"].(float64),

				Topics: phase.Topics{
					Voltage:  topics["voltage"].(string),
					Power:    topics["power"].(string),
					Current:  topics["current"].(string),
					Imported: topics["imported"].(string),
					Exported: topics["exported"].(string),
				},
			})
		}

	}

}

func main() {
	// Need to implement following paths:
	// https://github.com/victronenergy/venus/wiki/dbus#grid-meter
	// also in system.py
	victronValues[0]["/Connected"] = dbus.MakeVariant(1)
	victronValues[1]["/Connected"] = dbus.MakeVariant("1")

	victronValues[0]["/CustomName"] = dbus.MakeVariant("Grid meter")
	victronValues[1]["/CustomName"] = dbus.MakeVariant("Grid meter")

	victronValues[0]["/DeviceInstance"] = dbus.MakeVariant(30)
	victronValues[1]["/DeviceInstance"] = dbus.MakeVariant("30")

	// also in system.py
	victronValues[0]["/DeviceType"] = dbus.MakeVariant(71)
	victronValues[1]["/DeviceType"] = dbus.MakeVariant("71")

	victronValues[0]["/ErrorCode"] = dbus.MakeVariantWithSignature(0, dbus.SignatureOf(123))
	victronValues[1]["/ErrorCode"] = dbus.MakeVariant("0")

	victronValues[0]["/FirmwareVersion"] = dbus.MakeVariant(2)
	victronValues[1]["/FirmwareVersion"] = dbus.MakeVariant("2")

	// also in system.py
	victronValues[0]["/Mgmt/Connection"] = dbus.MakeVariant("/dev/ttyUSB0")
	victronValues[1]["/Mgmt/Connection"] = dbus.MakeVariant("/dev/ttyUSB0")

	victronValues[0]["/Mgmt/ProcessName"] = dbus.MakeVariant("/opt/color-control/dbus-cgwacs/dbus-cgwacs")
	victronValues[1]["/Mgmt/ProcessName"] = dbus.MakeVariant("/opt/color-control/dbus-cgwacs/dbus-cgwacs")

	victronValues[0]["/Mgmt/ProcessVersion"] = dbus.MakeVariant("1.8.0")
	victronValues[1]["/Mgmt/ProcessVersion"] = dbus.MakeVariant("1.8.0")

	victronValues[0]["/Position"] = dbus.MakeVariantWithSignature(0, dbus.SignatureOf(123))
	victronValues[1]["/Position"] = dbus.MakeVariant("0")

	// also in system.py
	victronValues[0]["/ProductId"] = dbus.MakeVariant(45058)
	victronValues[1]["/ProductId"] = dbus.MakeVariant("45058")

	// also in system.py
	victronValues[0]["/ProductName"] = dbus.MakeVariant("Grid meter")
	victronValues[1]["/ProductName"] = dbus.MakeVariant("Grid meter")

	victronValues[0]["/Serial"] = dbus.MakeVariant("BP98305081235")
	victronValues[1]["/Serial"] = dbus.MakeVariant("BP98305081235")

	// Provide some initial values... note that the values must be a valid formt otherwise dbus_systemcalc.py exits like this:
	//@400000005ecc11bf3782b374   File "/opt/victronenergy/dbus-systemcalc-py/dbus_systemcalc.py", line 386, in _handletimertick
	//@400000005ecc11bf37aa251c     self._updatevalues()
	//@400000005ecc11bf380e74cc   File "/opt/victronenergy/dbus-systemcalc-py/dbus_systemcalc.py", line 678, in _updatevalues
	//@400000005ecc11bf383ab4ec     c = _safeadd(c, p, pvpower)
	//@400000005ecc11bf386c9674   File "/opt/victronenergy/dbus-systemcalc-py/sc_utils.py", line 13, in safeadd
	//@400000005ecc11bf387b28ec     return sum(values) if values else None
	//@400000005ecc11bf38b2bb7c TypeError: unsupported operand type(s) for +: 'int' and 'unicode'
	//
	victronValues[0]["/Ac/L1/Power"] = dbus.MakeVariant(0.0)
	victronValues[1]["/Ac/L1/Power"] = dbus.MakeVariant("0 W")
	victronValues[0]["/Ac/L2/Power"] = dbus.MakeVariant(0.0)
	victronValues[1]["/Ac/L2/Power"] = dbus.MakeVariant("0 W")
	victronValues[0]["/Ac/L3/Power"] = dbus.MakeVariant(0.0)
	victronValues[1]["/Ac/L3/Power"] = dbus.MakeVariant("0 W")

	victronValues[0]["/Ac/L1/Voltage"] = dbus.MakeVariant(230)
	victronValues[1]["/Ac/L1/Voltage"] = dbus.MakeVariant("230 V")
	victronValues[0]["/Ac/L2/Voltage"] = dbus.MakeVariant(230)
	victronValues[1]["/Ac/L2/Voltage"] = dbus.MakeVariant("230 V")
	victronValues[0]["/Ac/L3/Voltage"] = dbus.MakeVariant(230)
	victronValues[1]["/Ac/L3/Voltage"] = dbus.MakeVariant("230 V")

	victronValues[0]["/Ac/L1/Current"] = dbus.MakeVariant(0.0)
	victronValues[1]["/Ac/L1/Current"] = dbus.MakeVariant("0 A")
	victronValues[0]["/Ac/L2/Current"] = dbus.MakeVariant(0.0)
	victronValues[1]["/Ac/L2/Current"] = dbus.MakeVariant("0 A")
	victronValues[0]["/Ac/L3/Current"] = dbus.MakeVariant(0.0)
	victronValues[1]["/Ac/L3/Current"] = dbus.MakeVariant("0 A")

	victronValues[0]["/Ac/L1/Energy/Forward"] = dbus.MakeVariant(0.0)
	victronValues[1]["/Ac/L1/Energy/Forward"] = dbus.MakeVariant("0 kWh")
	victronValues[0]["/Ac/L2/Energy/Forward"] = dbus.MakeVariant(0.0)
	victronValues[1]["/Ac/L2/Energy/Forward"] = dbus.MakeVariant("0 kWh")
	victronValues[0]["/Ac/L3/Energy/Forward"] = dbus.MakeVariant(0.0)
	victronValues[1]["/Ac/L3/Energy/Forward"] = dbus.MakeVariant("0 kWh")

	victronValues[0]["/Ac/L1/Energy/Reverse"] = dbus.MakeVariant(0.0)
	victronValues[1]["/Ac/L1/Energy/Reverse"] = dbus.MakeVariant("0 kWh")
	victronValues[0]["/Ac/L2/Energy/Reverse"] = dbus.MakeVariant(0.0)
	victronValues[1]["/Ac/L2/Energy/Reverse"] = dbus.MakeVariant("0 kWh")
	victronValues[0]["/Ac/L3/Energy/Reverse"] = dbus.MakeVariant(0.0)
	victronValues[1]["/Ac/L3/Energy/Reverse"] = dbus.MakeVariant("0 kWh")

	basicPaths := []dbus.ObjectPath{
		"/Connected",
		"/CustomName",
		"/DeviceInstance",
		"/DeviceType",
		"/ErrorCode",
		"/FirmwareVersion",
		"/Mgmt/Connection",
		"/Mgmt/ProcessName",
		"/Mgmt/ProcessVersion",
		"/Position",
		"/ProductId",
		"/ProductName",
		"/Serial",
	}

	updatingPaths := []dbus.ObjectPath{
		"/Ac/L1/Power",
		"/Ac/L2/Power",
		"/Ac/L3/Power",
		"/Ac/L1/Voltage",
		"/Ac/L2/Voltage",
		"/Ac/L3/Voltage",
		"/Ac/L1/Current",
		"/Ac/L2/Current",
		"/Ac/L3/Current",
		"/Ac/L1/Energy/Forward",
		"/Ac/L2/Energy/Forward",
		"/Ac/L3/Energy/Forward",
		"/Ac/L1/Energy/Reverse",
		"/Ac/L2/Energy/Reverse",
		"/Ac/L3/Energy/Reverse",
	}

	defer conn.Close()
	// Some of the victron stuff requires it be called grid.cgwacs... using the only known valid value (from the simulator)
	// This can _probably_ be changed as long as it matches com.victronenergy.grid.cgwacs_*
	reply, err := conn.RequestName("com.victronenergy.grid.cgwacs_ttyUSB0_di30_mb1",
		dbus.NameFlagDoNotQueue)
	if err != nil {
		log.Panic("Something went horribly wrong in the dbus connection")
		panic(err)
	}

	if reply != dbus.RequestNameReplyPrimaryOwner {
		log.Panic("name cgwacs_ttyUSB0_di30_mb1 already taken on dbus.")
		os.Exit(1)
	}

	for i, s := range basicPaths {
		log.Debug("Registering dbus basic path #", i, ": ", s)
		conn.Export(objectpath(s), s, "com.victronenergy.BusItem")
		conn.Export(introspect.Introspectable(intro), s, "org.freedesktop.DBus.Introspectable")
	}

	for i, s := range updatingPaths {
		log.Debug("Registering dbus update path #", i, ": ", s)
		conn.Export(objectpath(s), s, "com.victronenergy.BusItem")
		conn.Export(introspect.Introspectable(intro), s, "org.freedesktop.DBus.Introspectable")
	}

	log.Info("Successfully connected to dbus and registered as a meter... Commencing reading of the SDM630 meter")

	// MQTT Subscripte
	opts := mqtt.NewClientOptions()
	opts.AddBroker(fmt.Sprintf("tcp://%s:%d", BROKER, PORT))
	opts.SetClientID(CLIENT_ID)
	opts.SetUsername(USERNAME)
	opts.SetPassword(PASSWORD)
	opts.SetDefaultPublishHandler(messagePubHandler)
	opts.OnConnect = connectHandler
	opts.OnConnectionLost = connectLostHandler
	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}
	sub(client)
	// Infinite loop
	for true {
		//fmt.Println("Infinite Loop entered")
		time.Sleep(time.Second)
	}

	// This is a forever loop^^
	panic("Error: We terminated.... how did we ever get here?")
}

/* MQTT Subscribe Function */
func sub(client mqtt.Client) {
	topic := TOPIC
	token := client.Subscribe(topic, 1, nil)
	token.Wait()
	log.Info("Subscribed to topic: " + topic)
}

/* Write dbus Values to Victron handler */
func updateVariant(value float64, unit string, path string) {
	emit := make(map[string]dbus.Variant)
	emit["Text"] = dbus.MakeVariant(fmt.Sprintf("%.2f", value) + unit)
	emit["Value"] = dbus.MakeVariant(float64(value))
	victronValues[0][objectpath(path)] = emit["Value"]
	victronValues[1][objectpath(path)] = emit["Text"]
	log.WithFields(log.Fields{"path": path, "unit": unit, "value": value}).Debug("new dbus value")
	conn.Emit(dbus.ObjectPath(path), "com.victronenergy.BusItem.PropertiesChanged", emit)
}

/* Convert binary to float64 */
func bin2Float64(bin string) float64 {
	foostring := string(bin)
	result, err := strconv.ParseFloat(foostring, 64)
	if err != nil {
		panic(err)
	}
	return result
}

/* Called if connection is established */
var connectHandler mqtt.OnConnectHandler = func(client mqtt.Client) {
	log.Info(fmt.Sprintf("Connected to broker %s:%d", BROKER, PORT))
}

/* Called if connection is lost  */
var connectLostHandler mqtt.ConnectionLostHandler = func(client mqtt.Client, err error) {
	log.Info(fmt.Sprintf("Connect lost: %v", err))
}

/* Search for string with regex */
func ContainString(searchstring string, str string) bool {
	var obj bool

	obj, err = regexp.MatchString(searchstring, str)

	if err != nil {
		panic(err)
	}

	return obj
}

/* MQTT Subscribe Handler */
var messagePubHandler mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {

	log.Debug(fmt.Sprintf("Received message: %s from topic: %s\n", msg.Payload(), msg.Topic()))
	value_correction = false
	var foundSomething bool

	for key := 0; key < len(phase.Lines); key++ {
		v := &phase.Lines[key]
		payload := string(msg.Payload())

		//power
		if v.Topics.Power != "" && ContainString(".*"+v.Topics.Power+"$", msg.Topic()) {
			v.Power = bin2Float64(payload)
			log.WithFields(log.Fields{"phase": v.Name, "payload": payload, "topic": v.Topics.Power}).
				Debug("found matching topic for Power")
			foundSomething = true
		}

		//current
		if v.Topics.Current != "" && ContainString(".*"+v.Topics.Current+"$", msg.Topic()) {
			v.Current = bin2Float64(payload)
			log.WithFields(log.Fields{"phase": v.Name, "payload": payload, "topic": v.Topics.Current}).
				Debug("found matching topic for Current")
			foundSomething = true
		}

		//voltage
		if v.Topics.Voltage != "" && ContainString(".*"+v.Topics.Voltage+"$", msg.Topic()) {
			v.Voltage = bin2Float64(payload)
			log.WithFields(log.Fields{"phase": v.Name, "payload": payload, "topic": v.Topics.Voltage}).
				Debug("found matching topic for Voltage")
			foundSomething = true
		}

		//Imported
		if v.Topics.Imported != "" && ContainString(".*"+v.Topics.Imported+"$", msg.Topic()) {
			v.Imported = bin2Float64(payload)
			log.WithFields(log.Fields{"phase": v.Name, "payload": payload, "topic": v.Topics.Imported}).
				Debug("found matching topic for Imported")
			foundSomething = true
		}

		//exported
		if v.Topics.Exported != "" && ContainString(".*"+v.Topics.Exported+"$", msg.Topic()) {
			v.Exported = bin2Float64(payload)
			log.WithFields(log.Fields{"phase": v.Name, "payload": payload, "topic": v.Topics.Exported}).
				Debug("found matching topic for Exported")
			foundSomething = true
		}
	}

	if foundSomething {
		var tKw float64
		var tImported float64
		var tExported float64
		var emptyCurrent bool
		var emptyPower bool

		for key := 0; key < len(phase.Lines); key++ {

			v := &phase.Lines[key]
			//fix / calc values
			if v.Voltage == 0 {
				log.Warn("Voltage missing, setting default value of 230")
				v.Voltage = 230
			}
			if v.Power != 0 && v.Current == 0 {
				log.Debug("current missing, calculating value")
				v.Current = v.Power / v.Voltage
				emptyCurrent = true
			}
			if v.Current != 0 && v.Power == 0 {
				log.Debug("power missing, calculating value")
				v.Power = v.Voltage * v.Current
				emptyPower = true
			}

			updateVariant(v.Power, "W", "/Ac/"+v.Name+"/Power")
			updateVariant(v.Current, "A", "/Ac/"+v.Name+"/Current")
			updateVariant(v.Voltage, "V", "/Ac/"+v.Name+"/Voltage")
			updateVariant(v.Exported, "kWh", "/Ac/"+v.Name+"/Energy/Forward")
			updateVariant(v.Imported, "kWh", "/Ac/"+v.Name+"/Energy/Reverse")

			tKw += v.Power
			tImported += v.Imported
			tExported += v.Exported

			log.WithFields(log.Fields{
				"Phase":    v.Name,
				"Power":    v.Power,
				"Current":  v.Current,
				"Voltage":  v.Voltage,
				"Exported": v.Exported,
				"Imported": v.Imported,
			}).Info("New values for " + v.Name)
			if emptyCurrent {
				v.Current = 0
			}
			if emptyPower {
				v.Power = 0
			}
		}

		updateVariant(tKw, "W", "/Ac/Power")
		updateVariant(tExported, "kWh", "/Ac/Energy/Forward")
		updateVariant(tImported, "kWh", "/Ac/Energy/Reverse")
	}

}
