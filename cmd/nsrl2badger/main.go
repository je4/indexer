// Copyright 2021 Juergen Enge, info-age GmbH, Basel. All rights reserved.
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
	"archive/zip"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	badger "github.com/dgraph-io/badger/v3"
	"github.com/golang/snappy"
	"github.com/goph/emperror"
	"github.com/hooklift/iso9660"
	"io"
	"log"
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

func copyCSV(indexField string, db *badger.DB, freader io.Reader, ignore []int) error {
	defer fmt.Println()
	var counter int64
	csvR := csv.NewReader(freader)
	fields, err := csvR.Read()
	if err != nil {
		return emperror.Wrapf(err, "cannot read csv fields from %v", freader)
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
	dataStruct := make(map[string]string)
	for {
		done := false
		num := 1000
		d := make(map[string][]map[string]string)
		for {
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
			}
			d[daKey] = appendIfNotExists(d[daKey], dataStruct)
			num--
			if num <= 0 {
				break
			}
		}
		if err := db.Update(func(txn *badger.Txn) error {
			for key, list := range d {
				item, err := txn.Get([]byte(key))
				if err != nil {
					if err != badger.ErrKeyNotFound {
						return emperror.Wrapf(err, "cannot get key %s", key)
					}
				} else {
					item.Value(func(val []byte) error {
						d, err := snappy.Decode(nil, val)
						if err != nil {
							return emperror.Wrapf(err, "cannot decode %s", val)
						}
						var ds []map[string]string
						if err := json.Unmarshal(d, &ds); err != nil {
							return emperror.Wrapf(err, "cannot unmarshal %s", string(val))
						}
						list = appendIfNotExists(list, ds...)
						return nil
					})
				}
				fmt.Printf("%v: %s [%v]    \r", counter, key, len(list))
				counter++
				d, err := json.Marshal(list)
				if err != nil {
					return emperror.Wrapf(err, "cannot marshal data %v", list)
				}
				d2 := snappy.Encode(nil, d)
				if err := txn.Set([]byte(key), d2); err != nil {
					return emperror.Wrapf(err, "cannot store struct %v", dataStruct)
				}
			}
			return nil
		}); err != nil {
			return emperror.Wrapf(err, "cannot store csv data")
		}
		if done {
			break
		}
	}
	return nil
}

func main() {
	println(NSRL2BADGER)

	isoFile := flag.String("iso", "RDS_modern.iso", "NSRL ISO file")
	badgerFolder := flag.String("badger", "./", "badger folder")
	fileFile := flag.String("file", "/nsrlfile.zip", "nsrl file hashes")
	mfgFile := flag.String("mfg", "/nsrlmfg.txt", "nsrl mfg code and name")
	osFile := flag.String("os", "/nsrlos.txt", "nsrl os code and name")
	prodFile := flag.String("prod", "/nsrlprod.txt", "nsrl prod code and name")
	checkSum := flag.String("checksum", "MD5", "MD5 OR SHA-1 as key value")
	noFile := flag.Bool("nofile", false, "ignore nsrlfile.zip if true")
	noMfg := flag.Bool("nomfg", false, "ignore nsrlmfg.txt if true")

	flag.Parse()

	stat, err := os.Stat(*isoFile)
	if err != nil {
		fmt.Printf("cannot stat iso %s: %v\n", *isoFile, err)
		return
	}
	fmt.Printf("%s: %vbyte\n", *isoFile, stat.Size())

	stat2, err := os.Stat(*badgerFolder)
	if err != nil {
		fmt.Printf("cannot stat badger folder %s: %v\n", *badgerFolder, err)
		return
	}
	if !stat2.IsDir() {
		fmt.Printf("%s is not a directory\n", *badgerFolder)
		return
	}

	bconfig := badger.DefaultOptions(*badgerFolder)
	db, err := badger.Open(bconfig)
	if err != nil {
		log.Panicf("cannot open badger database: %v\n", err)
		return
	}
	defer db.Close()

	image, err := os.Open(*isoFile)
	if err != nil {
		fmt.Printf("cannot open iso file %s: %v\n", *isoFile, err)
		return
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

		switch f.Name() {
		case *fileFile:
			if *noFile {
				continue
			}
			switch strings.ToUpper(*checkSum) {
			case "MD5":
			case "SHA-1":
			case "":
			default:
				fmt.Printf("invalid checksum field for file table: %s", *checkSum)
				return
			}
			freaderInt := f.Sys()
			freader, ok := freaderInt.(io.ReaderAt)
			if !ok {
				fmt.Printf("%v os not a reader", f)
				return
			}
			zipR, err := zip.NewReader(freader, f.Size())
			if err != nil {
				fmt.Printf("cannot open zip file %s", f.Name())
				return
			}
			for _, fi := range zipR.File {
				fr, err := fi.Open()
				if err != nil {
					fmt.Printf("cannot open zip content %s/%s")
				}
				if err := copyCSV(strings.ToUpper(*checkSum), db, fr, []int{0, 1, 2}); err != nil {
					fmt.Printf("cannot copy file csv %s: %v", fi.Name, err)
					return
				}
				fr.Close()
			}

		case *osFile:
			switch strings.ToUpper(*checkSum) {
			case "MD5":
			case "SHA-1":
			case "":
			default:
				fmt.Printf("invalid checksum type: %s", *checkSum)
				return
			}
			freader, ok := f.Sys().(io.Reader)
			if !ok {
				fmt.Printf("%v os not a reader", f)
				return
			}
			if err := copyCSV("", db, freader, []int{0}); err != nil {
				fmt.Printf("cannot copy os csv %s: %v", f.Name(), err)
				return
			}
		case *mfgFile:
			if *noMfg {
				continue
			}
			freader, ok := f.Sys().(io.Reader)
			if !ok {
				fmt.Printf("%v mfg not a reader", f)
				return
			}
			if err := copyCSV("", db, freader, []int{0}); err != nil {
				fmt.Printf("cannot copy mfg csv %s: %v", f.Name(), err)
				return
			}
		case *prodFile:
			freader, ok := f.Sys().(io.Reader)
			if !ok {
				fmt.Printf("%v prod not a reader", f)
				return
			}
			if err := copyCSV("", db, freader, []int{0}); err != nil {
				fmt.Printf("cannot copy prod csv %s: %v", f.Name(), err)
				return
			}
		}
	}
}
