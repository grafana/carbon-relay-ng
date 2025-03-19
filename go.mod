module github.com/grafana/carbon-relay-ng

go 1.23.4

require (
	cloud.google.com/go/pubsub v1.48.0
	github.com/BurntSushi/toml v0.3.1
	github.com/Dieterbe/artisanalhistogram v0.0.0-20170619072513-f61b7225d304
	github.com/Dieterbe/go-metrics v0.0.0-20181015090856-87383909479d
	github.com/Dieterbe/topic v0.0.0-20141209014555-1850ffda9965
	github.com/Shopify/sarama v1.23.0
	github.com/aws/aws-sdk-go v1.55.6
	github.com/bmizerany/assert v0.0.0-20120716205630-e17e99893cb6
	github.com/dgryski/go-linlog v0.0.0-20180207191225-edcf2dfd90ff
	github.com/elazarl/go-bindata-assetfs v1.0.1
	github.com/golang/snappy v0.0.4
	github.com/google/go-cmp v0.7.0
	github.com/gorilla/handlers v1.5.2
	github.com/gorilla/mux v1.8.1
	github.com/grafana/configparser v0.0.0-20210707122942-2593eb86a3ee
	github.com/grafana/metrictank v1.0.1-0.20221128152741-61182cf5f40e
	github.com/jpillora/backoff v1.0.0
	github.com/kisielk/og-rek v1.3.0
	github.com/metrics20/go-metrics20 v0.0.0-20180821133656-717ed3a27bf9
	github.com/prometheus/procfs v0.16.0
	github.com/raintank/dur v0.0.0-20220106223125-d7c6a541ef3a
	github.com/sirupsen/logrus v1.9.3
	github.com/streadway/amqp v1.1.0
	github.com/stretchr/testify v1.10.0
	github.com/taylorchu/toki v0.0.0-20141019163204-20e86122596c
	github.com/xdg/scram v1.0.5
)

require (
	cloud.google.com/go v0.119.0 // indirect
	cloud.google.com/go/auth v0.15.0 // indirect
	cloud.google.com/go/auth/oauth2adapt v0.2.7 // indirect
	cloud.google.com/go/compute/metadata v0.6.0 // indirect
	cloud.google.com/go/iam v1.4.1 // indirect
	github.com/DataDog/zstd v1.3.6-0.20190409195224-796139022798 // indirect
	github.com/aristanetworks/gomap v0.0.0-20230726210543-f4e41046dced // indirect
	github.com/cespare/xxhash v0.0.0-00010101000000-000000000000 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/dgryski/go-jump v0.0.0-20170409065014-e1f439676b57 // indirect
	github.com/eapache/go-resiliency v1.1.0 // indirect
	github.com/eapache/go-xerial-snappy v0.0.0-20180814174437-776d5712da21 // indirect
	github.com/eapache/queue v1.1.0 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/google/s2a-go v0.1.9 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.3.5 // indirect
	github.com/googleapis/gax-go/v2 v2.14.1 // indirect
	github.com/hashicorp/go-uuid v1.0.1 // indirect
	github.com/jcmturner/gofork v0.0.0-20190328161633-dc7c13fece03 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/kr/pretty v0.3.1 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/pelletier/go-toml v1.9.1 // indirect
	github.com/philhofer/fwd v0.0.0-20151120024002-92647f2bd94a // indirect
	github.com/pierrec/lz4 v0.0.0-20190327172049-315a67e90e41 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/rcrowley/go-metrics v0.0.0-20181016184325-3113b8401b8a // indirect
	github.com/rogpeppe/go-internal v1.13.1 // indirect
	github.com/smartystreets/goconvey v1.6.4 // indirect
	github.com/tinylib/msgp v1.1.0 // indirect
	github.com/xdg/stringprep v1.0.0 // indirect
	go.opencensus.io v0.24.0 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.59.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.59.0 // indirect
	go.opentelemetry.io/otel v1.34.0 // indirect
	go.opentelemetry.io/otel/metric v1.34.0 // indirect
	go.opentelemetry.io/otel/trace v1.34.0 // indirect
	golang.org/x/crypto v0.35.0 // indirect
	golang.org/x/exp v0.0.0-20230725093048-515e97ebf090 // indirect
	golang.org/x/net v0.35.0 // indirect
	golang.org/x/oauth2 v0.28.0 // indirect
	golang.org/x/sync v0.12.0 // indirect
	golang.org/x/sys v0.30.0 // indirect
	golang.org/x/text v0.22.0 // indirect
	golang.org/x/time v0.11.0 // indirect
	google.golang.org/api v0.224.0 // indirect
	google.golang.org/genproto v0.0.0-20250303144028-a0af3efb3deb // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20250303144028-a0af3efb3deb // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250227231956-55c901821b1e // indirect
	google.golang.org/grpc v1.71.0 // indirect
	google.golang.org/protobuf v1.36.5 // indirect
	gopkg.in/jcmturner/aescts.v1 v1.0.1 // indirect
	gopkg.in/jcmturner/dnsutils.v1 v1.0.1 // indirect
	gopkg.in/jcmturner/goidentity.v3 v3.0.0 // indirect
	gopkg.in/jcmturner/gokrb5.v7 v7.2.3 // indirect
	gopkg.in/jcmturner/rpc.v1 v1.1.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/cespare/xxhash => github.com/cespare/xxhash/v2 v2.1.1

replace github.com/BurntSushi/toml v0.3.1 => github.com/Dieterbe/toml v0.2.1-0.20181015092100-96f3d827bb6c
