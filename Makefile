.PHONY: build

build:
	rm -rf build
	gomobile bind -v -target=ios,tvos,macos -tags with_gvisor .

install: build
	rm -rf ../WayToFly/Libbox.xcframework
	cp -r Libbox.xcframework ../WayToFly/Libbox.xcframework

build_install:
	go install -v github.com/sagernet/gomobile/cmd/gomobile@v0.1.4
	go install -v github.com/sagernet/gomobile/cmd/gobind@v0.1.4

fmt:
	@gofumpt -l -w .
	@gofmt -s -w .
	@gci write --custom-order -s standard -s "prefix(github.com/nekohasekai)" -s "default" .

fmt_install:
	go install -v mvdan.cc/gofumpt@latest
	go install -v github.com/daixiang0/gci@latest

download_geodb:
	wget -O ../WayToFly/Library/Assets.xcassets/geoip.dataset/geoip.dat https://github.com/v2fly/geoip/releases/latest/download/cn.dat
	wget -O ../WayToFly/Library/Assets.xcassets/geosite.dataset/geosite.dat https://github.com/v2fly/domain-list-community/releases/latest/download/dlc.dat
