package indexer

import (
	"crypto/sha1"
	"encoding/json"
	"fmt"
	badger "github.com/dgraph-io/badger/v3"
	"github.com/golang/snappy"
	"github.com/goph/emperror"
	"io"
	"net/url"
	"os"
	"time"
)

const NSRL_OS = "OpSystemCode-"
const NSRL_PROD = "ProductCode-"
const NSRL_MFG = "MFgCode-"
const NSRL_File = "SHA-1-"

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
	an := &ActionNSRL{name: "nsrl", nsrldb: nsrldb, server: server, caps: ACTFILE}
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

func (aNSRL *ActionNSRL) getNSRL(sha1sum string) (interface{}, error) {
	var result []ActionNSRLMeta
	aNSRL.nsrldb.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(NSRL_File + sha1sum))
		if err != nil {
			return emperror.Wrapf(err, "cannot get os %s", NSRL_File+sha1sum)
		}
		var fileData []map[string]string
		if err := item.Value(func(val []byte) error {
			jsonStr, err := snappy.Decode(nil, val)
			if err != nil {
				return emperror.Wrapf(err, "cannot decompress snappy of %s", NSRL_File+sha1sum)
			}
			if err := json.Unmarshal([]byte(jsonStr), &fileData); err != nil {
				return emperror.Wrapf(err, "cannot unmarshal %s for %s", jsonStr, NSRL_File+sha1sum)
			}
			return nil
		}); err != nil {
			return emperror.Wrapf(err, "cannot get value of %s", NSRL_File+sha1sum)
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
		filename, err := aNSRL.server.fm.Get(uri)
		if err != nil {
			return nil, emperror.Wrapf(err, "no file url")
		}
		fp, err := os.Open(filename)
		if err != nil {
			return nil, emperror.Wrapf(err, "cannot open file %s", filename)
		}
		defer fp.Close()
		hash := sha1.New()
		if _, err := io.Copy(hash, fp); err != nil {
			return nil, emperror.Wrapf(err, "cannot read %s", filename)
		}
		sum := hash.Sum(nil)
		sumStr := fmt.Sprintf("%X", sum)
		checksums = make(map[string]string)
		checksums["SHA-1"] = sumStr
		//return nil, fmt.Errorf("need checksum (md5) to check nsrl")
	}

	SHA1sumStr, ok := checksums["SHA-1"]
	if !ok {
		return nil, fmt.Errorf("no md5 checksum given to check nsrl")
	}
	/*
		result := make([]map[string]interface{}, 0)
		if err := aNSRL.nsrldb.View(func(txn *badger.Txn) error {
			item, err := txn.Get([]byte("SHA-1-"+SHA1sumStr))
			if err == badger.ErrKeyNotFound {
				return nil
			} else if err != nil {
				return emperror.Wrapf(err, "cannot lookup checksum SHA-1-%s", SHA1sumStr)
			}
			if err := item.Value(func(val []byte) error {
				if err := json.Unmarshal(val, &result); err != nil {
					return emperror.Wrapf(err, "cannot unmarshal %s", string(val))
				}
				return nil
			}); err != nil {
				return emperror.Wrapf(err, "cannot get value of nsrl from SHA-1-%s", SHA1sumStr)
			}
			return nil
		}); err != nil {
			return nil, emperror.Wrapf(err, "cannot get entry for SHA-1-%s", sumStr)
		}
	*/

	return aNSRL.getNSRL(SHA1sumStr)
}
