module bessarabov/mac2mqtt

go 1.23.0

toolchain go1.24.3

require (
	github.com/antonfisher/go-media-devices-state v0.2.0
	github.com/cloudfoundry/gosigar v1.3.112
	github.com/eclipse/paho.mqtt.golang v1.3.5
	gopkg.in/yaml.v2 v2.4.0
)

require (
	github.com/gorilla/websocket v1.4.2 // indirect
	github.com/kr/pretty v0.3.1 // indirect
	golang.org/x/net v0.43.0 // indirect
	golang.org/x/sys v0.35.0 // indirect
)

// using my fork until PR #9 is resolved in upstream
replace github.com/antonfisher/go-media-devices-state => github.com/johntdyer/go-media-devices-state v0.0.0-20251204145225-5b3592a6499f
