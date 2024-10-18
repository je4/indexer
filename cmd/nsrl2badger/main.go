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
package main

import (
	"archive/zip"
	"emperror.dev/errors"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	badger "github.com/dgraph-io/badger/v4"
	"github.com/golang/snappy"
	"github.com/hooklift/iso9660"
	"io"
	"os"
	"strings"
)

const NSRL2BADGER = "nsrl2badger v0.2, info-age GmbH Basel"

func contains(s []int, e int) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

func appendIfNotExists(ls []map[string]string, els ...map[string]string) []map[string]string {
	for _, el := range els {
		found := false
		for _, l := range ls {
			same := true
			for key, val := range l {
				if el[key] != val {
					same = false
					break
				}
			}
			if same {
				found = true
				break
			}
		}
		if !found {
			ls = append(ls, el)
		}
	}
	return ls
}

func copyCSV(indexField string, db *badger.DB, freader io.Reader, ignore []int, checkonly bool) error {
	defer fmt.Println()
	var counter int64
	csvR := csv.NewReader(freader)
	fields, err := csvR.Read()
	if err != nil {
		return errors.Wrapf(err, "cannot read csv fields from %v", freader)
	}
	indexId := 0
	if indexField != "" {
		for key, val := range fields {
			if val == indexField {
				fmt.Printf("%v: %s == %s\n", key, val, indexField)
				indexId = key
				break
			}
		}
	}
	fmt.Printf("indexfield: %s --> %v:%s\n", indexField, indexId, fields[indexId])
	for {
		done := false
		num := 1000
		d := make(map[string][]map[string]string)
		if err := db.View(func(txn *badger.Txn) error {
			for {
				dataStruct := make(map[string]string)
				data, err := csvR.Read()
				if err == io.EOF {
					done = true
					break
				}
				if err != nil {
					fmt.Printf("cannot read csv data: %v", err)
					continue
				}
				for key, name := range fields {
					if contains(ignore, key) {
						continue
					}
					dataStruct[name] = data[key]
				}
				daKey := fields[indexId] + "-" + data[indexId]
				if _, ok := d[daKey]; !ok {
					d[daKey] = make([]map[string]string, 0)
					if !checkonly {
						item, err := txn.Get([]byte(daKey))
						if err != nil {
							if err != badger.ErrKeyNotFound {
								return errors.Wrapf(err, "cannot get key %s", daKey)
							}
						} else {
							item.Value(func(val []byte) error {
								dec, err := snappy.Decode(nil, val)
								if err != nil {
									return errors.Wrapf(err, "cannot decode %s", val)
								}
								var ds []map[string]string
								if err := json.Unmarshal(dec, &ds); err != nil {
									return errors.Wrapf(err, "cannot unmarshal %s", string(val))
								}
								_list0 := d[daKey]
								_list := appendIfNotExists(_list0, ds...)
								d[daKey] = _list
								return nil
							})
						}
					}
				}
				_list0 := d[daKey]
				if !checkonly || len(_list0) == 0 {
					_list := appendIfNotExists(_list0, dataStruct)
					d[daKey] = _list
				}
				num--
				if num <= 0 {
					break
				}
			}
			return nil
		}); err != nil {
		}
		if err := db.Update(func(txn *badger.Txn) error {
			for key, list := range d {
				fmt.Printf("%v: %s [%v]    \r", counter, key, len(list))
				counter++
				d, err := json.Marshal(list)
				if err != nil {
					return errors.Wrapf(err, "cannot marshal data %v", list)
				}
				d2 := snappy.Encode(nil, d)
				if err := txn.Set([]byte(key), d2); err != nil {
					return errors.Wrapf(err, "cannot store struct %v", d)
				}
			}
			return nil
		}); err != nil {
			return errors.Wrapf(err, "cannot store csv data")
		}
		if done {
			break
		}
	}
	return nil
}

