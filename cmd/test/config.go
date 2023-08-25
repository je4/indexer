package main

import (
	"github.com/BurntSushi/toml"
	indexerConfig "github.com/je4/indexer/v2/pkg/config"
	"log"
	"os"
	"os/user"
	"path/filepath"
)

type Config struct {
	ErrorTemplate string
	Logfile       string
	Loglevel      string
	LogFormat     string
	Indexer       indexerConfig.Indexer `toml:"indexer"`
}

func LoadConfig(fp string) *Config {
	user, err := user.Current()
	if err != nil {
		log.Fatalln("cannot get current user", err)
	}
	var conf = &Config{
		Loglevel:  "DEBUG",
		LogFormat: `%{time:2006-01-02T15:04:05.000} %{shortpkg}::%{longfunc} [%{shortfile}] > %{level:.5s} - %{message}`,
		Indexer: indexerConfig.Indexer{
			Siegfried: indexerConfig.ConfigSiegfried{
				SignatureFile: filepath.Join(user.HomeDir, "siegfried", "default.sig"),
			},
			TempDir: os.TempDir(),
		},
	}

	if _, err := toml.DecodeFile(fp, conf); err != nil {
		log.Fatalln("Error on loading config: ", err)
	}

	return conf
}
