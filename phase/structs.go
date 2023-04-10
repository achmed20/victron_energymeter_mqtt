package phase

import (
	"reflect"
	"time"

	log "github.com/sirupsen/logrus"
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

	lastPowerUpdate time.Time
}

func init() {

}

func (i *SinglePhase) SetByName(propName string, propValue float64) *SinglePhase {
	reflect.ValueOf(i).Elem().FieldByName(propName).Set(reflect.ValueOf(propValue))
	if propName == "Power" {
		i.lastPowerUpdate = time.Now()
	}
	return i
}

func (i SinglePhase) GetLastPowerUpdate() time.Time {
	return i.lastPowerUpdate
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
