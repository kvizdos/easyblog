package builder

type OGImageConfig struct {
	IconPath string  `yaml:"IconPath"`
	FontPath string  `yaml:"FontPath"`
	FontSize float64 `yaml:"FontSize"`
}

type StaticConfig struct {
	Path string `yaml:"Path"`
}

type Config struct {
	InputDirectory string        `yaml:"InputDirectory"`
	BaseURL        string        `yaml:"BaseURL"`
	OGImageConfig  OGImageConfig `yaml:"OGImageConfig"`
	CodeStyle      string        `yaml:"CodeStyle"` // Chroma Style
	StaticConfig   StaticConfig  `yaml:"StaticConfig"`
}
