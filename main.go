package main

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"os/signal"
	"reflect"
	"strconv"
	"strings"
	"syscall"

	"victron_energymeter_mqtt/dbustools"
	"victron_energymeter_mqtt/phase"

	"github.com/davecgh/go-spew/spew"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
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

// var P1 float64 = 0.00
// var P2 float64 = 0.00
// var P3 float64 = 0.00
// var psum float64 = 0.00
// var psum_update bool = true
// var value_correction bool = false
var dryrun bool
var totalMessages uint32
var logInterval int32

func init() {
	log.SetFormatter(&log.TextFormatter{
		// DisableColors: true,
		FullTimestamp: true,
	})

	viper.SetConfigName("victron-mqtt-bridge") // name of config file (without extension)
	viper.SetConfigType("yaml")                // REQUIRED if the config file does not have the extension in the name
	viper.AddConfigPath("/etc")                // path to look for the config file in
	viper.AddConfigPath("/data")               // optionally look for config in the working directory
	viper.AddConfigPath(".")                   // optionally look for config in the working directory
	err := viper.ReadInConfig()                // Find and read the config file
	if err != nil {                            // Handle errors reading the config file
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

	//-----------------------------

	if viper.GetBool("dryrun") {
		log.Warn("dry run / dbus disabled")
		dbustools.DryRun = true
	}

	switch viper.GetString("loglevel") {
	case "info":
		log.SetLevel(log.InfoLevel)
		break
	case "debug":
		log.SetLevel(log.DebugLevel)
		break
	case "warn":
		log.SetLevel(log.WarnLevel)
		break
	case "trace":
		log.SetLevel(log.TraceLevel)
		break
	default:
		log.SetOutput(ioutil.Discard)
	}
	logInterval = viper.GetInt32("loginterval")
	if logInterval == 0 {
		logInterval = 3600
	}
	log.Info(fmt.Sprintf("log interval set to %d", logInterval))

	// -------- setup phases -----------
	phase.LoadConfig(viper.GetStringMap("l1"))
}

func main() {
	dbustools.Connect()
	defer dbustools.Close()

	// mqtt.ERROR = llog.New(os.Stdout, "[ERROR] ", 0)
	// mqtt.CRITICAL = llog.New(os.Stdout, "[CRIT] ", 0)
	// mqtt.WARN = llog.New(os.Stdout, "[WARN]  ", 0)
	// mqtt.DEBUG = llog.New(os.Stdout, "[DEBUG] ", 0)

	log.Info("Successfully connected to dbus and registered as a '" + CLIENT_ID + "'")
	// MQTT Subscripte
	opts := mqtt.NewClientOptions()
	opts.SetOrderMatters(false) //important or it will crash
	opts.AddBroker(fmt.Sprintf("tcp://%s:%d", BROKER, PORT))
	opts.SetClientID(CLIENT_ID + RandomString(10))
	opts.SetUsername(USERNAME)
	opts.SetPassword(PASSWORD)
	opts.SetDefaultPublishHandler(messageHandler) //func that handles all messages
	opts.OnConnect = connectHandler
	opts.OnConnectionLost = connectLostHandler
	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		log.WithField("error", token.Error()).Panic("could not connect to MQTT server")
	}
	func(client mqtt.Client) {
		topic := TOPIC
		token := client.Subscribe(topic, 1, nil)
		token.Wait()
		log.Info("Subscribed to topic: " + topic)
	}(client)
	// Infinite loop

	done := make(chan os.Signal, 1)
	signal.Notify(done, syscall.SIGINT, syscall.SIGTERM)
	fmt.Println("press ctrl+c to exit")
	<-done

}

// ------------------------------------------------------------------------------------

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
	//panic and let the script restart
	log.Panic(fmt.Sprintf("Connect lost: %v", err))
	//os.Exit(1)
}

/* Search for string with regex */
func IsPartOf(searchstring string, str string) bool {
	out := strings.HasSuffix(str, searchstring)
	// spew.Dump(str, searchstring, out)
	return out
	// return strings.Contains(str, searchstring)
}

// ##########################################################################################

type phaseCache struct {
	Field string
	Phase *phase.SinglePhase
}

var cache map[string]phaseCache

var messageHandler mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {

	log.Debug(fmt.Sprintf("Received message: %s from topic: %s\n", msg.Payload(), msg.Topic()))

	payload := bin2Float64(string(msg.Payload()))

	if _, ok := cache[msg.Topic()]; ok {
		//itterate through phases if not found in cache
		for key := 0; key < len(phase.Lines); key++ {
			ph := phase.Lines[key]

			v := reflect.ValueOf(ph.Topics)
			typeOfS := v.Type()

			for i := 0; i < v.NumField(); i++ {
				subtopic := v.Field(i).String()
				if IsPartOf(subtopic, msg.Topic()) {
					cache[msg.Topic()] = phaseCache{
						Field: typeOfS.Field(i).Name,
						Phase: &phase.Lines[key],
					}
					continue
				}
				// spew.Dump(typeOfS.Field(i).Name, v.Field(i).Interface())
			}

		}
	} else if ph, ok := cache[msg.Topic()]; ok {
		ph.Phase.SetByName(ph.Field, payload)
	}

	for key := 0; key < len(phase.Lines); key++ {
		spew.Dump(phase.Lines[key])
	}

}

