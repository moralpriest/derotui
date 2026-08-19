module github.com/deroproject/dero-wallet-cli

go 1.26

toolchain go1.26.0

require (
	charm.land/bubbles/v2 v2.1.1
	charm.land/bubbletea/v2 v2.0.8
	charm.land/lipgloss/v2 v2.0.6
	github.com/atotto/clipboard v0.1.4
	github.com/civilware/derodpkg v0.0.0-00010101000000-000000000000
	github.com/civilware/epoch v0.0.0-20241002060739-1ed2fc6f74cb
	github.com/creachadair/jrpc2 v0.36.0
	github.com/deroproject/derohe v0.0.0-20240405032004-bd300c0e086e
	github.com/gorilla/websocket v1.5.3
	github.com/skip2/go-qrcode v0.0.0-20200617195104-da1b6568686e
	go-miner v0.0.0-00010101000000-000000000000
	golang.org/x/sys v0.47.0
)

require (
	github.com/VictoriaMetrics/metrics v1.40.2 // indirect
	github.com/beevik/ntp v1.5.0 // indirect
	github.com/blang/semver/v4 v4.0.0 // indirect
	github.com/caarlos0/env/v6 v6.10.1 // indirect
	github.com/cenkalti/hub v1.0.2 // indirect
	github.com/cenkalti/rpc2 v1.0.5 // indirect
	github.com/cespare/xxhash v1.1.0 // indirect
	github.com/charmbracelet/colorprofile v0.4.3 // indirect
	github.com/charmbracelet/ultraviolet v0.0.0-20260811164956-006e29f97886 // indirect
	github.com/charmbracelet/x/ansi v0.11.8 // indirect
	github.com/charmbracelet/x/term v0.2.2 // indirect
	github.com/charmbracelet/x/termios v0.1.1 // indirect
	github.com/charmbracelet/x/windows v0.2.2 // indirect
	github.com/chzyer/readline v1.5.1 // indirect
	github.com/civilware/tela v0.0.0-20260530200926-176ee608babd // indirect
	github.com/clipperhouse/displaywidth v0.11.0 // indirect
	github.com/clipperhouse/uax29/v2 v2.7.0 // indirect
	github.com/coder/websocket v1.8.14 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/dchest/siphash v1.2.3 // indirect
	github.com/deroproject/graviton v0.0.0-20220130070622-2c248a53b2e1 // indirect
	github.com/docopt/docopt-go v0.0.0-20180111231733-ee0de3bc6815 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/fxamacker/cbor/v2 v2.9.0 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/zapr v1.3.0 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/hashicorp/golang-lru v1.0.2 // indirect
	github.com/klauspost/cpuid/v2 v2.3.0 // indirect
	github.com/klauspost/reedsolomon v1.13.0 // indirect
	github.com/lesismal/llib v1.2.2 // indirect
	github.com/lesismal/nbio v1.6.8 // indirect
	github.com/lucasb-eyer/go-colorful v1.4.1 // indirect
	github.com/mattn/go-runewidth v0.0.24 // indirect
	github.com/minio/sha256-simd v1.0.1 // indirect
	github.com/muesli/cancelreader v0.2.2 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/robfig/cron/v3 v3.0.1 // indirect
	github.com/satori/go.uuid v1.2.0 // indirect
	github.com/segmentio/fasthash v1.0.3 // indirect
	github.com/stretchr/testify v1.11.1 // indirect
	github.com/tjfoc/gmsm v1.4.1 // indirect
	github.com/valyala/fastrand v1.1.0 // indirect
	github.com/valyala/histogram v1.2.0 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	github.com/xo/terminfo v0.0.0-20220910002029-abceb7e1c41e // indirect
	github.com/xtaci/kcp-go/v5 v5.6.61 // indirect
	github.com/ybbus/jsonrpc v2.1.2+incompatible // indirect
	go.uber.org/multierr v1.10.0 // indirect
	go.uber.org/zap v1.27.1 // indirect
	golang.org/x/crypto v0.50.0 // indirect
	golang.org/x/net v0.53.0 // indirect
	golang.org/x/sync v0.22.0 // indirect
	golang.org/x/time v0.14.0 // indirect
	golang.org/x/xerrors v0.0.0-20240903120638-7835f813f4da // indirect
	gopkg.in/natefinch/lumberjack.v2 v2.0.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/deroproject/derohe => github.com/DEROFDN/derohe v0.0.0-20260817052654-390295b40c0b

replace github.com/civilware/epoch => github.com/moralpriest/epoch v0.0.0-20260812013238-2104b271e52a

replace github.com/civilware/derodpkg => github.com/moralpriest/derodpkg v0.0.0-20260421204848-b0b2a0af70d7

replace github.com/creachadair/jrpc2 => github.com/creachadair/jrpc2 v0.35.4

replace github.com/lesismal/nbio => github.com/lesismal/nbio v1.2.20

replace github.com/lesismal/llib => github.com/lesismal/llib v1.1.6

replace go-miner => ../Dirtybird-Go-Miner
