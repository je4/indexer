// Copyright 2020 Juergen Enge, info-age GmbH, Basel. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
package main

import (
	"github.com/BurntSushi/toml"
	"log"
	"time"
)

type duration struct {
	Duration time.Duration
}

func (d *duration) UnmarshalText(text []byte) error {
	var err error
	d.Duration, err = time.ParseDuration(string(text))
	return err
}

type ConfigSiegfried struct {
	Address string
}

type ConfigTika struct {
	Address    string
	Timeout    duration
	RegexpMime string
}

type ConfigFFMPEG struct {
	FFProbe string
	Wsl     bool
	Timeout duration
}

type ConfigImageMagick struct {
	Identify string
	Convert  string
	Wsl      bool
	Timeout  duration
}

type Config struct {
	ErrorTemplate   string
	Logfile         string
	Loglevel        string
	AccessLog       string
	CertPEM         string
	KeyPEM          string
	Addr            string
	JwtKey          string
	JwtAlg          []string
	TempDir         string
	HeaderTimeout   duration
	HeaderSize      int64
	DownloadMime    string
	MaxDownloadSize int64
	Siegfried       ConfigSiegfried
	FFMPEG          ConfigFFMPEG
	ImageMagick     ConfigImageMagick
	Tika            ConfigTika
}

func LoadConfig(filepath string) Config {
	var conf Config
	_, err := toml.DecodeFile(filepath, &conf)
	if err != nil {
		log.Fatalln("Error on loading config: ", err)
	}
	return conf
}
