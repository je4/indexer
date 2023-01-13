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
	"strings"
	"sync"
)

type SSHConnectionPool struct {
	// Protects access to fields below
	mu    sync.Mutex
	table map[string]*SSHConnection
	log   *logging.Logger
}

func NewSSHConnectionPool(log *logging.Logger) *SSHConnectionPool {
	return &SSHConnectionPool{
		mu:    sync.Mutex{},
		table: map[string]*SSHConnection{},
		log:   log,
	}
}

func (cp *SSHConnectionPool) GetConnection(address, user string, config *ssh.ClientConfig) (*SSHConnection, error) {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	id := strings.ToLower(fmt.Sprintf("%s@%s", user, address))

	conn, ok := cp.table[id]
	if ok {
		return conn, nil
	}
	var err error
	cp.log.Infof("new ssh connection to %v", id)
	conn, err = NewSSHConnection(address, user, config, cp.log)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot open ssh connection")
	}
	cp.table[id] = conn
	return conn, nil
}
