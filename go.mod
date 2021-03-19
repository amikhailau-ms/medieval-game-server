module github.com/amikhailau/medieval-game-server

go 1.15

require (
	agones.dev/agones v1.13.0
	github.com/Tarliton/collision2d v0.1.0
	github.com/golang/protobuf v1.4.3
	github.com/spf13/afero v1.5.1 // indirect
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.7.1
	golang.org/x/net v0.0.0-20200822124328-c89045814202 // indirect
	golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1 // indirect
	google.golang.org/grpc v1.33.1
	google.golang.org/protobuf v1.25.0
	gopkg.in/yaml.v2 v2.4.0 // indirect
	k8s.io/apimachinery v0.18.15
	k8s.io/client-go v0.18.15
)

replace github.com/Tarliton/collision2d v0.1.0 => github.com/amikhailau/collision2d v0.1.1