func isoZip(isoFile, badgerFolder, fileFile, osFile, mfgFile, prodFile, checksum string, noFile, checkonly bool) error {

	switch strings.ToUpper(checksum) {
	case "MD5":
	case "SHA-1":
	case "":
	default:
		return fmt.Errorf("invalid checksum type: %s", checksum)
	}

	stat, err := os.Stat(isoFile)
	if err != nil {
		return errors.Wrapf(err, "cannot stat iso %s", isoFile)
	}
	fmt.Printf("%s: %vbyte\n", isoFile, stat.Size())

	stat2, err := os.Stat(badgerFolder)
	if err != nil {
		return errors.Wrapf(err, "cannot stat badger folder %s", badgerFolder)
	}
	if !stat2.IsDir() {
		return fmt.Errorf("%s is not a directory", badgerFolder)
	}

	bconfig := badger.DefaultOptions(badgerFolder)
	db, err := badger.Open(bconfig)
	if err != nil {
		return errors.Wrapf(err, "cannot open badger database")
	}
	defer db.Close()

	image, err := os.Open(isoFile)
	if err != nil {
		return errors.Wrapf(err, "cannot open iso file %s", isoFile)
	}
	defer image.Close()

	reader, err := iso9660.NewReader(image)
	for {
		fi, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Printf("error reading iso image: %v\n", err)
			break
		}
		f := fi.(*iso9660.File)
		if f.IsDir() {
			fmt.Printf("dir: %s\n", f.Name())
			continue
		}
		fmt.Printf("file: %s\n", f.Name())

		switch strings.ToLower(f.Name()) {
		case fileFile:
			if noFile {
				continue
			}
			freaderInt := f.Sys()
			freader, ok := freaderInt.(io.ReaderAt)
			if !ok {
				return fmt.Errorf("%v os not a reader", f)
			}
			zipR, err := zip.NewReader(freader, f.Size())
			if err != nil {
				return errors.Wrapf(err, "cannot open zip file %s", f.Name())
			}
			for _, fi := range zipR.File {
				fr, err := fi.Open()
				if err != nil {
					fmt.Printf("cannot open zip content %s/%s")
				}
				if err := copyCSV(strings.ToUpper(checksum), db, fr, []int{0, 1, 2}, checkonly); err != nil {
					return errors.Wrapf(err, "cannot copy file csv %s", fi.Name)
				}
				fr.Close()
			}

		case osFile:
			freader, ok := f.Sys().(io.Reader)
			if !ok {
				return fmt.Errorf("%v os not a reader", f)
			}
			if err := copyCSV("", db, freader, []int{0}, checkonly); err != nil {
				return errors.Wrapf(err, "cannot copy os csv %s", f.Name())
			}
		case mfgFile:
			/*
				if noMfg {
					continue
				}
			*/
			freader, ok := f.Sys().(io.Reader)
			if !ok {
				return fmt.Errorf("%v mfg not a reader", f)
			}
			if err := copyCSV("", db, freader, []int{0}, checkonly); err != nil {
				return errors.Wrapf(err, "cannot copy mfg csv %s", f.Name())
			}
		case prodFile:
			freader, ok := f.Sys().(io.Reader)
			if !ok {
				return fmt.Errorf("%v prod not a reader", f)
			}
			if err := copyCSV("", db, freader, []int{0}, checkonly); err != nil {
				return errors.Wrapf(err, "cannot copy prod csv %s", f.Name())
			}
		}
	}
	return nil
}

