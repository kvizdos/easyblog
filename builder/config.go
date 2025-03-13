package builder

type OGImageConfig struct {
	IconPath string `yaml:"IconPath"`
	FontPath string `yaml:"FontPath"`
}

type Config struct {
	InputDirectory string        `yaml:"InputDirectory"`
	BaseURL        string        `yaml:"BaseURL"`
	OGImageConfig  OGImageConfig `yaml:"OGImageConfig"`
}
