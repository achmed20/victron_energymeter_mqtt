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

	vc "victron_energymeter_mqtt/config"
	"victron_energymeter_mqtt/dbustools"
	"victron_energymeter_mqtt/phase"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/fsnotify/fsnotify"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

/* Configuration */
var Config vc.Config

var Cache sync.Map
var totalMessages int

// [string]phaseCache

type phaseCache struct {
	Field string
	Valid bool
	Phase *phase.SinglePhase
}

var validLineImported map[string]*phase.SinglePhase
var validLineExported map[string]*phase.SinglePhase

func init() {
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
	viper.OnConfigChange(func(e fsnotify.Event) {
		log.Info("Config changed!")
		loadConfig()
	})
	viper.WatchConfig()

	Config.SetDefaults()
	loadConfig()

}

func main() {
	dbustools.Connect()
	defer dbustools.Close()
	log.Info("Successfully connected to dbus")
	// MQTT Subscripte
	opts := mqtt.NewClientOptions()
	opts.SetOrderMatters(false) //important or it will crash
	opts.AddBroker(fmt.Sprintf("tcp://%s:%d", Config.Mqtt.Broker, Config.Mqtt.Port))
	opts.SetClientID(Config.Name + RandomString(10))
	opts.SetUsername(Config.Mqtt.User)
	opts.SetPassword(Config.Mqtt.Password)
	opts.SetDefaultPublishHandler(messageHandler) //func that handles all messages
	opts.OnConnect = connectHandler
	opts.OnConnectionLost = connectLostHandler
	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		log.WithField("error", token.Error()).Panic("could not connect to MQTT server")
	}
	func(client mqtt.Client) {
		token := client.Subscribe(Config.Mqtt.Topic, 1, nil)
		token.Wait()
		log.Info("Subscribed to topic: " + Config.Mqtt.Topic)
	}(client)

	go func() {
		logTicker := time.NewTicker(time.Second * time.Duration(Config.Logging.Interval))
		for _ = range logTicker.C {
			if Config.CheckForUpdates && totalMessages == 0 {
				log.Fatal("No updates from MQTT topic. something is off ...")
			}
			log.WithField("updates_sent", totalMessages).Info("still allive")
			totalMessages = 0
		}
	}()

	if Config.Updates > 0 {
		go func() {
			updateTicker := time.NewTicker(time.Millisecond * time.Duration(Config.Updates))
			log.WithField("ms", Config.Updates).Info("update interval set to delayed")
			for _ = range updateTicker.C {
				for _, ph := range phase.Lines {
					UpdateDbusPhase(&ph)
				}
				UpdateDbusGlobal()
			}
		}()
	} else {
		log.WithField("ms", Config.Updates).Info("update interval set to LIVE")
	}

	// Wait for ctrl+c
	done := make(chan os.Signal, 1)
	signal.Notify(done, syscall.SIGINT, syscall.SIGTERM)
	<-done
	os.Exit(0)

}

// ------------------------------------------------------------------------------------

func loadConfig() {
	err := viper.ReadInConfig() // Find and read the config file
	if err != nil {             // Handle errors reading the config file
		panic(fmt.Errorf("fatal error config file: %w", err))
	}

	viper.Unmarshal(&Config)
	Config.FixValues()

	if Config.DryRun {
		log.Warn("dry run / dbus disabled")
		dbustools.DryRun = true
	}

	switch Config.Logging.Level {
	case "info":
		log.SetLevel(log.InfoLevel)
	case "debug":
		log.SetLevel(log.DebugLevel)
	case "warn":
		log.SetLevel(log.WarnLevel)
	case "trace":
		log.SetLevel(log.TraceLevel)
	default:
		log.SetOutput(ioutil.Discard)
	}

	log.Info(fmt.Sprintf("log interval set to %d", Config.Updates))

	// -------- setup phases -----------
	phase.Lines = Config.Phases
	Cache = sync.Map{}
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
	log.Info(fmt.Sprintf("Connected to broker"))
}

/* Called if connection is lost  */
var connectLostHandler mqtt.ConnectionLostHandler = func(client mqtt.Client, err error) {
	//panic and let the script restart
	log.Panic(fmt.Sprintf("Connect lost: %v", err))
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

			//handle Factors
			switch ph.Field {
			case "Imported":
				payload = payload * Config.Factors.Imported
			case "Exported":
				payload = payload * Config.Factors.Exported
			}

			ph.Phase.SetByName(ph.Field, payload)
			switch ph.Field {
			case "Power":
				if Config.Updates == 0 {
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
	rand.Seed(time.Now().UnixNano())
	s := make([]rune, n)
	for i := range s {
		s[i] = letters[rand.Intn(len(letters))]
	}
	return string(s)
}
