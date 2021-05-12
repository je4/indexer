package indexer

import (
	"encoding/json"
	"fmt"
	badger "github.com/dgraph-io/badger/v3"
	"github.com/golang/snappy"
	"github.com/goph/emperror"
	"net/url"
	"time"
)

const NSRL_OS = "OpSystemCode-"
const NSRL_PROD = "ProductCode-"
const NSRL_MFG = "MFgCode-"
const NSRL_File = "MD5-"

type ActionNSRL struct {
	name   string
	caps   ActionCapability
	server *Server
	nsrldb *badger.DB
}

type ActionNSRLMeta struct {
	File    map[string]string
	FileMfG map[string]string
	OS      map[string]string
	OSMfg   map[string]string
	Prod    map[string]string
	ProdMfg map[string]string
}

func NewActionNSRL(nsrldb *badger.DB, server *Server) Action {
	an := &ActionNSRL{name: "NSRL", nsrldb: nsrldb, server: server, caps: ACTALL}
	server.AddAction(an)
	return an
}

func getStringMap(txn *badger.Txn, key string) (map[string]string, error) {
	var result map[string]string
	item, err := txn.Get([]byte(key))
	if err != nil {
		if err == badger.ErrKeyNotFound {
			return nil, nil
		}
		return nil, emperror.Wrapf(err, "cannot get %s", key)
	}
	if err := item.Value(func(val []byte) error {
		jsonStr, err := snappy.Decode(nil, val)
		if err != nil {
			return emperror.Wrapf(err, "cannot decompress snappy of %s", key)
		}
		if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
			return emperror.Wrapf(err, "cannot unmarshal %s for %s", jsonStr, key)
		}
		return nil
	}); err != nil {
		return nil, emperror.Wrapf(err, "cannot get data of %s", key)
	}
	return result, nil
}

func (aNSRL *ActionNSRL) getNSRL(md5 string) (interface{}, error) {
	var result []ActionNSRLMeta
	aNSRL.nsrldb.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(NSRL_File + md5))
		if err != nil {
			return emperror.Wrapf(err, "cannot get os %s", NSRL_File+md5)
		}
		var fileData []map[string]string
		if err := item.Value(func(val []byte) error {
			jsonStr, err := snappy.Decode(nil, val)
			if err != nil {
				return emperror.Wrapf(err, "cannot decompress snappy of %s", NSRL_File+md5)
			}
			if err := json.Unmarshal([]byte(jsonStr), &fileData); err != nil {
				return emperror.Wrapf(err, "cannot unmarshal %s for %s", jsonStr, NSRL_File+md5)
			}
			return nil
		}); err != nil {
			return emperror.Wrapf(err, "cannot get value of %s", NSRL_File+md5)
		}

		for _, file := range fileData {
			var am ActionNSRLMeta
			am.File = file
			if am.File["MfgCode"] != "" {
				am.FileMfG, _ = getStringMap(txn, NSRL_MFG+am.Prod["MfgCode"])
			}
			am.Prod, err = getStringMap(txn, NSRL_PROD+file["ProductCode"])
			if err != nil {
				return emperror.Wrapf(err, "cannot get data of %s", NSRL_PROD+file["ProductCode"])
			}
			if am.Prod["MfgCode"] != "" {
				am.ProdMfg, _ = getStringMap(txn, NSRL_MFG+am.Prod["MfgCode"])
			}
			am.OS, err = getStringMap(txn, NSRL_OS+file["OpSystemCode"])
			if err != nil {
				return emperror.Wrapf(err, "cannot get data of %s", NSRL_PROD+file["ProductCode"])
			}
			if am.OS["MfgCode"] != "" {
				am.OSMfg, _ = getStringMap(txn, NSRL_MFG+am.Prod["MfgCode"])
			}
			result = append(result, am)
		}
		return nil
	})
	return result, nil
}

func (aNSRL *ActionNSRL) GetCaps() ActionCapability {
	return aNSRL.caps
}

func (aNSRL *ActionNSRL) GetName() string {
	return aNSRL.name
}

func (aNSRL *ActionNSRL) Do(uri *url.URL, mimetype *string, width *uint, height *uint, duration *time.Duration, checksums map[string]string) (interface{}, error) {
	if checksums == nil {
		return nil, fmt.Errorf("need checksum (md5) to check nsrl")
	}

	md5sumStr, ok := checksums["md5"]
	if !ok {
		return nil, fmt.Errorf("no md5 checksum given to check nsrl")
	}

	return aNSRL.getNSRL(md5sumStr)
}
