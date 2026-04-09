module wx_channel

go 1.20

exclude (
	github.com/andybalholm/brotli v1.2.0
	golang.org/x/crypto v0.22.0
	golang.org/x/crypto v0.23.0
	golang.org/x/crypto v0.24.0
	golang.org/x/crypto v0.25.0
	golang.org/x/crypto v0.26.0
	golang.org/x/crypto v0.27.0
	golang.org/x/crypto v0.28.0
	golang.org/x/crypto v0.29.0
	golang.org/x/crypto v0.30.0
	golang.org/x/crypto v0.31.0
	golang.org/x/crypto v0.32.0
	golang.org/x/crypto v0.33.0
	golang.org/x/crypto v0.34.0
	golang.org/x/crypto v0.35.0
	golang.org/x/crypto v0.36.0
	golang.org/x/crypto v0.44.0
)

require (
	github.com/GopeedLab/gopeed v0.0.0-00010101000000-000000000000
	github.com/adrg/xdg v0.4.0
	github.com/andybalholm/brotli v1.1.1
	github.com/blang/semver v3.5.1+incompatible
	github.com/ebitengine/oto/v3 v3.2.0-alpha.4
	github.com/fatih/color v1.16.0
	github.com/gin-gonic/gin v1.9.1
	github.com/golang-migrate/migrate/v4 v4.19.1
	github.com/gorilla/websocket v1.5.3
	github.com/inconshreveable/go-update v0.0.0-20160112193335-8152e7eb6ccf
	github.com/ltaoo/echo v0.8.0
	github.com/pterm/pterm v0.12.50
	github.com/qtgolang/SunnyNet v1.4.0
	github.com/rhysd/go-github-selfupdate v1.2.3
	github.com/rs/zerolog v1.31.0
	github.com/samber/lo v1.39.0
	github.com/spf13/cobra v1.8.0
	github.com/spf13/viper v1.16.0
	github.com/zeebo/blake3 v0.2.4
	golang.org/x/net v0.47.0
	golang.org/x/text v0.31.0
	gorm.io/driver/mysql v1.5.7
	gorm.io/driver/postgres v1.5.11
	gorm.io/driver/sqlite v1.5.7
	gorm.io/gorm v1.25.12
)

require (
	github.com/JohannesKaufmann/html-to-markdown v1.4.1 // indirect
	github.com/PuerkitoBio/goquery v1.8.1 // indirect
	github.com/go-sourcemap/sourcemap v2.1.3+incompatible // indirect
	golang.org/x/exp v0.0.0-20251023183803-a4bb9ffd2546 // indirect
	golang.org/x/image v0.23.0 // indirect
)

replace github.com/GopeedLab/gopeed => ./pkg/gopeed

// replace github.com/qtgolang/SunnyNet => ./pkg/SunnyNet

// Downgrade golang.org/x/* to versions compatible with Go 1.20
replace (
	golang.org/x/crypto => golang.org/x/crypto v0.17.0
	golang.org/x/exp => golang.org/x/exp v0.0.0-20230905200255-921286631fa9
	golang.org/x/image => golang.org/x/image v0.14.0
	golang.org/x/net => golang.org/x/net v0.17.0
	golang.org/x/sync => golang.org/x/sync v0.5.0
	golang.org/x/sys => golang.org/x/sys v0.15.0
	golang.org/x/term => golang.org/x/term v0.15.0
	golang.org/x/text => golang.org/x/text v0.14.0
)

require (
	atomicgo.dev/cursor v0.2.0 // indirect
	atomicgo.dev/keyboard v0.2.9 // indirect
	filippo.io/edwards25519 v1.1.0 // indirect
	github.com/Trisia/gosysproxy v1.1.0 // indirect
	github.com/andybalholm/cascadia v1.3.2 // indirect
	github.com/bytedance/sonic v1.14.0 // indirect
	github.com/bytedance/sonic/loader v0.3.0 // indirect
	github.com/cloudwego/base64x v0.1.5 // indirect
	github.com/containerd/console v1.0.5 // indirect
	github.com/dlclark/regexp2 v1.11.4 // indirect
	github.com/dop251/goja v0.0.0-20260106131823-651366fbe6e3 // indirect
	github.com/ebitengine/purego v0.9.0 // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/gabriel-vasile/mimetype v1.4.3 // indirect
	github.com/gin-contrib/sse v0.1.0 // indirect
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/go-playground/validator/v10 v10.17.0 // indirect
	github.com/go-sql-driver/mysql v1.8.1 // indirect
	github.com/goccy/go-json v0.10.5 // indirect
	github.com/google/go-github/v30 v30.1.0 // indirect
	github.com/google/go-querystring v1.1.0 // indirect
	github.com/google/gopacket v1.1.19 // indirect
	github.com/google/pprof v0.0.0-20250317173921-a4b03ec1a45e // indirect
	github.com/gookit/color v1.5.4 // indirect
	github.com/hashicorp/hcl v1.0.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/pgx/v5 v5.6.0 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jinzhu/now v1.1.5 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/compress v1.17.11 // indirect
	github.com/klauspost/cpuid/v2 v2.2.8 // indirect
	github.com/leodido/go-urn v1.4.0 // indirect
	github.com/lib/pq v1.10.9 // indirect
	github.com/lithammer/fuzzysearch v1.1.8 // indirect
	github.com/magiconair/properties v1.8.7 // indirect
	github.com/matoous/go-nanoid/v2 v2.0.0 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-ieproxy v0.0.11 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mattn/go-runewidth v0.0.16 // indirect
	github.com/mattn/go-sqlite3 v1.14.22 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/onsi/gomega v1.27.6 // indirect
	github.com/pelletier/go-toml/v2 v2.1.0 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/rogpeppe/go-internal v1.11.0 // indirect
	github.com/shirou/gopsutil v3.21.11+incompatible // indirect
	github.com/songgao/water v0.0.0-20200317203138-2b4b6d7c09d8 // indirect
	github.com/spf13/afero v1.10.0 // indirect
	github.com/spf13/cast v1.6.0 // indirect
	github.com/spf13/jwalterweatherman v1.1.0 // indirect
	github.com/spf13/pflag v1.0.10 // indirect
	github.com/subosito/gotenv v1.6.0 // indirect
	github.com/tcnksm/go-gitconfig v0.1.2 // indirect
	github.com/tidwall/gjson v1.18.0 // indirect
	github.com/tidwall/match v1.1.1 // indirect
	github.com/tidwall/pretty v1.2.0 // indirect
	github.com/tidwall/sjson v1.2.5 // indirect
	github.com/tklauser/go-sysconf v0.3.14 // indirect
	github.com/tklauser/numcpus v0.8.0 // indirect
	github.com/twitchyliquid64/golang-asm v0.15.1 // indirect
	github.com/ugorji/go/codec v1.2.11 // indirect
	github.com/ulikunitz/xz v0.5.9 // indirect
	github.com/xiaoqidun/setft v0.0.0-20220310121541-be86327699ad // indirect
	github.com/xo/terminfo v0.0.0-20220910002029-abceb7e1c41e // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	go.etcd.io/bbolt v1.3.8 // indirect
	golang.org/x/arch v0.3.0 // indirect
	golang.org/x/crypto v0.45.0 // indirect
	golang.org/x/oauth2 v0.30.0 // indirect
	golang.org/x/sync v0.19.0 // indirect
	golang.org/x/sys v0.38.0 // indirect
	golang.org/x/term v0.37.0 // indirect
	google.golang.org/protobuf v1.36.7 // indirect
	gopkg.in/ini.v1 v1.67.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
