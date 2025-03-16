package builder

type OGImageConfig struct {
	IconPath string  `yaml:"IconPath"`
	FontPath string  `yaml:"FontPath"`
	FontSize float64 `yaml:"FontSize"`
}

type Config struct {
	InputDirectory string        `yaml:"InputDirectory"`
	BaseURL        string        `yaml:"BaseURL"`
	OGImageConfig  OGImageConfig `yaml:"OGImageConfig"`
}
