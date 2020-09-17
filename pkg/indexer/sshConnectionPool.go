package indexer

import (
	"fmt"
	"github.com/goph/emperror"
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
	conn, err = NewSSHConnection(address, user, config, cp.log)
	if err != nil {
		return nil, emperror.Wrapf(err, "cannot open ssh connection")
	}
	cp.table[id] = conn
	return conn, nil
}
