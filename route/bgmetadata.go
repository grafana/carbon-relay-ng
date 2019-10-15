package route

import (
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"hash/crc32"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/graphite-ng/carbon-relay-ng/encoding"
	"github.com/graphite-ng/carbon-relay-ng/matcher"
	"github.com/graphite-ng/carbon-relay-ng/metrics"
	"github.com/willf/bloom"
	"go.uber.org/zap"
)

type shard struct {
	num    int
	filter bloom.BloomFilter
	lock   sync.RWMutex
}

// BloomFilterConfig contains filter size and false positive chance for all bloom filters
type BloomFilterConfig struct {
	N              uint
	P              float64
	ShardingFactor int
	Cache          string
	ClearInterval  time.Duration
	ClearWait      time.Duration
	logger         *zap.Logger
}

// BgMetadata contains data required to start, stop and reset a metric metadata producer.
type BgMetadata struct {
	baseRoute
	shards []shard
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	bfCfg  BloomFilterConfig
	mm     metrics.BgMetadataMetrics
}

// NewBloomFilterConfig creates a new BloomFilterConfig
func NewBloomFilterConfig(n uint, p float64, shardingFactor int, cache string, clearInterval, clearWait time.Duration) (BloomFilterConfig, error) {
	bfc := BloomFilterConfig{
		N:              n,
		P:              p,
		ShardingFactor: shardingFactor,
		Cache:          cache,
		ClearInterval:  clearInterval,
		ClearWait:      clearWait,
		logger:         zap.L(),
	}
	if clearWait != 0 {
		bfc.ClearWait = clearWait
	} else {
		bfc.ClearWait = clearInterval / time.Duration(shardingFactor)
		bfc.logger.Warn("overiding clear_wait value", zap.Duration("clear_wait", bfc.ClearWait))
	}
	return bfc, nil
}

// NewBgMetadataRoute creates BgMetadata, starts sharding and filtering incoming metrics.
func NewBgMetadataRoute(key, prefix, sub, regex string, bfCfg BloomFilterConfig) (*BgMetadata, error) {
	m := BgMetadata{
		baseRoute: *newBaseRoute(key, "bg_metadata"),
		shards:    make([]shard, bfCfg.ShardingFactor),
		bfCfg:     bfCfg,
	}

	// init every shard with filter
	for shardNum := 0; shardNum < bfCfg.ShardingFactor; shardNum++ {
		m.shards[shardNum] = shard{
			num:    shardNum,
			filter: *bloom.NewWithEstimates(bfCfg.N, bfCfg.P),
		}
	}

	if m.bfCfg.Cache != "" {
		err := m.createCacheDirectory()
		if err != nil {
			return &m, err
		}
		err = m.validateCachedFilterConfig()
		if err != nil {
			m.logger.Warn("cache validation failed", zap.Error(err))
			err = m.deleteCache()
			if err != nil {
				return &m, err
			}
			err = m.saveFilterConfig()
			if err != nil {
				return &m, err
			}
		} else {
			m.logger.Info("loading cached filter states")
			for shardNum := 0; shardNum < bfCfg.ShardingFactor; shardNum++ {
				err = m.shards[shardNum].loadShardState(*m.logger, bfCfg.Cache)
				if err != nil {
					// skip state loading if file doesn't exist or cannot be loaded
					m.logger.Warn("cannot load state file", zap.Int("shard", shardNum), zap.Error(err))
				}
			}
		}
	}
	m.ctx, m.cancel = context.WithCancel(context.Background())

	m.mm = metrics.NewBgMetadataMetrics(key)
	m.rm = metrics.NewRouteMetrics(key, "bg_metadata", nil)

	go m.clearBloomFilter()

	// matcher required to initialise route.Config for routing table, othewise it will panic
	mt, err := matcher.New(prefix, sub, regex)
	if err != nil {
		return nil, err
	}
	m.config.Store(baseConfig{*mt, nil})

	return &m, nil
}

func (s *shard) loadShardState(logger zap.Logger, cachePath string) error {
	file, err := os.Open(filepath.Join(cachePath, fmt.Sprintf("shard%d.state", s.num)))
	if err != nil {
		return err
	}
	defer file.Close()
	logger.Debug("decoding shard state file", zap.String("filename", file.Name()))
	err = gob.NewDecoder(file).Decode(&s.filter)
	if err != nil {
		return err
	}
	return nil
}

func (s *shard) saveShardState(cachePath string) error {
	stateFile, err := os.Create(filepath.Join(cachePath, fmt.Sprintf("shard%d.state", s.num)))
	if err != nil {
		return err
	}
	defer stateFile.Close()
	err = gob.NewEncoder(stateFile).Encode(&s.filter)
	if err != nil {
		return err
	}
	return nil
}

