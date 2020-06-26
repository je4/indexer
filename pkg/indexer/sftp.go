package indexer

import (
	"fmt"
	"github.com/goph/emperror"
	"github.com/op/go-logging"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
	"io"
	"io/ioutil"
	"net/url"
	"os"
)

type SFTP struct {
	config *ssh.ClientConfig
	log    *logging.Logger
}

type SFTPClient struct {
	sftpclient *sftp.Client
	sshclient  *ssh.Client
}

func NewSFTP(PrivateKey []string, Password, KnownHosts string, log *logging.Logger) (*SFTP, error) {
	var signer []ssh.Signer
	var hostKeyCallback ssh.HostKeyCallback
	var err error

	sftp := &SFTP{log: log}
	for _, pk := range PrivateKey {
		key, err := ioutil.ReadFile(pk)
		if err != nil {
			return nil, emperror.Wrapf(err, "cannot read private key file %s")
		}
		// Create the Signer for this private key.
		s, err := ssh.ParsePrivateKey(key)
		if err != nil {
			return nil, emperror.Wrapf(err, "unable to parse private key %v", string(key))
		}
		signer = append(signer, s)
	}
	if KnownHosts != "" {
		hostKeyCallback, err = knownhosts.New(KnownHosts)
		if err != nil {
			return nil, emperror.Wrapf(err, "could not create hostkeycallback function for %s", KnownHosts)
		}
	}
	sftp.config = &ssh.ClientConfig{
		Auth:            []ssh.AuthMethod{},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	if hostKeyCallback != nil {
		sftp.config.HostKeyCallback = hostKeyCallback
	}
	if len(signer) > 0 {
		sftp.config.Auth = append(sftp.config.Auth, ssh.PublicKeys(signer...))
	}
	if Password != "" {
		sftp.config.Auth = append(sftp.config.Auth, ssh.Password(Password))
	}
	return sftp, nil
}

func (s *SFTP) Connect(address, user string) (*SFTPClient, error) {
	var err error
	sc := &SFTPClient{}
	s.config.User = user
	sc.sshclient, err = ssh.Dial("tcp", address, s.config)
	if err != nil {
		return nil, emperror.Wrapf(err, "unable to connect to %v", address)
	}
	sc.sftpclient, err = sftp.NewClient(sc.sshclient)
	if err != nil {
		return nil, emperror.Wrap(err, "unable to create SFTP session")
	}
	return sc, nil
}

func (sc *SFTPClient) Close()  {
	sc.sftpclient.Close()
	sc.sshclient.Close()
}

func (sc *SFTPClient) ReadFile(path string, w io.Writer) (int64, error)  {
	r, err := sc.sftpclient.Open(path)
	if err != nil {
		return 0, emperror.Wrapf(err, "cannot open remote file %s", path)
	}
	defer r.Close()
	written, err := io.Copy(w, r)
	if err != nil {
		return 0, emperror.Wrap(err, "cannot copy data")
	}
	return written, nil
}

func (sc *SFTPClient) WriteFile(path string, r io.Reader) (int64, error)  {
	w, err := sc.sftpclient.Create(path)
	if err != nil {
		return 0, emperror.Wrapf(err, "cannot create remote file %s", path)
	}
	written, err := io.Copy(w, r)
	if err != nil {
		return 0, emperror.Wrap(err, "cannot copy data")
	}
	return written, nil
}


func (s *SFTP) Get(uri url.URL, user string, w io.Writer) (int64, error) {
	if uri.Scheme != "sftp" {
		return 0, fmt.Errorf("invalid uri scheme %s for sftp", uri.Scheme)
	}
	client, err := s.Connect(uri.Host, user)
	if err != nil {
		return 0, emperror.Wrapf(err, "unable to connect to %v with user %v", uri.Host, user)
	}
	defer client.Close()

	written, err := client.ReadFile(uri.Path, w)
	if err != nil {
		return 0, emperror.Wrapf(err, "cannot read data from %v", uri.Path)
	}
	return written, nil
}

func (s *SFTP) GetFile(uri url.URL, user string, target string) (int64, error) {
	f, err := os.Create(target)
	if err != nil {
		return 0, emperror.Wrapf(err, "cannot create file %s", target)
	}
	defer f.Close()
	return s.Get(uri, user, f)
}

func (s *SFTP) PutFile(uri url.URL, user string, source string) (int64, error) {
	f, err := os.Open(source)
	if err != nil {
		return 0, emperror.Wrapf(err, "cannot open file %s", source)
	}
	defer f.Close()
	return s.Put(uri, user, f)
}

func (s *SFTP) Put(uri url.URL, user string, r io.Reader) (int64, error) {
	if uri.Scheme != "sftp" {
		return 0, fmt.Errorf("invalid uri scheme %s for sftp", uri.Scheme)
	}
	client, err := s.Connect(uri.Host, user)
	if err != nil {
		return 0, emperror.Wrapf(err, "unable to connect to %v with user %v", uri.Host, user)
	}
	defer client.Close()

	return client.WriteFile(uri.Path, r)
}