func zipTxt(zipFile, badgerFolder, fileFile, osFile, mfgFile, prodFile, checksum string, noFile, checkonly bool) error {

	switch strings.ToUpper(checksum) {
	case "MD5":
	case "SHA-1":
	case "":
	default:
		return fmt.Errorf("invalid checksum type: %s", checksum)
	}

	stat, err := os.Stat(zipFile)
	if err != nil {
		return errors.Wrapf(err, "cannot stat iso %s", zipFile)
	}
	fmt.Printf("%s: %vbyte\n", zipFile, stat.Size())

	stat2, err := os.Stat(badgerFolder)
	if err != nil {
		return errors.Wrapf(err, "cannot stat badger folder %s", badgerFolder)
	}
	if !stat2.IsDir() {
		return fmt.Errorf("%s is not a directory", badgerFolder)
	}

	bconfig := badger.DefaultOptions(badgerFolder)
	db, err := badger.Open(bconfig)
	if err != nil {
		return errors.Wrapf(err, "cannot open badger database")
	}
	defer db.Close()

	reader, err := zip.OpenReader(zipFile)
	if err != nil {
		return errors.Wrapf(err, "cannot open zip file %s", zipFile)
	}
	defer reader.Close()
	//	reader, err := iso9660.NewReader(image)
	for _, f := range reader.File {
		fname := strings.TrimPrefix(strings.ToLower(f.Name), "rds_modernm")
		switch fname {
		case fileFile:
			if noFile {
				continue
			}
			freader, err := f.Open()
			if err != nil {
				return errors.Wrapf(err, "cannot open %s", f.Name)
			}
			if err := copyCSV(strings.ToUpper(checksum), db, freader, []int{0, 1, 2}, checkonly); err != nil {
				freader.Close()
				return errors.Wrapf(err, "cannot copy file csv %s", f.Name)
			}
			freader.Close()
		case osFile:
			freader, err := f.Open()
			if err != nil {
				return errors.Wrapf(err, "cannot open %s", f.Name)
			}
			if err := copyCSV("", db, freader, []int{0}, checkonly); err != nil {
				return errors.Wrapf(err, "cannot copy os csv %s", f.Name)
			}
		case mfgFile:
			/*
				if noMfg {
					continue
				}
			*/
			freader, err := f.Open()
			if err != nil {
				return errors.Wrapf(err, "cannot open %s", f.Name)
			}
			if err := copyCSV("", db, freader, []int{0}, checkonly); err != nil {
				return errors.Wrapf(err, "cannot copy mfg csv %s", f.Name)
			}
		case prodFile:
			freader, err := f.Open()
			if err != nil {
				return errors.Wrapf(err, "cannot open %s", f.Name)
			}
			if err := copyCSV("", db, freader, []int{0}, checkonly); err != nil {
				return errors.Wrapf(err, "cannot copy prod csv %s", f.Name)
			}
		}
	}
	return nil
}

func main() {
	println(NSRL2BADGER)

	isoFile := flag.String("iso", "", "NSRL ISO file")
	zipFile := flag.String("zip", "", "NSRL ISO file")
	badgerFolder := flag.String("badger", "./", "badger folder")
	fileFile := flag.String("file", "", "nsrl file hashes")
	mfgFile := flag.String("mfg", "/nsrlmfg.txt", "nsrl mfg code and name")
	osFile := flag.String("os", "/nsrlos.txt", "nsrl os code and name")
	prodFile := flag.String("prod", "/nsrlprod.txt", "nsrl prod code and name")
	checkSum := flag.String("checksum", "MD5", "MD5 OR SHA-1 as key value")
	noFile := flag.Bool("nofile", false, "ignore nsrlfile.zip if true")
	checkOnly := flag.Bool("checkonly", false, "true, of only one entry per checksum is needed")

	flag.Parse()

	*fileFile = strings.ToLower(*fileFile)

	if *zipFile != "" {
		if *fileFile == "" {
			*fileFile = "/nsrlfile.txt"
		}
		if err := zipTxt(*zipFile, *badgerFolder, *fileFile, *osFile, *mfgFile, *prodFile, *checkSum, *noFile, *checkOnly); err != nil {
			fmt.Sprintf("Error: %v", err)
		}
	} else if *isoFile != "" {
		if *fileFile == "" {
			*fileFile = "/nsrlfile.zip"
		}
		if err := isoZip(*isoFile, *badgerFolder, *fileFile, *osFile, *mfgFile, *prodFile, strings.ToUpper(*checkSum), *noFile, *checkOnly); err != nil {
			fmt.Sprintf("Error: %v", err)
		}
	}
}
