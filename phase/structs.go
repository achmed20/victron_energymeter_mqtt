package phase

import (
	"reflect"
	"strconv"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

var Lines []SinglePhase

type Topics struct {
	Voltage  string `json:"voltage,omitempty"`
	Current  string `json:"current,omitempty"`
	Power    string `json:"power,omitempty"`
	Imported string `json:"imported,omitempty"`
	Exported string `json:"exported,omitempty"`
}
type SinglePhase struct {
	Name     string  `json:"name,omitempty"`
	Voltage  float64 `json:"voltage,omitempty"`  // Volts: 230,0
	Current  float64 `json:"current,omitempty"`  // Amps: 8,3
	Power    float64 `json:"power,omitempty"`    // Watts: 1909
	Imported float64 `json:"imported,omitempty"` // kWh, purchased power
	Exported float64 `json:"exported,omitempty"` // kWh, sold power

	Topics Topics `json:"topics,omitempty"`
}

func init() {

}

func (i *SinglePhase) SetByName(propName string, propValue float64) *SinglePhase {
	reflect.ValueOf(i).Elem().FieldByName(propName).Set(reflect.ValueOf(propValue))
	return i
}

func (s *SinglePhase) FixValues() {
	// if s.Voltage == 0 {
	// 	log.Trace("Voltage missing, setting default value of 230")
	// 	s.Voltage = 230
	// }
	if s.Power != 0 && s.Current == 0 {
		log.Trace("current missing, calculating value")
		s.Current = s.Power / s.Voltage
	}
	// if s.Current != 0 && s.Power == 0 {
	// 	log.Debug("power missing, calculating value")
	// 	s.Power = s.Voltage * s.Current
	// }
}

func LoadConfig() {
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
