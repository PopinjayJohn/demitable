// Package pack: dice configuration (ADR-0008, ADR-0022).
//
// Phase 2: a pack may now declare a `dice:` block. Packs without one
// (phase-1 packs) keep loading unchanged: Pack.Dice is nil and there are
// no roll workflows.
package pack

// DiceConfig is the pack-level dice block.
type DiceConfig struct {
	Default          string           `yaml:"default"`
	Advantage        *AdvantageConfig `yaml:"advantage"`
	Critical         *CriticalConfig  `yaml:"critical"`
	Direction        string           `yaml:"direction"` // high | low
	DegreesOfSuccess []string         `yaml:"degreesOfSuccess"`
	Workflows        []DiceWorkflow   `yaml:"workflows"`
	Sets             []DiceSet        `yaml:"sets"`
	Tables           []DiceTable      `yaml:"tables"`
}

// AdvantageConfig describes an advantage/disadvantage roll.
type AdvantageConfig struct {
	Mode  string `yaml:"mode"` // keep-highest | keep-lowest
	Dice  int    `yaml:"dice"`
	Sides int    `yaml:"sides"`
}

// CriticalConfig describes critical-hit extra dice.
type CriticalConfig struct {
	When   string `yaml:"when"` // natural-max | natural-min | none
	Effect string `yaml:"effect"`
}

// DiceWorkflow is one named roll workflow.
type DiceWorkflow struct {
	Key       string `yaml:"key"`
	Label     string `yaml:"label"`
	Dice      string `yaml:"dice"`
	Against   string `yaml:"against"`
	Bonus     string `yaml:"bonus"`
	OnSuccess string `yaml:"onSuccess"`
	Vs        string `yaml:"vs"`
	Outcome   string `yaml:"outcome"`
	Direction string `yaml:"direction"` // high | low, overrides pack default
}

// DiceSet declares a named die set (ADR-0022).
type DiceSet struct {
	Key     string `yaml:"key"`
	Sides   int    `yaml:"sides"`
	Numeric bool   `yaml:"numeric"`
	Table   string `yaml:"table"`
	Ranked  bool   `yaml:"ranked"`
}

// DiceTable is a table shipped by a pack (shadows? vault shadows pack).
type DiceTable struct {
	ID    string `yaml:"id"`
	Name  string `yaml:"name"`
	Faces []any  `yaml:"faces"`
}
