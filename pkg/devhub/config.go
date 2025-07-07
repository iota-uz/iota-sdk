package devhub

type Config struct {
	Services []ServiceConfig `yaml:"services"`
}

type ServiceConfig struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Port        string   `yaml:"port"`
	Command     string   `yaml:"command"`
	Args        []string `yaml:"args"`
}
