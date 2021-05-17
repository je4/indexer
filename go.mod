module github.com/je4/indexer

go 1.16

replace github.com/je4/indexer => ./

require (
	github.com/BurntSushi/toml v0.3.1
	github.com/c4milo/gotoolkit v0.0.0-20190525173301-67483a18c17a // indirect
	github.com/dgraph-io/badger v1.6.2
	github.com/dgraph-io/badger/v3 v3.2011.1
	github.com/gabriel-vasile/mimetype v1.3.0
	github.com/golang/snappy v0.0.3
	github.com/goph/emperror v0.17.2
	github.com/gorilla/handlers v1.5.1
	github.com/gorilla/mux v1.8.0
	github.com/hooklift/assert v0.1.0 // indirect
	github.com/hooklift/iso9660 v1.0.0
	github.com/je4/goffmpeg v0.0.0-20190920144005-0e5560003726
	github.com/op/go-logging v0.0.0-20160315200505-970db520ece7
	github.com/phayes/freeport v0.0.0-20180830031419-95f893ade6f2
	github.com/pkg/errors v0.9.1
	github.com/pkg/sftp v1.13.0
	github.com/richardlehane/siegfried v1.9.1 // indirect
	golang.org/x/crypto v0.0.0-20210506145944-38f3c27a63bf
)
