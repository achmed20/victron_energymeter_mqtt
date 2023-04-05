package config

import (
	"victron_energymeter_mqtt/phase"
)

type Config struct {
	Updates int
	DryRun  bool
	Name    string

	Logging LogConfig
	Mqtt    MqttConfig

	Factors FactorConfig

	Phases []phase.SinglePhase
}

type FactorConfig struct {
	Imported float64
	Exported float64
}

type LogConfig struct {
	Level    string `json:"level,omitempty"`
	Interval int    `json:"interval,omitempty"`
}

type MqttConfig struct {
	Broker   string `json:"broker"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	Topic    string `json:"topic"`
}

func NewConfig() *Config {
	var conf Config
	return &conf
}

func (c *Config) SetDefaults() {
	c.Updates = 0    //live updates
	c.DryRun = false //no dbus
	c.Name = "victron_energymeter_mqtt"

	c.Logging.Interval = 3600
	c.Logging.Level = "info"

	c.Factors.Imported = 1
	c.Factors.Exported = 1

	//MQTT values
	c.Mqtt.Broker = "localhost"
	c.Mqtt.Port = 1883
	c.Mqtt.Topic = "stromzaehler/#"
	c.Mqtt.User = ""
	c.Mqtt.Password = ""
}

func (c *Config) FixValues() {
	if c.Updates < 250 && c.Updates != 0 {
		c.Updates = 250
	}

	if c.Logging.Interval == 0 {
		c.Logging.Interval = 3600
	}

}