func (m *BgMetadata) clearBloomFilter() {
	m.logger.Debug("starting goroutine for bloom filter cleanup")
	m.wg.Add(1)
	defer m.wg.Done()
	t := time.NewTicker(m.bfCfg.ClearInterval)
	defer t.Stop()
	for {
		select {
		case <-m.ctx.Done():
			t.Stop()
			return
		case <-t.C:
			for i := range m.shards {
				// use array index to not copy lock
				sh := &m.shards[i]
				select {
				case <-m.ctx.Done():
					t.Stop()
					return
				default:
					sh.lock.Lock()
					if m.bfCfg.Cache != "" {
						err := sh.saveShardState(m.bfCfg.Cache)
						if err != nil {
							m.logger.Error("cannot save shard state to filesystem", zap.Error(err))
						}
					}
					m.logger.Debug("clearing filter for shard", zap.Int("shard_number", i+1))
					sh.filter.ClearAll()
					sh.lock.Unlock()
					time.Sleep(m.bfCfg.ClearWait)
				}
			}
		}
	}
}

func (m *BgMetadata) saveFilterConfig() error {
	configPath := filepath.Join(m.bfCfg.Cache, "bloom_filter_config")
	file, err := os.Create(configPath)
	if err != nil {
		m.logger.Warn("cannot save bloom filter config to filesystem", zap.Error(err))
		return err
	}
	defer file.Close()
	m.logger.Info("saving current filter config", zap.String("file", configPath))
	return gob.NewEncoder(file).Encode(&m.bfCfg)
}

func (m *BgMetadata) validateCachedFilterConfig() error {
	configPath := filepath.Join(m.bfCfg.Cache, "bloom_filter_config")
	file, err := os.Open(configPath)
	if err != nil {
		m.logger.Warn("cannot read bloom filter config", zap.Error(err))
		return err
	}
	defer file.Close()
	loadedConfig := BloomFilterConfig{}
	err = gob.NewDecoder(file).Decode(&loadedConfig)
	if err != nil {
		m.logger.Warn("cannot decode bloom filter config", zap.Error(err))
		return err
	}
	if cmp.Equal(loadedConfig, m.bfCfg, cmpopts.IgnoreUnexported(BloomFilterConfig{})) {
		m.logger.Info("cached bloom filter config matches current")
		m.bfCfg = loadedConfig
		return nil
	}
	return errors.New("cached config does not match current config")
}

func (m *BgMetadata) createCacheDirectory() error {
	f, err := os.Stat(m.bfCfg.Cache)
	if err != nil {
		if os.IsNotExist(err) {
			m.logger.Info("creating directory for cache", zap.String("path", m.bfCfg.Cache))
			mkdirErr := os.MkdirAll(m.bfCfg.Cache, os.ModePerm)
			if mkdirErr != nil {
				m.logger.Error("cannot create cache directory", zap.String("path", m.bfCfg.Cache))
				return mkdirErr
			}
			return nil
		}
		m.logger.Error("cannot not check cache directory", zap.String("path", m.bfCfg.Cache))
		return err
	}
	if !f.IsDir() {
		m.logger.Error("cache path already exists but is not a directory", zap.String("path", m.bfCfg.Cache))
		return err
	}
	m.logger.Info("cache path already exists", zap.String("path", m.bfCfg.Cache))
	return nil
}

func (m *BgMetadata) deleteCache() error {
	m.logger.Warn("deleting cached state and configuration files", zap.String("path", m.bfCfg.Cache))
	names, err := ioutil.ReadDir(m.bfCfg.Cache)
	if err != nil {
		return err
	}
	for _, entry := range names {
		os.RemoveAll(path.Join([]string{m.bfCfg.Cache, entry.Name()}...))
	}
	return nil
}

// Shutdown cancels the context used in BgMetadata and goroutines
// It waits for goroutines to close channels and finish before exiting
func (m *BgMetadata) Shutdown() error {
	m.logger.Info("shutting down bg_metadata")
	m.cancel()
	m.logger.Debug("waiting for goroutines")
	m.wg.Wait()
	return nil
}

// Dispatch puts each datapoint metric name in a bloom filter
// The channel is determined based on the name crc32 hash and sharding factor
func (m *BgMetadata) Dispatch(dp encoding.Datapoint) {
	// increase incoming metric prometheus counter
	m.rm.InMetrics.Inc()
	shardNum := crc32.ChecksumIEEE([]byte(dp.Name)) % uint32(m.bfCfg.ShardingFactor)
	shard := &m.shards[shardNum]
	shard.lock.Lock()
	defer shard.lock.Unlock()
	if !shard.filter.TestString(dp.Name) {
		shard.filter.AddString(dp.Name)
		m.mm.AddedMetrics.Inc()
		// do nothing for now
	} else {
		// don't output metrics already in the filter
		m.mm.FilteredMetrics.Inc()
	}
	return
}

func (m *BgMetadata) Snapshot() Snapshot {
	return makeSnapshot(&m.baseRoute)
}
