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
	"sync"
	"syscall"
	"time"

	"victron_energymeter_mqtt/dbustools"
	"victron_energymeter_mqtt/phase"

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

var Cache sync.Map

// [string]phaseCache

type phaseCache struct {
	Field string
	Valid bool
	Phase *phase.SinglePhase
}

var dryrun bool
var totalMessages uint32
var logInterval int32
var updateInterval int32
var factor map[string]float64

var validLineImported map[string]*phase.SinglePhase
var validLineExported map[string]*phase.SinglePhase

func init() {
	factor = make(map[string]float64)
	validLineImported = make(map[string]*phase.SinglePhase)
	validLineExported = make(map[string]*phase.SinglePhase)

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

	updateInterval = viper.GetInt32("updateinterval")
	if updateInterval < 250 && updateInterval != 0 {
		updateInterval = 250
	}

	logInterval = viper.GetInt32("loginterval")
	if logInterval == 0 {
		logInterval = 3600
	}
	log.Info(fmt.Sprintf("log interval set to %d", logInterval))

	// -------- setup phases -----------
	phase.LoadConfig()
	for _, v := range phase.Lines {
		log.Info("Configuration found for " + v.Name)
	}

	// -------- load factors -----------
	for k, v := range viper.GetStringMap("factors") {
		factor[k] = v.(float64)
	}
	if len(factor) > 0 {
		log.Info("Factors found and loaded")

	}
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

	go func() {
		logTicker := time.NewTicker(time.Second * time.Duration(logInterval))
		for _ = range logTicker.C {
			log.WithField("updates_sent", totalMessages).Info("still allive")
			totalMessages = 0
		}
	}()

	if updateInterval > 0 {
		go func() {
			updateTicker := time.NewTicker(time.Millisecond * time.Duration(updateInterval))
			log.WithField("ms", updateInterval).Info("update interval set to delayed")
			for _ = range updateTicker.C {
				for _, ph := range phase.Lines {
					UpdateDbusPhase(&ph)
				}
				UpdateDbusGlobal()
			}
		}()
	} else {
		log.WithField("ms", updateInterval).Info("update interval set to LIVE")
	}

	// Wait for ctrl+c
	done := make(chan os.Signal, 1)
	signal.Notify(done, syscall.SIGINT, syscall.SIGTERM)
	<-done
	os.Exit(0)

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

var messageHandler mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
	log.Trace(fmt.Sprintf("Received message: %s from topic: %s\n", msg.Payload(), msg.Topic()))

	payload := bin2Float64(string(msg.Payload()))
	if _, ok := Cache.Load(msg.Topic()); !ok {
		log.WithField("path", msg.Topic()).Trace("set cache for missing path")
		//itterate through phases if not found in cache
		for key := 0; key < len(phase.Lines); key++ {
			ph := phase.Lines[key]

			v := reflect.ValueOf(ph.Topics)
			typeOfS := v.Type()

			for i := 0; i < v.NumField(); i++ {
				subtopic := v.Field(i).String()
				if IsPartOf(subtopic, msg.Topic()) {
					Cache.Store(msg.Topic(), phaseCache{
						Field: typeOfS.Field(i).Name,
						Valid: true,
						Phase: &phase.Lines[key],
					})
					continue
				}
				// spew.Dump(typeOfS.Field(i).Name, v.Field(i).Interface())
			}

		}
		if _, ok := Cache.Load(msg.Topic()); !ok {
			log.WithField("path", msg.Topic()).Trace("path not found, creating dummy")
			Cache.Store(msg.Topic(), phaseCache{})
		}
	}

	if tmp, ok := Cache.Load(msg.Topic()); ok {
		ph := tmp.(phaseCache)
		if ph.Valid {
			log.WithField("path", msg.Topic()).Trace("cache found")
			if fac, ok := factor[strings.ToLower(ph.Field)]; ok {
				log.WithField("field", ph.Field).WithField("factor", fac).Trace("factor found and used")
				payload = payload * fac
			}
			ph.Phase.SetByName(ph.Field, payload)
			switch ph.Field {
			case "Power":
				if updateInterval == 0 {
					UpdateDbusPhase(ph.Phase)
					UpdateDbusGlobal()
				}
			case "Imported":
				validLineImported[ph.Phase.Name] = ph.Phase
			case "Exported":
				validLineExported[ph.Phase.Name] = ph.Phase
			}
		}
	}

}

func UpdateDbusPhase(uphase *phase.SinglePhase) {
	if uphase != nil {
		totalMessages++
		log.WithFields(log.Fields{
			"Phase":    uphase.Name,
			"Power":    uphase.Power,
			"Current":  uphase.Current,
			"Voltage":  uphase.Voltage,
			"Exported": uphase.Exported,
			"Imported": uphase.Imported,
		}).Debug("values for " + uphase.Name)

		dbustools.Update(uphase.Power, "W", "/Ac/"+uphase.Name+"/Power")
		dbustools.Update(uphase.Current, "A", "/Ac/"+uphase.Name+"/Current")
		dbustools.Update(uphase.Voltage, "V", "/Ac/"+uphase.Name+"/Voltage")
		dbustools.Update(uphase.Exported, "kWh", "/Ac/"+uphase.Name+"/Energy/Forward")
		dbustools.Update(uphase.Imported, "kWh", "/Ac/"+uphase.Name+"/Energy/Reverse")

	}
}

func UpdateDbusGlobal() {

	var tKw float64
	var tImported float64
	var tExported float64
	for _, ph := range phase.Lines {
		tKw += ph.Power
		tExported += ph.Exported
		tImported += ph.Imported
	}

	totalMessages++
	dbustools.Update(tKw, "W", "/Ac/Power")
	log.WithFields(log.Fields{"W": tKw}).Debug("global Dbus update")
	if len(validLineImported) >= len(phase.Lines) {
		dbustools.Update(tExported, "kWh", "/Ac/Energy/Forward") //imported from grid
		log.WithFields(log.Fields{"forwared": tImported}).Debug("global Dbus update")
	}
	if len(validLineExported) >= len(phase.Lines) {
		dbustools.Update(tImported, "kWh", "/Ac/Energy/Reverse") //sold to grid
		log.WithFields(log.Fields{"reverse": tExported}).Debug("global Dbus update")
	}

}

func RandomString(n int) string {
	var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

	s := make([]rune, n)
	for i := range s {
		s[i] = letters[rand.Intn(len(letters))]
	}
	return string(s)
}
