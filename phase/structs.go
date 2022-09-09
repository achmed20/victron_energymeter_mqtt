package phase

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
