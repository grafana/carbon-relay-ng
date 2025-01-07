package cfg

import (
	"fmt"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/grafana/carbon-relay-ng/aggregator"
	"github.com/grafana/carbon-relay-ng/imperatives"
	"github.com/grafana/carbon-relay-ng/matcher"
	"github.com/grafana/carbon-relay-ng/rewriter"
	"github.com/grafana/carbon-relay-ng/route"
	"github.com/grafana/carbon-relay-ng/table"
	"github.com/grafana/metrictank/cluster/partitioner"
	log "github.com/sirupsen/logrus"
)

func InitTable(table table.Interface, config Config, meta toml.MetaData) error {
	err := InitCmd(table, config)
	if err != nil {
		return err
	}

	err = InitBlocklist(table, config)
	if err != nil {
		return err
	}

	err = InitAggregation(table, config)
	if err != nil {
		return err
	}

	err = InitRewrite(table, config)
	if err != nil {
		return err
	}

	err = InitRoutes(table, config, meta)
	if err != nil {
		return err
	}

	return nil
}

func InitCmd(table table.Interface, config Config) error {
	for i, cmd := range config.Init.Cmds {
		log.Infof("applying: %s", cmd)
		err := imperatives.Apply(table, cmd)
		if err != nil {
			log.Errorf("could not apply init cmd #%d: %s", i+1, err.Error())
			return fmt.Errorf("could not apply init cmd #%d", i+1)
		}
	}

	return nil
}

func InitBlocklist(table table.Interface, config Config) error {

	// backwards compat
	blocklist := append(config.BlockList, config.BlackList...)

	for i, entry := range blocklist {
		parts := strings.SplitN(entry, " ", 2)
		if len(parts) < 2 {
			return fmt.Errorf("invalid blocklist cmd #%d", i+1)
		}

		prefix := ""
		notPrefix := ""
		sub := ""
		notSub := ""
		regex := ""
		notRegex := ""

		switch parts[0] {
		case "prefix":
			prefix = parts[1]
		case "notPrefix":
			notPrefix = parts[1]
		case "sub":
			sub = parts[1]
		case "notSub":
			notSub = parts[1]
		case "regex":
			regex = parts[1]
		case "notRegex":
			notRegex = parts[1]
		default:
			return fmt.Errorf("invalid blocklist method for cmd #%d: %s", i+1, parts[1])
		}

		m, err := matcher.New(prefix, notPrefix, sub, notSub, regex, notRegex)
		if err != nil {
			log.Error(err.Error())
			return fmt.Errorf("could not apply blocklist cmd #%d", i+1)
		}

		table.AddBlocklist(&m)
	}

	return nil
}

func InitAggregation(table table.Interface, config Config) error {
	for i, aggConfig := range config.Aggregation {
		// for backwards compatibility we need to check both "sub" and "substr",
		// but "sub" gets preference if both are defined
		sub := aggConfig.Substr
		if len(aggConfig.Sub) > 0 {
			sub = aggConfig.Sub
		}

		matcher, err := matcher.New(aggConfig.Prefix, aggConfig.NotPrefix, sub, aggConfig.NotSub, aggConfig.Regex, aggConfig.NotRegex)
		if err != nil {
			return fmt.Errorf("Failed to instantiate matcher: %s", err)
		}
		agg, err := aggregator.New(aggConfig.Function, matcher, aggConfig.Format, aggConfig.Cache, uint(aggConfig.Interval), uint(aggConfig.Wait), aggConfig.DropRaw, table.GetIn())
		if err != nil {
			log.Error(err.Error())
			return fmt.Errorf("could not add aggregation #%d", i+1)
		}

		table.AddAggregator(agg)
	}

	return nil
}

func InitRewrite(table table.Interface, config Config) error {
	for i, rewriterConfig := range config.Rewriter {
		rw, err := rewriter.New(rewriterConfig.Old, rewriterConfig.New, rewriterConfig.Not, rewriterConfig.Max)
		if err != nil {
			log.Error(err.Error())
			return fmt.Errorf("could not add rewriter #%d", i+1)
		}

		table.AddRewriter(rw)
	}

	return nil
}

