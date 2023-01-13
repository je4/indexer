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
package indexer

import (
	"emperror.dev/errors"
	"fmt"
	"github.com/op/go-logging"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
	"io"
	"net/url"
	"os"
)

type SFTP struct {
	config *ssh.ClientConfig
	log    *logging.Logger
	pool   *SSHConnectionPool
}

func NewSFTP(PrivateKey []string, Password, KnownHosts string, log *logging.Logger) (*SFTP, error) {
	var signer []ssh.Signer

	sftp := &SFTP{
		log: log,
		config: &ssh.ClientConfig{
			Auth:            []ssh.AuthMethod{},
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		},
		pool: NewSSHConnectionPool(log),
	}

	for _, pk := range PrivateKey {
		key, err := os.ReadFile(pk)
		if err != nil {
			return nil, errors.Wrapf(err, "cannot read private key file %s")
		}
		// Create the Signer for this private key.
		s, err := ssh.ParsePrivateKey(key)
		if err != nil {
			return nil, errors.Wrapf(err, "unable to parse private key %v", string(key))
		}
		signer = append(signer, s)
	}
	if len(signer) > 0 {
		sftp.config.Auth = append(sftp.config.Auth, ssh.PublicKeys(signer...))
	}
	if KnownHosts != "" {
		hostKeyCallback, err := knownhosts.New(KnownHosts)
		if err != nil {
			return nil, errors.Wrapf(err, "could not create hostkeycallback function for %s", KnownHosts)
		}
		sftp.config.HostKeyCallback = hostKeyCallback
	}
	if Password != "" {
		sftp.config.Auth = append(sftp.config.Auth, ssh.Password(Password))
	}
	return sftp, nil
}

func (s *SFTP) GetConnection(address, user string) (*SSHConnection, error) {
	return s.pool.GetConnection(address, user, s.config)
}

func (s *SFTP) Get(uri url.URL, w io.Writer) (int64, error) {
	if uri.Scheme != "sftp" {
		return 0, fmt.Errorf("invalid uri scheme %s for sftp", uri.Scheme)
	}
	user := ""
	if uri.User != nil {
		user = uri.User.Username()
	}
	conn, err := s.GetConnection(uri.Host, user)
	if err != nil {
		return 0, errors.Wrapf(err, "unable to connect to %v with user %v", uri.Host, user)
	}

	written, err := conn.ReadFile(uri.Path, w)
	if err != nil {
		return 0, errors.Wrapf(err, "cannot read data from %v", uri.Path)
	}
	return written, nil
}

func (s *SFTP) GetFile(uri url.URL, user string, target string) (int64, error) {
	f, err := os.Create(target)
	if err != nil {
		return 0, errors.Wrapf(err, "cannot create file %s", target)
	}
	defer f.Close()
	return s.Get(uri, f)
}

func (s *SFTP) PutFile(uri url.URL, user string, source string) (int64, error) {
	f, err := os.Open(source)
	if err != nil {
		return 0, errors.Wrapf(err, "cannot open file %s", source)
	}
	defer f.Close()
	return s.Put(uri, user, f)
}

func (s *SFTP) Put(uri url.URL, user string, r io.Reader) (int64, error) {
	if uri.Scheme != "sftp" {
		return 0, fmt.Errorf("invalid uri scheme %s for sftp", uri.Scheme)
	}
	conn, err := s.GetConnection(uri.Host, user)
	if err != nil {
		return 0, errors.Wrapf(err, "unable to connect to %v with user %v", uri.Host, user)
	}
	return conn.WriteFile(uri.Path, r)
}
