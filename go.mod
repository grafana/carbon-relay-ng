module github.com/grafana/carbon-relay-ng

go 1.25.12

require (
	cloud.google.com/go/pubsub v1.51.0
	github.com/BurntSushi/toml v0.3.1
	github.com/Dieterbe/artisanalhistogram v0.0.0-20210330070510-f7596a8a7dbe
	github.com/Dieterbe/go-metrics v0.0.0-20181015090856-87383909479d
	github.com/Dieterbe/topic v0.0.0-20141209014555-1850ffda9965
	github.com/IBM/sarama v1.50.1
	github.com/aws/aws-sdk-go-v2 v1.41.12
	github.com/aws/aws-sdk-go-v2/config v1.32.22
	github.com/aws/aws-sdk-go-v2/service/cloudwatch v1.58.2
	github.com/dgryski/go-linlog v0.0.0-20211015210535-87314c522abf
	github.com/elazarl/go-bindata-assetfs v1.0.1
	github.com/golang/snappy v1.0.0
	github.com/google/go-cmp v0.7.0
	github.com/gorilla/handlers v1.5.2
	github.com/gorilla/mux v1.8.1
	github.com/grafana/configparser v0.0.0-20210707122942-2593eb86a3ee
	github.com/grafana/metrictank v1.0.1-0.20221128152741-61182cf5f40e
	github.com/jpillora/backoff v1.0.0
	github.com/kisielk/og-rek v1.3.0
	github.com/metrics20/go-metrics20 v0.0.0-20240216190837-a2a3be37ef3e
	github.com/prometheus/procfs v0.20.1
	github.com/raintank/dur v0.0.0-20220106223125-d7c6a541ef3a
	github.com/sirupsen/logrus v1.9.4
	github.com/streadway/amqp v1.1.0
	github.com/stretchr/testify v1.11.1
	github.com/taylorchu/toki v0.0.0-20240927201529-36c157adcc65
	github.com/xdg/scram v1.0.5
)

require (
	cloud.google.com/go v0.123.0 // indirect
	cloud.google.com/go/auth v0.20.0 // indirect
	cloud.google.com/go/auth/oauth2adapt v0.2.8 // indirect
	cloud.google.com/go/compute/metadata v0.9.0 // indirect
	cloud.google.com/go/iam v1.11.0 // indirect
	cloud.google.com/go/pubsub/v2 v2.6.0 // indirect
	github.com/aristanetworks/gomap v0.0.0-20230726210543-f4e41046dced // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.19.21 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.18.27 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.4.28 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.7.28 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.4.28 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.13.11 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.13.27 // indirect
	github.com/aws/aws-sdk-go-v2/service/signin v1.1.3 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.31.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.36.4 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.43.1 // indirect
	github.com/aws/smithy-go v1.27.1 // indirect
	github.com/bmizerany/assert v0.0.0-20160611221934-b7ed37b82869 // indirect
	github.com/cespare/xxhash v0.0.0-00010101000000-000000000000 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/dgryski/go-jump v0.0.0-20170409065014-e1f439676b57 // indirect
	github.com/eapache/go-resiliency v1.7.0 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/google/s2a-go v0.1.9 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.3.17 // indirect
	github.com/googleapis/gax-go/v2 v2.23.0 // indirect
	github.com/hashicorp/go-uuid v1.0.3 // indirect
	github.com/jcmturner/aescts/v2 v2.0.0 // indirect
	github.com/jcmturner/dnsutils/v2 v2.0.0 // indirect
	github.com/jcmturner/gofork v1.7.6 // indirect
	github.com/jcmturner/gokrb5/v8 v8.4.4 // indirect
	github.com/jcmturner/rpc/v2 v2.0.3 // indirect
	github.com/klauspost/compress v1.18.6 // indirect
	github.com/pelletier/go-toml v1.9.1 // indirect
	github.com/philhofer/fwd v0.0.0-20151120024002-92647f2bd94a // indirect
	github.com/pierrec/lz4/v4 v4.1.26 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/rcrowley/go-metrics v0.0.0-20250401214520-65e299d6c5c9 // indirect
	github.com/smartystreets/goconvey v1.6.4 // indirect
	github.com/tinylib/msgp v1.1.0 // indirect
	github.com/xdg/stringprep v1.0.0 // indirect
	go.opencensus.io v0.24.0 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.67.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.67.0 // indirect
	go.opentelemetry.io/otel v1.44.0 // indirect
	go.opentelemetry.io/otel/metric v1.44.0 // indirect
	go.opentelemetry.io/otel/trace v1.44.0 // indirect
	golang.org/x/crypto v0.53.0 // indirect
	golang.org/x/exp v0.0.0-20230725093048-515e97ebf090 // indirect
	golang.org/x/net v0.56.0 // indirect
	golang.org/x/oauth2 v0.36.0 // indirect
	golang.org/x/sync v0.21.0 // indirect
	golang.org/x/sys v0.46.0 // indirect
	golang.org/x/text v0.38.0 // indirect
	golang.org/x/time v0.15.0 // indirect
	google.golang.org/api v0.287.1 // indirect
	google.golang.org/genproto v0.0.0-20260319201613-d00831a3d3e7 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20260630182238-925bb5da69e7 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260630182238-925bb5da69e7 // indirect
	google.golang.org/grpc v1.82.0 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/cespare/xxhash => github.com/cespare/xxhash/v2 v2.1.1

replace github.com/BurntSushi/toml v0.3.1 => github.com/Dieterbe/toml v0.2.1-0.20181015092100-96f3d827bb6c