func InitRoutes(table table.Interface, config Config, meta toml.MetaData) error {
	for _, routeConfig := range config.Route {
		// for backwards compatibility we need to check both "sub" and "substr",
		// but "sub" gets preference if both are defined
		sub := routeConfig.Substr
		if len(routeConfig.Sub) > 0 {
			sub = routeConfig.Sub
		}
		matcher, err := matcher.New(routeConfig.Prefix, routeConfig.NotPrefix, sub, routeConfig.NotSub, routeConfig.Regex, routeConfig.NotRegex)
		if err != nil {
			return fmt.Errorf("Failed to instantiate matcher: %s", err)
		}

		switch routeConfig.Type {
		case "sendAllMatch":
			destinations, err := imperatives.ParseDestinations(routeConfig.Destinations, table, true, routeConfig.Key)
			if err != nil {
				log.Error(err.Error())
				return fmt.Errorf("could not parse destinations for route '%s'", routeConfig.Key)
			}
			if len(destinations) == 0 {
				return fmt.Errorf("must get at least 1 destination for route '%s'", routeConfig.Key)
			}

			route, err := route.NewSendAllMatch(routeConfig.Key, matcher, destinations)
			if err != nil {
				log.Error(err.Error())
				return fmt.Errorf("error adding route '%s'", routeConfig.Key)
			}
			table.AddRoute(route)
		case "sendFirstMatch":
			destinations, err := imperatives.ParseDestinations(routeConfig.Destinations, table, true, routeConfig.Key)
			if err != nil {
				log.Error(err.Error())
				return fmt.Errorf("could not parse destinations for route '%s'", routeConfig.Key)
			}
			if len(destinations) == 0 {
				return fmt.Errorf("must get at least 1 destination for route '%s'", routeConfig.Key)
			}

			route, err := route.NewSendFirstMatch(routeConfig.Key, matcher, destinations)
			if err != nil {
				log.Error(err.Error())
				return fmt.Errorf("error adding route '%s'", routeConfig.Key)
			}
			table.AddRoute(route)
		case "consistentHashing", "consistentHashing-v2":
			destinations, err := imperatives.ParseDestinations(routeConfig.Destinations, table, false, routeConfig.Key)
			if err != nil {
				log.Error(err.Error())
				return fmt.Errorf("could not parse destinations for route '%s'", routeConfig.Key)
			}
			if len(destinations) < 2 {
				return fmt.Errorf("must get at least 2 destination for route '%s'", routeConfig.Key)
			}

			withFix := (routeConfig.Type == "consistentHashing-v2")

			route, err := route.NewConsistentHashing(routeConfig.Key, matcher, destinations, withFix)
			if err != nil {
				log.Error(err.Error())
				return fmt.Errorf("error adding route '%s'", routeConfig.Key)
			}
			table.AddRoute(route)
		case "grafanaNet":

			cfg, err := route.NewGrafanaNetConfig(routeConfig.Addr, routeConfig.ApiKey, routeConfig.SchemasFile, routeConfig.AggregationFile)
			if err != nil {
				log.Error(err.Error())
				log.Info("grafanaNet route configuration details: https://github.com/grafana/carbon-relay-ng/blob/main/docs/config.md#grafananet-route")
				return fmt.Errorf("error adding route '%s'", routeConfig.Key)
			}

			// by merely looking at a boolean field we can't differentiate between:
			// * the user not specifying the option (which should leave our config unaffected)
			// * the user specifying it as false explicitly (which should set it to false)
			// both cases result in routeConfig.SslVerify being set to false.
			// So, we must look at the metadata returned by the config parser.

			routeMeta := meta.Mapping["route"].([]map[string]interface{})

			// Note: toml library allows arbitrary casing of properties,
			// and the map keys are these properties as specified by user
			// so we can't look up directly
			for _, routemeta := range routeMeta {
				for k, v := range routemeta {
					if strings.ToLower(k) == "key" && v == routeConfig.Key {
						for k2, v2 := range routemeta {
							if strings.ToLower(k2) == "sslverify" {
								cfg.SSLVerify = v2.(bool)
							}
							if strings.ToLower(k2) == "spool" {
								cfg.Spool = v2.(bool)
							}
							if strings.ToLower(k2) == "blocking" {
								cfg.Blocking = v2.(bool)
							}
						}
					}
				}
			}

			if routeConfig.BufSize != 0 {
				cfg.BufSize = routeConfig.BufSize
			}
			if routeConfig.FlushMaxNum != 0 {
				cfg.FlushMaxNum = routeConfig.FlushMaxNum
			}
			if routeConfig.FlushMaxWait != 0 {
				cfg.FlushMaxWait = time.Duration(routeConfig.FlushMaxWait) * time.Millisecond
			}
			if routeConfig.Timeout != 0 {
				cfg.Timeout = time.Millisecond * time.Duration(routeConfig.Timeout)
			}
			if routeConfig.Concurrency != 0 {
				cfg.Concurrency = routeConfig.Concurrency
			}
			if routeConfig.OrgId != 0 {
				cfg.OrgID = routeConfig.OrgId
			}
			if routeConfig.ErrBackoffMin != 0 {
				cfg.ErrBackoffMin = time.Millisecond * time.Duration(routeConfig.ErrBackoffMin)
			}
			if routeConfig.ErrBackoffFactor != 0 {
				cfg.ErrBackoffFactor = routeConfig.ErrBackoffFactor
			}

			route, err := route.NewGrafanaNet(routeConfig.Key, matcher, cfg)
			if err != nil {
				log.Error(err.Error())
				return fmt.Errorf("error adding route '%s'", routeConfig.Key)
			}
			table.AddRoute(route)
		case "kafkaMdm":
			var bufSize = int(1e7)  // since a message is typically around 100B this is 1GB
			var flushMaxNum = 10000 // number of metrics
			var flushMaxWait = 500  // in ms
			var timeout = 2000      // in ms
			var orgId = 1

			_, err := partitioner.NewKafka(routeConfig.PartitionBy)
			if err != nil {
				return fmt.Errorf("config error for route '%s': %s", routeConfig.Key, err.Error())
			}

			if routeConfig.BufSize != 0 {
				bufSize = routeConfig.BufSize
			}
			if routeConfig.FlushMaxNum != 0 {
				flushMaxNum = routeConfig.FlushMaxNum
			}
			if routeConfig.FlushMaxWait != 0 {
				flushMaxWait = routeConfig.FlushMaxWait
			}
			if routeConfig.Timeout != 0 {
				timeout = routeConfig.Timeout
			}
			if routeConfig.OrgId != 0 {
				orgId = routeConfig.OrgId
			}

			route, err := route.NewKafkaMdm(routeConfig.Key, matcher, routeConfig.Topic, routeConfig.Codec, routeConfig.SchemasFile, routeConfig.PartitionBy, routeConfig.Brokers, bufSize, orgId, flushMaxNum, flushMaxWait, timeout, routeConfig.Blocking, routeConfig.TLSEnabled, routeConfig.TLSSkipVerify, routeConfig.TLSClientCert, routeConfig.TLSClientKey, routeConfig.SASLEnabled, routeConfig.SASLMechanism, routeConfig.SASLUsername, routeConfig.SASLPassword)
			if err != nil {
				log.Error(err.Error())
				return fmt.Errorf("error adding route '%s'", routeConfig.Key)
			}
			table.AddRoute(route)
		case "pubsub":
			var codec = "gzip"
			var format = "plain"                    // aka graphite 'linemode'
			var bufSize = int(1e7)                  // since a message is typically around 100B this is 1GB
			var flushMaxSize = int(1e7) - int(4096) // 5e6 = 5MB. max size of message. Note google limits to 10M, but we want to limit to less to account for overhead
			var flushMaxWait = 1000                 // in ms

			if routeConfig.Codec != "" {
				codec = routeConfig.Codec
			}
			if routeConfig.Format != "" {
				format = routeConfig.Format
			}
			if routeConfig.BufSize != 0 {
				bufSize = routeConfig.BufSize
			}
			if routeConfig.FlushMaxSize != 0 {
				flushMaxSize = routeConfig.FlushMaxSize
			}
			if routeConfig.FlushMaxWait != 0 {
				flushMaxWait = routeConfig.FlushMaxWait
			}

			route, err := route.NewPubSub(routeConfig.Key, matcher, routeConfig.Project, routeConfig.Topic, format, codec, bufSize, flushMaxSize, flushMaxWait, routeConfig.Blocking)
			if err != nil {
				log.Error(err.Error())
				return fmt.Errorf("error adding route '%s'", routeConfig.Key)
			}
			table.AddRoute(route)
		case "cloudWatch":
			var bufSize = int(1e7)            // since a message is typically around 100B this is 1GB
			var flushMaxSize = int(20)        // Amazon limits to 20 MetricDatum/PutMetricData request https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/cloudwatch_limits.html
			var flushMaxWait = 10000          // in ms
			var storageResolution = int64(60) // Default CloudWatch resolution is 60s
			var awsProfile = ""
			var awsRegion = ""
			var awsNamespace = ""
			var awsDimensions [][]string

			if routeConfig.BufSize != 0 {
				bufSize = routeConfig.BufSize
			}
			if routeConfig.FlushMaxSize != 0 {
				flushMaxSize = routeConfig.FlushMaxSize
			}
			if routeConfig.FlushMaxWait != 0 {
				flushMaxWait = routeConfig.FlushMaxWait
			}
			if routeConfig.Profile != "" {
				awsProfile = routeConfig.Profile
			}
			if routeConfig.Region != "" {
				awsRegion = routeConfig.Region
			}
			if routeConfig.Namespace != "" {
				awsNamespace = routeConfig.Namespace
			}
			if len(routeConfig.Dimensions) > 0 {
				awsDimensions = routeConfig.Dimensions
			}
			if routeConfig.StorageResolution != 0 {
				storageResolution = routeConfig.StorageResolution
			}

			route, err := route.NewCloudWatch(routeConfig.Key, matcher, awsProfile, awsRegion, awsNamespace, awsDimensions, bufSize, flushMaxSize, flushMaxWait, storageResolution, routeConfig.Blocking)
			if err != nil {
				log.Error(err.Error())
				return fmt.Errorf("error adding route '%s'", routeConfig.Key)
			}
			table.AddRoute(route)
		default:
			return fmt.Errorf("unrecognized route type '%s'", routeConfig.Type)
		}
	}

	return nil
}