// #########################################################################

/* MQTT Subscribe Handler */
var messagePubHandler mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {

	log.Debug(fmt.Sprintf("Received message: %s from topic: %s\n", msg.Payload(), msg.Topic()))
	var foundSomething bool
	var updateDbusGlobal bool

	//itterate through phases
	for key := 0; key < len(phase.Lines); key++ {
		v := &phase.Lines[key]
		payload := string(msg.Payload())

		//power
		if v.Topics.Power != "" && IsPartOf(v.Topics.Power, msg.Topic()) {
			v.Power = bin2Float64(payload)
			log.WithFields(log.Fields{"phase": v.Name, "payload": payload, "topic": v.Topics.Power}).
				Trace("found matching topic for Power")
			foundSomething = true
			updateDbusGlobal = true
		}

		//current
		if v.Topics.Current != "" && IsPartOf(v.Topics.Current, msg.Topic()) {
			v.Current = bin2Float64(payload)
			log.WithFields(log.Fields{"phase": v.Name, "payload": payload, "topic": v.Topics.Current}).
				Trace("found matching topic for Current")
			foundSomething = true
		}

		//voltage
		if v.Topics.Voltage != "" && IsPartOf(v.Topics.Voltage, msg.Topic()) {
			v.Voltage = bin2Float64(payload)
			log.WithFields(log.Fields{"phase": v.Name, "payload": payload, "topic": v.Topics.Voltage}).
				Trace("found matching topic for Voltage")
			foundSomething = true
		}

		//Imported
		// if v.Topics.Imported != "" && IsPartOf(v.Topics.Imported, msg.Topic()) {
		if v.Topics.Imported != "" && IsPartOf(v.Topics.Imported, msg.Topic()) {
			v.Imported = bin2Float64(payload)
			log.WithFields(log.Fields{"phase": v.Name, "payload": payload, "topic": v.Topics.Imported}).
				Trace("found matching topic for Imported")
			foundSomething = true
		}

		//exported
		if v.Topics.Exported != "" && IsPartOf(v.Topics.Exported, msg.Topic()) {
			v.Exported = bin2Float64(payload)
			log.WithFields(log.Fields{"phase": v.Name, "payload": payload, "topic": v.Topics.Exported}).
				Trace("found matching topic for Exported")
			foundSomething = true
		}

		if foundSomething {
			totalMessages++
			//fix / calc values
			if v.Voltage == 0 {
				log.Warn("Voltage missing, setting default value of 230")
				v.Voltage = 230
			}
			if v.Power != 0 && v.Current == 0 {
				log.Debug("current missing, calculating value")
				v.Current = v.Power / v.Voltage
			}
			if v.Current != 0 && v.Power == 0 {
				log.Debug("power missing, calculating value")
				v.Power = v.Voltage * v.Current
			}
			//update totals
			if updateDbusGlobal {
				UpdateDbus()
			}

		}
	}

}

func UpdateDbus() {

	var tKw float64
	var tImported float64
	var tExported float64
	for pk := 0; pk < len(phase.Lines); pk++ {
		ph := &phase.Lines[pk]
		tKw += ph.Power
		tImported += ph.Imported
		tExported += ph.Exported

		log.WithFields(log.Fields{
			// "payload":  string(msg.Payload()),
			// "topic":    msg.Topic(),
			"Phase":    ph.Name,
			"Power":    ph.Power,
			"Current":  ph.Current,
			"Voltage":  ph.Voltage,
			"Exported": ph.Exported,
			"Imported": ph.Imported,
		}).Debug("New MQTT values for " + ph.Name)

		dbustools.Update(ph.Power, "W", "/Ac/"+ph.Name+"/Power")
		dbustools.Update(ph.Current, "A", "/Ac/"+ph.Name+"/Current")
		dbustools.Update(ph.Voltage, "V", "/Ac/"+ph.Name+"/Voltage")
		dbustools.Update(ph.Exported, "kWh", "/Ac/"+ph.Name+"/Energy/Forward")
		dbustools.Update(ph.Imported, "kWh", "/Ac/"+ph.Name+"/Energy/Reverse")

	}
	dbustools.Update(tKw, "W", "/Ac/Power")
	dbustools.Update(tExported, "kWh", "/Ac/Energy/Forward")
	dbustools.Update(tImported, "kWh", "/Ac/Energy/Reverse")

}

func RandomString(n int) string {
	var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

	s := make([]rune, n)
	for i := range s {
		s[i] = letters[rand.Intn(len(letters))]
	}
	return string(s)
}
