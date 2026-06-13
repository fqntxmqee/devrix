package config

// Config is the hot-reloadable application configuration snapshot.
type Config struct {
	Log struct {
		Level string `yaml:"level"`
	} `yaml:"log"`
	LLM struct {
		Provider string `yaml:"provider"`
		Model    string `yaml:"model"`
	} `yaml:"llm"`
}
