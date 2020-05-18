package crossplane

type Payload struct {
	Status string         `json:"status"`
	Errors []PayloadError `json:"errors"`
	Config []Config       `json:"config"`
}

type PayloadError struct {
	File     string      `json:"file"`
	Line     *int        `json:"line"`
	Error    string      `json:"error"`
	Callback interface{} `json:"callback,omitempty"`
}

type Config struct {
	File   string        `json:"file"`
	Status string        `json:"status"`
	Errors []ConfigError `json:"errors"`
	Parsed []Directive   `json:"parsed"`
}

type ConfigError struct {
	Line  *int   `json:"line"`
	Error string `json:"error"`
}

type Directive struct {
	Directive string       `json:"directive"`
	Line      int          `json:"line"`
	Args      []string     `json:"args"`
	Includes  *[]int       `json:"includes,omitempty"`
	Block     *[]Directive `json:"block,omitempty"`
	Comment   *string      `json:"comment,omitempty"`
}

func (d Directive) IsBlock() bool {
	return d.Block != nil
}

func (d Directive) IsInclude() bool {
	return d.Includes != nil
}

func (d Directive) IsComment() bool {
	return d.Comment != nil
}
