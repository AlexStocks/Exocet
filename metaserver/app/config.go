package main

import (
	"io/ioutil"
)

import (
	"gopkg.in/yaml.v2"
)

// ConfYaml is config structure.
type ConfYaml struct {
	Core  SectionCore  `yaml:"core"`
	Redis SectionRedis `yaml:"redis"`
}

// SectionPID is sub section of config.
type SectionPID struct {
	Enabled  bool   `yaml:"enabled"`
	Path     string `yaml:"path"`
	Override bool   `yaml:"override"`
}

// SectionCore is sub section of config.
type SectionCore struct {
	Mode            string     `yaml:"mode"`
	FailFastTimeout int        `yaml:"fail_fast_timeout"`
	PID             SectionPID `yaml:"pid"`
}

// SectionKafka is sub section of config.
type SectionRedis struct {
	Sentinels []string `yaml:"sentinel"`
	MetaDB    string   `yaml:"meta_db"`
}

// LoadConfYaml provide load yml config.
func LoadConfYaml(confPath string) (ConfYaml, error) {
	var config ConfYaml

	configFile, err := ioutil.ReadFile(confPath)

	if err != nil {
		return config, err
	}

	err = yaml.Unmarshal(configFile, &config)

	if err != nil {
		return config, err
	}

	return config, nil
}
