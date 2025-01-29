module github.com/je4/indexer/v3

go 1.23.0

toolchain go1.23.1

replace github.com/google/flatbuffers => github.com/google/flatbuffers v1.12.1

require (
	emperror.dev/errors v0.8.1
	github.com/BurntSushi/toml v1.4.0
	github.com/dgraph-io/badger/v4 v4.5.1
	github.com/gabriel-vasile/mimetype v1.4.8
	github.com/golang/snappy v0.0.4
	github.com/gorilla/handlers v1.5.2
	github.com/gorilla/mux v1.8.1
	github.com/hooklift/iso9660 v1.0.0
	github.com/je4/filesystem/v3 v3.0.23
	github.com/je4/goffmpeg v0.0.0-20220114092308-33ab9986404d
	github.com/je4/utils/v2 v2.0.57
	github.com/op/go-logging v0.0.0-20160315200505-970db520ece7
	github.com/phayes/freeport v0.0.0-20220201140144-74d24b5ae9f5
	github.com/pkg/sftp v1.13.7
	github.com/richardlehane/siegfried v1.11.1
	github.com/rs/zerolog v1.33.0
	github.com/tamerh/xml-stream-parser v1.5.0
	gitlab.switch.ch/ub-unibas/go-ublogger/v2 v2.0.1
	go.ub.unibas.ch/cloud/certloader/v2 v2.0.18
	golang.org/x/crypto v0.32.0
	golang.org/x/exp v0.0.0-20250128182459-e0ece0dbea4c
)

require (
	filippo.io/edwards25519 v1.1.0 // indirect
	github.com/KyleBanks/depth v1.2.1 // indirect
	github.com/antchfx/xpath v1.3.0 // indirect
	github.com/bluele/gcache v0.0.2 // indirect
	github.com/bytedance/sonic v1.12.8 // indirect
	github.com/bytedance/sonic/loader v0.2.3 // indirect
	github.com/c4milo/gotoolkit v0.0.0-20190525173301-67483a18c17a // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/cloudwego/base64x v0.1.5 // indirect
	github.com/cloudwego/iasm v0.2.0 // indirect
	github.com/dgraph-io/ristretto/v2 v2.1.0 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/fsnotify/fsnotify v1.8.0 // indirect
	github.com/gin-contrib/cors v1.7.3 // indirect
	github.com/gin-contrib/sse v1.0.0 // indirect
	github.com/gin-gonic/gin v1.10.0 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-openapi/jsonpointer v0.21.0 // indirect
	github.com/go-openapi/jsonreference v0.21.0 // indirect
	github.com/go-openapi/spec v0.21.0 // indirect
	github.com/go-openapi/swag v0.23.0 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/go-playground/validator/v10 v10.24.0 // indirect
	github.com/goccy/go-json v0.10.5 // indirect
	github.com/golang/groupcache v0.0.0-20241129210726-2c02b8208cf8 // indirect
	github.com/google/certificate-transparency-go v1.3.1 // indirect
	github.com/google/flatbuffers v25.1.24+incompatible // indirect
	github.com/hooklift/assert v0.1.0 // indirect
	github.com/je4/trustutil/v2 v2.0.28 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/compress v1.17.11 // indirect
	github.com/klauspost/cpuid/v2 v2.2.9 // indirect
	github.com/kr/fs v0.1.0 // indirect
	github.com/leodido/go-urn v1.4.0 // indirect
	github.com/mailru/easyjson v0.9.0 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/pelletier/go-toml/v2 v2.2.3 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/richardlehane/characterize v1.0.0 // indirect
	github.com/richardlehane/match v1.0.5 // indirect
	github.com/richardlehane/mscfb v1.0.4 // indirect
	github.com/richardlehane/msoleps v1.0.4 // indirect
	github.com/richardlehane/webarchive v1.0.3 // indirect
	github.com/richardlehane/xmldetect v1.0.2 // indirect
	github.com/ross-spencer/spargo v0.4.1 // indirect
	github.com/ross-spencer/wikiprov v1.0.0 // indirect
	github.com/smallstep/certinfo v1.13.0 // indirect
	github.com/swaggo/files v1.0.1 // indirect
	github.com/swaggo/gin-swagger v1.6.0 // indirect
	github.com/swaggo/swag v1.16.4 // indirect
	github.com/tamerh/xpath v1.0.0 // indirect
	github.com/telkomdev/go-stash v1.0.6 // indirect
	github.com/twitchyliquid64/golang-asm v0.15.1 // indirect
	github.com/ugorji/go/codec v1.2.12 // indirect
	go.opencensus.io v0.24.0 // indirect
	go.step.sm/crypto v0.57.0 // indirect
	go.ub.unibas.ch/cloud/minivault/v2 v2.0.15 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/arch v0.13.0 // indirect
	golang.org/x/image v0.23.0 // indirect
	golang.org/x/net v0.34.0 // indirect
	golang.org/x/sys v0.29.0 // indirect
	golang.org/x/text v0.21.0 // indirect
	golang.org/x/tools v0.29.0 // indirect
	google.golang.org/protobuf v1.36.4 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/klog/v2 v2.130.1 // indirect
)
