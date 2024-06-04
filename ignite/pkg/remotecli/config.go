package remotecli

import (
	"bytes"
	"os"
	"path"

	"github.com/pelletier/go-toml/v2"

	"github.com/ignite/cli/v29/ignite/config"
	"github.com/ignite/cli/v29/ignite/pkg/errors"
)

type (
	Config struct {
		Chains Chains `toml:"chains"`
	}
	Chains map[string]interface{}
)

var EmptyConfig = &Config{
	Chains: Chains{},
}

func GetConfigDir() (string, error) {
	globalPath, err := config.DirPath()
	if err != nil {
		return "", err
	}

	configDir := path.Join(globalPath, "remotecli")
	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		return configDir, os.MkdirAll(configDir, 0o750)
	}

	return configDir, nil
}

func Load(configDir string) (*Config, error) {
	configPath := configFilename(configDir)

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return EmptyConfig, nil
	}

	bz, err := os.ReadFile(configPath)
	if err != nil {
		return nil, errors.Wrapf(err, "can't read config file: %s", configPath)
	}

	c := &Config{}
	if err = toml.Unmarshal(bz, c); err != nil {
		return nil, errors.Wrapf(err, "can't load config file: %s", configPath)
	}

	return c, err
}

func Save(configDir string, config *Config) error {
	var (
		buf = &bytes.Buffer{}
		enc = toml.NewEncoder(buf)
	)
	if err := enc.Encode(config); err != nil {
		return err
	}

	if err := os.MkdirAll(configDir, 0o750); err != nil {
		return err
	}

	configPath := configFilename(configDir)
	if err := os.WriteFile(configPath, buf.Bytes(), 0o600); err != nil {
		return err
	}

	return nil
}

func configFilename(configDir string) string {
	return path.Join(configDir, "config.toml")
}
