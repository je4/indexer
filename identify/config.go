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
