package main

import (
	"github.com/BurntSushi/toml"
	"github.com/je4/filesystem/v3/pkg/vfsrw"
	"github.com/je4/indexer/v3/pkg/indexer"
	"github.com/je4/utils/v2/pkg/stashconfig"
	"log"
	"os"
	"os/user"
	"path/filepath"
)

type Config struct {
	ErrorTemplate string
	Indexer       indexer.IndexerConfig `toml:"indexer"`
	VFS           map[string]*vfsrw.VFS `toml:"vfs"`
	Log           stashconfig.Config    `toml:"log"`
}

func LoadConfig(fp string) *Config {
	user, err := user.Current()
	if err != nil {
		log.Fatalln("cannot get current user", err)
	}
	var conf = &Config{
		Indexer: indexer.IndexerConfig{
			Siegfried: indexer.ConfigSiegfried{
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
