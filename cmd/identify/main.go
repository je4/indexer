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
	"context"
	"flag"
	"fmt"
	badger "github.com/dgraph-io/badger/v3"
	"github.com/je4/indexer/pkg/indexer"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

const INDEXER = "indexer v0.2, info-age GmbH Basel"

func main() {
	println(INDEXER)

	configFile := flag.String("cfg", "./indexer.toml", "config file location")
	flag.Parse()

	var exPath = ""
	// if configfile not found try path of executable as prefix
	if !indexer.FileExists(*configFile) {
		ex, err := os.Executable()
		if err != nil {
			panic(err)
		}
		exPath = filepath.Dir(ex)
		if indexer.FileExists(filepath.Join(exPath, *configFile)) {
			*configFile = filepath.Join(exPath, *configFile)
		} else {
			log.Fatalf("cannot find configuration file: %v", *configFile)
			return
		}
	}
	// configfile should exists at this place
	var config Config
	config = LoadConfig(*configFile)

	// create logger instance
	log, lf := indexer.CreateLogger("indexer", config.Logfile, config.Loglevel)
	defer lf.Close()

	var accesslog io.Writer
	if config.AccessLog == "" {
		accesslog = os.Stdout
	} else {
		f, err := os.OpenFile(config.AccessLog, os.O_WRONLY|os.O_CREATE, 0755)
		if err != nil {
			log.Panicf("cannot open file %s: %v", config.AccessLog, err)
			return
		}
		defer f.Close()
		accesslog = f
	}

	mapping := map[string]string{}
	for _, val := range config.FileMap {
		mapping[strings.ToLower(val.Alias)] = val.Folder
	}
	fm := indexer.NewFileMapper(mapping)

	sftp, err := indexer.NewSFTP(config.SFTP.PrivateKey, config.SFTP.Password, config.SFTP.Knownhosts, log)
	if err != nil {
		log.Panicf("cannot initialize sftp client: %v", err)
		return
	}

	stat2, err := os.Stat(config.NSRLBadger)
	if err != nil {
		fmt.Printf("cannot stat badger folder %s: %v\n", config.NSRLBadger, err)
		return
	}
	if !stat2.IsDir() {
		fmt.Printf("%s is not a directory\n", config.NSRLBadger)
		return
	}

	srv, err := indexer.NewServer(
		config.HeaderTimeout.Duration,
		config.HeaderSize,
		config.DownloadMime,
		config.MaxDownloadSize,
		config.JwtKey,
		config.JwtAlg,
		config.InsecureCert,
		log,
		accesslog,
		config.ErrorTemplate,
		config.TempDir,
		fm,
		sftp,
	)
	if err != nil {
		log.Panicf("cannot initialize server: %v", err)
		return
	}

	var nsrldb *badger.DB
	if config.NSRLBadger != "" {
		bconfig := badger.DefaultOptions(config.NSRLBadger)
		bconfig.ReadOnly = true
		nsrldb, err = badger.Open(bconfig)
		if err != nil {
			log.Panicf("cannot open badger database in %s: %v\n", config.NSRLBadger, err)
			return
		}
		//log.Infof("nsrl max batch count: %v", nsrldb.MaxBatchCount())
		defer nsrldb.Close()

		indexer.NewActionNSRL(nsrldb, srv)
		//return
	}

	if config.Siegfried.Enabled {
		indexer.NewActionSiegfried(config.Siegfried.Address, srv)
		//srv.AddAction(sf)
	}

	if config.FFMPEG.Enabled {
		indexer.NewActionFFProbe(
			config.FFMPEG.FFProbe,
			config.FFMPEG.Wsl,
			config.FFMPEG.Timeout.Duration,
			config.FFMPEG.Online,
			srv)
		//srv.AddAction(ffprobe)
	}

	if config.ImageMagick.Enabled {
		indexer.NewActionIdentify(
			config.ImageMagick.Identify,
			config.ImageMagick.Convert,
			config.ImageMagick.Wsl,
			config.ImageMagick.Timeout.Duration,
			config.ImageMagick.Online,
			srv)
		//srv.AddAction(identify)
	}

	if config.Tika.Enabled {
		indexer.NewActionTika(
			config.Tika.Address,
			config.Tika.Timeout.Duration,
			config.Tika.RegexpMime,
			config.Tika.Online,
			srv)
		//srv.AddAction(tika)
	}

	for _, eaconfig := range config.External {
		var caps indexer.ActionCapability
		for _, c := range eaconfig.ActionCapabilities {
			caps |= c
		}
		indexer.NewActionExternal(eaconfig.Name, eaconfig.Address, caps, eaconfig.CallType, eaconfig.Mimetype, srv)
		//srv.AddAction(ea)
	}

	go func() {
		if err := srv.ListenAndServe(config.Addr, config.CertPEM, config.KeyPEM); err != nil {
			log.Errorf("server died: %v", err)
		}
	}()

	end := make(chan bool, 1)

	// process waiting for interrupt signal (TERM or KILL)
	go func() {
		sigint := make(chan os.Signal, 1)

		// interrupt signal sent from terminal
		signal.Notify(sigint, os.Interrupt)

		signal.Notify(sigint, syscall.SIGTERM)
		signal.Notify(sigint, syscall.SIGKILL)

		<-sigint

		// We received an interrupt signal, shut down.
		log.Infof("shutdown requested")
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		srv.Shutdown(ctx)

		end <- true
	}()

	<-end
	log.Info("server stopped")
}
