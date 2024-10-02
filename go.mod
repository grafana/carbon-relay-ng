module github.com/grafana/carbon-relay-ng

go 1.21

require (
	cloud.google.com/go/pubsub v1.30.0
	github.com/BurntSushi/toml v0.3.1
	github.com/Dieterbe/artisanalhistogram v0.0.0-20170619072513-f61b7225d304
	github.com/Dieterbe/go-metrics v0.0.0-20181015090856-87383909479d
	github.com/Dieterbe/topic v0.0.0-20141209014555-1850ffda9965
	github.com/Shopify/sarama v1.23.0
	github.com/aws/aws-sdk-go v1.44.292
	github.com/bmizerany/assert v0.0.0-20120716205630-e17e99893cb6
	github.com/dgryski/go-linlog v0.0.0-20180207191225-edcf2dfd90ff
	github.com/elazarl/go-bindata-assetfs v1.0.1
	github.com/golang/snappy v0.0.4
	github.com/google/go-cmp v0.5.9
	github.com/gorilla/handlers v1.5.2
	github.com/gorilla/mux v1.8.1
	github.com/grafana/configparser v0.0.0-20210707122942-2593eb86a3ee
	github.com/grafana/metrictank v1.0.1-0.20221128152741-61182cf5f40e
	github.com/jpillora/backoff v1.0.0
	github.com/kisielk/og-rek v1.2.0
	github.com/metrics20/go-metrics20 v0.0.0-20180821133656-717ed3a27bf9
	github.com/prometheus/procfs v0.11.0
	github.com/sirupsen/logrus v1.9.3
	github.com/streadway/amqp v1.1.0
	github.com/stretchr/testify v1.9.0
	github.com/taylorchu/toki v0.0.0-20141019163204-20e86122596c
	github.com/xdg/scram v0.0.0-20180814205039-7eeb5667e42c
)

require (
	cloud.google.com/go v0.110.0 // indirect
	cloud.google.com/go/compute v1.19.1 // indirect
	cloud.google.com/go/compute/metadata v0.2.3 // indirect
	cloud.google.com/go/iam v0.13.0 // indirect
	github.com/DataDog/zstd v1.3.6-0.20190409195224-796139022798 // indirect
	github.com/cespare/xxhash v0.0.0-00010101000000-000000000000 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/dgryski/go-jump v0.0.0-20170409065014-e1f439676b57 // indirect
	github.com/eapache/go-resiliency v1.1.0 // indirect
	github.com/eapache/go-xerial-snappy v0.0.0-20180814174437-776d5712da21 // indirect
	github.com/eapache/queue v1.1.0 // indirect
	github.com/felixge/httpsnoop v1.0.3 // indirect
	github.com/golang/groupcache v0.0.0-20200121045136-8c9f03a8e57e // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.2.3 // indirect
	github.com/googleapis/gax-go/v2 v2.7.1 // indirect
	github.com/hashicorp/go-uuid v1.0.1 // indirect
	github.com/jcmturner/gofork v0.0.0-20190328161633-dc7c13fece03 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/kr/pretty v0.0.0-20160325215624-add1dbc86daf // indirect
	github.com/kr/text v0.0.0-20150905224508-bb797dc4fb83 // indirect
	github.com/pelletier/go-toml v1.9.1 // indirect
	github.com/philhofer/fwd v0.0.0-20151120024002-92647f2bd94a // indirect
	github.com/pierrec/lz4 v0.0.0-20190327172049-315a67e90e41 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/rcrowley/go-metrics v0.0.0-20181016184325-3113b8401b8a // indirect
	github.com/smartystreets/goconvey v1.6.4 // indirect
	github.com/tinylib/msgp v1.1.0 // indirect
	github.com/xdg/stringprep v1.0.0 // indirect
	go.opencensus.io v0.24.0 // indirect
	golang.org/x/crypto v0.21.0 // indirect
	golang.org/x/net v0.23.0 // indirect
	golang.org/x/oauth2 v0.7.0 // indirect
	golang.org/x/sync v0.2.0 // indirect
	golang.org/x/sys v0.18.0 // indirect
	golang.org/x/text v0.14.0 // indirect
	google.golang.org/api v0.114.0 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/genproto v0.0.0-20230410155749-daa745c078e1 // indirect
	google.golang.org/grpc v1.56.3 // indirect
	google.golang.org/protobuf v1.30.0 // indirect
	gopkg.in/jcmturner/aescts.v1 v1.0.1 // indirect
	gopkg.in/jcmturner/dnsutils.v1 v1.0.1 // indirect
	gopkg.in/jcmturner/goidentity.v3 v3.0.0 // indirect
	gopkg.in/jcmturner/gokrb5.v7 v7.2.3 // indirect
	gopkg.in/jcmturner/rpc.v1 v1.1.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/cespare/xxhash => github.com/cespare/xxhash/v2 v2.1.1

replace github.com/BurntSushi/toml v0.0.0-00010101000000-000000000000 => github.com/Dieterbe/toml v0.2.1-0.20181015092100-96f3d827bb6c
