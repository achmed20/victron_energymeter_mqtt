package phase

import (
	"reflect"
	"strconv"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

var Lines []SinglePhase

type Topics struct {
	Voltage  string
	Current  string
	Power    string
	Imported string
	Exported string
}
type SinglePhase struct {
	Name     string
	Voltage  float64 // Volts: 230,0
	Current  float64 // Amps: 8,3
	Power    float64 // Watts: 1909
	Imported float64 // kWh, purchased power
	Exported float64 // kWh, sold power

	Topics Topics
}

func (s *SinglePhase) SetDefaults(def SinglePhase) {

	s.Voltage = def.Voltage
	s.Current = def.Current
	s.Power = def.Power

	s.Imported = def.Imported
	s.Exported = def.Exported
}

func (i *SinglePhase) SetByName(propName string, propValue float64) *SinglePhase {
	reflect.ValueOf(i).Elem().FieldByName(propName).Set(reflect.ValueOf(propValue))
	return i
}

func LoadConfig(lineDefaults map[string]interface{}) {
	for i := 1; i < 10; i++ {
		var lineName = "l" + strconv.Itoa(i)
		var lineVals = viper.GetStringMap(lineName)
		log.Trace("getting config for " + lineName)
		if len(lineVals) == 0 && lineName == "l1" {
			log.Panic("at least L1 required in config")
		} else if len(lineVals) == 0 {
			log.Trace("no more sequential Lines configured")
			return //no more configs
		}

		if len(lineVals) == 0 {
			Lines = append(Lines, SinglePhase{
				Name:    "L" + strconv.Itoa(i),
				Voltage: Lines[0].Voltage,
			})
		} else {
			topics := lineVals["topic"].(map[string]interface{})
			Lines = append(Lines, SinglePhase{
				Name:     "L" + strconv.Itoa(i),
				Voltage:  lineVals["voltage"].(float64),
				Current:  lineVals["current"].(float64),
				Power:    lineVals["power"].(float64),
				Imported: lineVals["imported"].(float64),
				Exported: lineVals["exported"].(float64),

				Topics: Topics{
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
