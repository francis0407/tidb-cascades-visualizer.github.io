module SVGTest

go 1.12

require (
	github.com/ajstarks/svgo v0.0.0-20191124160048-bd5c74aaa11c
	github.com/pingcap/parser v0.0.0-20200422082501-7329d80eaf2c
	github.com/pingcap/tidb v2.0.11+incompatible
	golang.org/dl v0.0.0-20200205193131-62c734104014 // indirect
)

replace github.com/pingcap/check => github.com/tiancaiamao/check v0.0.0-20191119042138-8e73d07b629d

replace github.com/coreos/go-systemd => github.com/5kbpers/go-systemd v0.0.0-20191226123609-22b03c51af0f

replace github.com/cznic/mathutil => github.com/5kbpers/mathutil v0.0.0-20200212065626-c4124e10c01c
