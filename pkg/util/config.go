// Package util
// Copyright 2021 Juergen Enge, info-age GmbH, Basel. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package util

import (
	"emperror.dev/errors"
	"github.com/BurntSushi/toml"
	"github.com/je4/indexer/v3/pkg/indexer"
	"github.com/je4/utils/v2/pkg/zLogger"
	"os"
	"os/user"
	"path/filepath"
)

type Config struct {
	Indexer *indexer.IndexerConfig
	Log     zLogger.Config `toml:"log"`
}

func LoadConfig(tomlBytes []byte) (*Config, error) {
	var conf = &Config{
		Indexer: &indexer.IndexerConfig{},
		Log: zLogger.Config{
			Level: "DEBUG",
		},
	}

	if err := toml.Unmarshal(tomlBytes, conf); err != nil {
		return nil, errors.Wrapf(err, "Error unmarshalling config")
	}
	if conf.Indexer.Siegfried.SignatureFile == "" {
		user, err := user.Current()
		if err != nil {
			return nil, errors.Wrap(err, "cannot get current user")
		}
		fp := filepath.Join(user.HomeDir, "siegfried", "default.sig")
		fi, err := os.Stat(fp)
		if err == nil && !fi.IsDir() {
			conf.Indexer.Siegfried.SignatureFile = fp
		}
	}
	return conf, nil
}
