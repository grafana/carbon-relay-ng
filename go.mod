module github.com/grafana/carbon-relay-ng

go 1.13

require (
	cloud.google.com/go v0.18.1-0.20180119164648-b1067c1d21b5
	github.com/BurntSushi/toml v0.0.0-00010101000000-000000000000
	github.com/Dieterbe/artisanalhistogram v0.0.0-20170619072513-f61b7225d304
	github.com/Dieterbe/go-metrics v0.0.0-20181015090856-87383909479d
	github.com/Dieterbe/topic v0.0.0-20141209014555-1850ffda9965
	github.com/Shopify/sarama v1.23.0
	github.com/alyu/configparser v0.0.0-20191103060215-744e9a66e7bc // indirect
	github.com/aws/aws-sdk-go v1.15.54
	github.com/bmizerany/assert v0.0.0-20120716205630-e17e99893cb6
	github.com/cespare/xxhash v0.0.0-00010101000000-000000000000 // indirect
	github.com/dgryski/go-jump v0.0.0-20170409065014-e1f439676b57 // indirect
	github.com/dgryski/go-linlog v0.0.0-20180207191225-edcf2dfd90ff
	github.com/elazarl/go-bindata-assetfs v0.0.0-20151224045452-57eb5e1fc594
	github.com/go-ini/ini v1.38.3 // indirect
	github.com/golang/glog v0.0.0-20210429001901-424d2337a529 // indirect
	github.com/golang/protobuf v0.0.0-20171113180720-1e59b77b52bf // indirect
	github.com/golang/snappy v0.0.1
	github.com/google/go-cmp v0.5.5
	github.com/googleapis/gax-go v2.0.0+incompatible // indirect
	github.com/gorilla/context v0.0.0-20141217160251-215affda49ad // indirect
	github.com/gorilla/handlers v1.4.0
	github.com/gorilla/mux v0.0.0-20140624184626-14cafe285133
	github.com/grafana/configparser v0.0.0-20210622142741-3e3967fd8d20
	github.com/grafana/metrictank v1.0.1-0.20210114150051-52835b9a8775
	github.com/jpillora/backoff v0.0.0-20160414055204-0496a6c14df0
	github.com/kisielk/og-rek v0.0.0-20170405223746-ec792bc6e6aa
	github.com/konsorten/go-windows-terminal-sequences v1.0.1 // indirect
	github.com/kr/pretty v0.0.0-20160325215624-add1dbc86daf // indirect
	github.com/kr/text v0.0.0-20150905224508-bb797dc4fb83 // indirect
	github.com/metrics20/go-metrics20 v0.0.0-20180821133656-717ed3a27bf9
	github.com/pelletier/go-toml v1.9.1 // indirect
	github.com/philhofer/fwd v0.0.0-20151120024002-92647f2bd94a // indirect
	github.com/prometheus/procfs v0.0.0-20190425082905-87a4384529e0
	github.com/sirupsen/logrus v1.1.2-0.20181020050904-08e90462da34
	github.com/smartystreets/goconvey v1.6.4 // indirect
	github.com/streadway/amqp v0.0.0-20170521212453-dfe15e360485
	github.com/stretchr/testify v1.3.0
	github.com/taylorchu/toki v0.0.0-20141019163204-20e86122596c
	github.com/tinylib/msgp v1.1.0 // indirect
	github.com/xdg/scram v0.0.0-20180814205039-7eeb5667e42c
	golang.org/x/oauth2 v0.0.0-20180118004544-b28fcf2b08a1 // indirect
	golang.org/x/text v0.3.1-0.20171227012246-e19ae1496984 // indirect
	google.golang.org/api v0.0.0-20180122000316-bc96e9251952 // indirect
	google.golang.org/appengine v1.0.1-0.20170921170648-24e4144ec923 // indirect
	google.golang.org/genproto v0.0.0-20171212231943-a8101f21cf98 // indirect
	google.golang.org/grpc v1.2.1-0.20180119173759-b71aced4a2a1 // indirect
	gopkg.in/ini.v1 v1.62.0 // indirect
	gopkg.in/jcmturner/goidentity.v3 v3.0.0 // indirect
)

replace github.com/cespare/xxhash => github.com/cespare/xxhash/v2 v2.1.1

replace github.com/BurntSushi/toml v0.0.0-00010101000000-000000000000 => github.com/Dieterbe/toml v0.2.1-0.20181015092100-96f3d827bb6c
