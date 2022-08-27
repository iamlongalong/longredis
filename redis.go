package longredis

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	json "github.com/json-iterator/go"
	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
)

var (
	subClient     = make(map[string]*client)
	subClientLock = sync.Mutex{}
)

var Nil = redis.Nil

type UniversalClient = redis.UniversalClient

type client struct {
	prefix   string
	clientid string

	conn redis.UniversalClient

	requestTimeout time.Duration
	defaultExpire  time.Duration
	opt            Option

	logger Logger
}

func (c *client) clone() *client {
	cli := *c
	return &cli
}

func (c *client) RedisConn() redis.UniversalClient {
	return c.conn
}

func newConn(opt Option) redis.UniversalClient {
	redisConn := redis.NewUniversalClient(convertRedisOption(opt))
	redisConn.AddHook(&injectorHook{clientid: genID(8)})

	if !opt.DisableErrorLog {
		redisConn.AddHook(newErrorLogHook(opt.logger))
	}

	if !opt.Tracing.Disable {
		redisConn.AddHook(newTracingHook(opt.Tracing))
	}

	if !opt.Metrics.Disable {
		redisConn.AddHook(&metricsHook{})
	}

	if !opt.SlowLog.Disable {
		redisConn.AddHook(&slowlog{
			slowTime: opt.SlowLog.SlowTime,
			logLevel: opt.SlowLog.LogLevel,
		})
	}

	return redisConn
}

func newClient(opt Option) (*client, error) {
	// 处理 redis 类型与地址的问题
	switch opt.RedisType {
	case RedisTypeSingle, RedisTypeSingleton:
		if len(opt.Addrs) >= 1 {
			opt.Addrs = opt.Addrs[0:1]
		}
	case RedisTypeCluster, "": // 默认为 cluster
		if len(opt.Addrs) == 1 {
			opt.Addrs = append(opt.Addrs, "")
		}
	}

	// 处理各类默认值
	{
		if opt.DefaultExpire == 0 {
			opt.DefaultExpire = time.Hour * 24 * 7
		}
		if opt.RequestTimeout == 0 {
			opt.RequestTimeout = time.Second * 3
		}
		if opt.DialTimeout == 0 {
			opt.DialTimeout = time.Second * 5
		}
		if opt.ReadTimeout == 0 {
			opt.ReadTimeout = time.Second * 5
		}
		if opt.WriteTimeout == 0 {
			opt.WriteTimeout = time.Second * 5
		}
		if opt.MaxConnAge == 0 {
			opt.MaxConnAge = time.Minute * 10
		}
		if opt.PoolTimeout == 0 {
			opt.PoolTimeout = time.Minute * 10
		}
		if opt.IdleTimeout == 0 {
			opt.IdleTimeout = time.Minute * 10
		}
		if opt.IdleCheckFrequency == 0 {
			opt.IdleCheckFrequency = time.Minute
		}
		if opt.PoolSize == 0 {
			opt.PoolSize = 5
		}
		if opt.MinIdleConns == 0 {
			opt.MinIdleConns = 2
		}
	}

	{
		if opt.Supervisor.CheckInterval == 0 {
			opt.Supervisor.CheckInterval = time.Second * 10
		}
		if opt.Supervisor.PingTimeout == 0 {
			opt.Supervisor.PingTimeout = opt.WriteTimeout
		}
		if opt.Supervisor.CheckMaxRetries == 0 {
			opt.Supervisor.CheckMaxRetries = 5
		}
	}

	{
		if !opt.Tracing.Disable && opt.Tracing.Tracer == nil {
			opt.Tracing.Tracer = opentracing.GlobalTracer()
		}
	}

	{
		if opt.SlowLog.SlowTime == 0 {
			opt.SlowLog.SlowTime = time.Millisecond * 300
		}

		if opt.SlowLog.LogLevel.val == 0 {
			opt.SlowLog.LogLevel = LevelError
		}

		if opt.SlowLog.LogLevelStr != "" {
			opt.SlowLog.LogLevel = ParseLevel(opt.SlowLog.LogLevelStr)
		}
	}

	{
		if opt.Metrics.Namespace != "" || opt.Metrics.Subsystem != "" { // 有改动
			if opt.Metrics.Namespace == "" {
				opt.Metrics.Namespace = namespace
			}
			if opt.Metrics.Subsystem == "" {
				opt.Metrics.Subsystem = subsystem
			}

			ResetMetrics(opt.Metrics.Namespace, opt.Metrics.Subsystem)
		}
	}

	{
		if opt.Log == nil {
			opt.Log = dfLogger
		}
	}

	Log(LevelInfo, "create new client", "hosts", strings.Join(opt.Addrs, ","), "redisType", opt.RedisType)

	redisConn := newConn(opt)

	c := &client{
		conn:           redisConn,
		prefix:         opt.Prefix,
		logger:         opt.Log,
		defaultExpire:  opt.DefaultExpire,
		requestTimeout: opt.RequestTimeout,

		opt: opt,
	}

	go c.check()

	if err := c.Ping(context.Background()); err != nil {
		c.logger.Log(LevelError, fmt.Sprintf("redis client ping error: %s", err), "address", strings.Join(c.opt.Addrs, ","))
		return nil, errors.Wrap(err, "[iredis] newClient ping error")
	}

	return c, nil
}

func (c *client) SubPrefix(subprefix string) *client {
	prefix := ""
	if c.prefix == "" {
		prefix = subprefix
	} else {
		prefix = c.prefix + ":" + subprefix
	}

	var cc *client
	var ok bool

	subClientLock.Lock()
	defer subClientLock.Unlock()

	if cc, ok = subClient[prefix]; !ok {
		cc = c.clone()
		cc.prefix = prefix
		subClient[prefix] = cc
	}
	return cc
}

// 获取redis连接状态
func (c *client) RedisStatus() *redis.PoolStats {
	return c.conn.PoolStats()
}

func (c *client) WithPrefix(key string) string {
	if c.prefix == "" {
		return key
	}
	return c.prefix + ":" + key
}

func (c *client) SetEx(ctx context.Context, key string, val interface{}, expires ...time.Duration) error {
	return c.Set(ctx, key, val, expires...)
}

func (c *client) Expire(ctx context.Context, key string, duration time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, c.requestTimeout)
	defer cancel()
	key = c.keyPre(key)
	cmd := c.conn.Expire(ctx, key, duration)

	return cmd.Err()
}

func (c *client) SetNx(ctx context.Context, key string, val interface{}, expire ...time.Duration) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, c.requestTimeout)
	defer cancel()
	key = c.keyPre(key)
	duration := time.Minute * 8
	if len(expire) > 0 {
		duration = expire[0]
	}

	cmd := c.conn.SetNX(ctx, key, val, duration)

	return cmd.Val(), cmd.Err()
}

func (c *client) Set(ctx context.Context, key string, val interface{}, expires ...time.Duration) error {
	expire := time.Duration(0)
	if len(expires) > 0 {
		expire = expires[0]
	}
	ctx, cancel := context.WithTimeout(ctx, c.requestTimeout)
	defer cancel()
	key = c.keyPre(key)
	cmd := c.conn.Set(ctx, key, val, expire)

	return cmd.Err()
}

func (c *client) Get(ctx context.Context, key string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, c.requestTimeout)
	defer cancel()
	key = c.keyPre(key)

	cmd := c.conn.Get(ctx, key)
	if err := cmd.Err(); err != nil {
		return nil, err
	}

	return cmd.Bytes()
}

func (c *client) GeBool(ctx context.Context, key string) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, c.requestTimeout)
	defer cancel()
	key = c.keyPre(key)

	cmd := c.conn.Get(ctx, key)

	return cmd.Bool()
}

func (c *client) GetUInt64(ctx context.Context, key string) (uint64, error) {
	ctx, cancel := context.WithTimeout(ctx, c.requestTimeout)
	defer cancel()
	key = c.keyPre(key)

	return c.conn.Get(ctx, key).Uint64()
}

func (c *client) GetBind(ctx context.Context, key string, obj interface{}) error {
	ctx, cancel := context.WithTimeout(ctx, c.requestTimeout)
	defer cancel()
	key = c.keyPre(key)

	cmd := c.conn.Get(ctx, key)

	err := cmd.Scan(obj)
	if err == nil {
		return nil
	}

	err = json.Unmarshal([]byte(cmd.Val()), obj)
	if err != nil {
		c.logger.Log(LevelError, fmt.Sprintf("GetBind failed : %s", err), "command", fmt.Sprintf("get %s", key))
		return err
	}

	return nil
}

func (c *client) GetString(ctx context.Context, key string) (string, error) {
	b, err := c.Get(ctx, key)

	return string(b), err
}

// cluster hmset
func (c *client) HMSet(ctx context.Context, key string, vals ...interface{}) error {
	ctx, cancel := context.WithTimeout(ctx, c.requestTimeout)
	defer cancel()
	key = c.keyPre(key)

	cmd := c.conn.HMSet(ctx, key, vals...)
	return cmd.Err()
}

// cluster hmget
func (c *client) HMGet(ctx context.Context, key string, fields ...string) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, c.requestTimeout)
	defer cancel()
	key = c.keyPre(key)

	slices, err := c.conn.HMGet(ctx, key, fields...).Result()
	if err != nil {
		return nil, err
	}
	results := make([]string, 0, len(slices))
	for _, s := range slices {
		result, ok := s.(string)
		if ok {
			results = append(results, result)
		} else {
			results = append(results, "")
		}
	}
	return results, nil
}

func (c *client) Del(ctx context.Context, key string) error {
	ctx, cancel := context.WithTimeout(ctx, c.requestTimeout)
	defer cancel()
	key = c.keyPre(key)

	return c.conn.Del(ctx, key).Err()
}

func (c *client) Exists(ctx context.Context, key string) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, c.requestTimeout)
	defer cancel()
	key = c.keyPre(key)

	i, err := c.conn.Exists(ctx, key).Result()
	return i > 0, err
}

func (c *client) Type(ctx context.Context, key string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, c.requestTimeout)
	defer cancel()
	key = c.keyPre(key)

	res := c.conn.Type(ctx, key)
	return res.Val(), res.Err()
}

func (c *client) Incr(ctx context.Context, key string) (int64, error) {
	ctx, cancel := context.WithTimeout(ctx, c.requestTimeout)
	defer cancel()
	key = c.keyPre(key)

	res := c.conn.Incr(ctx, key)
	return res.Val(), res.Err()
}
func (c *client) Decr(ctx context.Context, key string) (int64, error) {
	ctx, cancel := context.WithTimeout(ctx, c.requestTimeout)
	defer cancel()
	key = c.keyPre(key)

	res := c.conn.Decr(ctx, key)
	return res.Val(), res.Err()
}

func (c *client) Append(ctx context.Context, key string, val string) (int64, error) {
	ctx, cancel := context.WithTimeout(ctx, c.requestTimeout)
	defer cancel()
	key = c.keyPre(key)

	res := c.conn.Append(ctx, key, val)
	return res.Val(), res.Err()
}

func (c *client) RenameNX(ctx context.Context, key, newKey string) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, c.requestTimeout)
	defer cancel()
	key = c.keyPre(key)
	newKey = c.keyPre(newKey)

	res := c.conn.RenameNX(ctx, key, newKey)
	return res.Val(), res.Err()
}

func (c *client) LPop(ctx context.Context, key string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, c.requestTimeout)
	defer cancel()
	key = c.keyPre(key)

	res := c.conn.LPop(ctx, key)

	return res.Val(), res.Err()
}

func (c *client) HSetNX(ctx context.Context, key, field string, val interface{}) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, c.requestTimeout)
	defer cancel()
	key = c.keyPre(key)

	res := c.conn.HSetNX(ctx, key, field, val)
	return res.Val(), res.Err()
}

func (c *client) HSet(ctx context.Context, key, field string, val interface{}) error {
	ctx, cancel := context.WithTimeout(ctx, c.requestTimeout)
	defer cancel()
	key = c.keyPre(key)

	return c.conn.HSet(ctx, key, field, val).Err()
}

func (c *client) HGetString(ctx context.Context, key, field string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, c.requestTimeout)
	defer cancel()
	key = c.keyPre(key)

	res := c.conn.HGet(ctx, key, field)
	return res.Val(), res.Err()
}

func (c *client) HGet(ctx context.Context, key, field string) ([]byte, error) {
	res, err := c.HGetString(ctx, key, field)
	return []byte(res), err
}

func (c *client) HGetBytes(ctx context.Context, key, field string) ([]byte, error) {
	res, err := c.HGetString(ctx, key, field)
	return []byte(res), err
}

func (c *client) HGetBool(ctx context.Context, key, field string) (bool, error) {
	res, err := c.HGetString(ctx, key, field)

	return res == "true" || res == "TRUE" || res == "t" || res == "True" || res == "T" || res == "1", err
}

func (c *client) HGetInt64(ctx context.Context, key, field string) (int64, error) {
	ctx, cancel := context.WithTimeout(ctx, c.requestTimeout)
	defer cancel()
	key = c.keyPre(key)

	var res *redis.StringCmd
	if res = c.conn.HGet(ctx, key, field); res.Err() != nil {
		return 0, res.Err()
	}
	return res.Int64()
}

func (c *client) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	var err error

	ctx, cancel := context.WithTimeout(ctx, c.requestTimeout)
	defer cancel()
	key = c.keyPre(key)

	var res *redis.StringStringMapCmd
	if res = c.conn.HGetAll(ctx, key); res.Err() != nil {
		return nil, res.Err()
	}
	var hmap map[string]string
	if hmap, err = res.Result(); err != nil {
		return nil, err
	}
	return hmap, nil
}

func (c *client) HGetBind(ctx context.Context, key, field string, dest interface{}) error {
	res, err := c.HGetString(ctx, key, field)
	if err != nil {
		return err
	}
	if err = json.UnmarshalFromString(res, dest); err != nil {
		return err
	}
	return nil
}

func (c *client) HDel(ctx context.Context, key string, fields ...string) error {
	ctx, cancel := context.WithTimeout(ctx, c.requestTimeout)
	defer cancel()
	key = c.keyPre(key)

	res := c.conn.HDel(ctx, key, fields...)
	return res.Err()
}

func (c *client) Do(ctx context.Context, cmd string, key string, args ...interface{}) (interface{}, error) {
	ctx, cancel := context.WithTimeout(ctx, c.requestTimeout)
	defer cancel()
	key = c.keyPre(key)

	newArgs := []interface{}{cmd, key}
	newArgs = append(newArgs, args...)

	res, err := c.conn.Do(ctx, newArgs...).Result()
	return res, err
}

func (c *client) TxPipeline(ctx context.Context) redis.Pipeliner {
	return c.conn.TxPipeline()
}

func (c *client) keyPre(key string) string {
	if c.prefix == "" {
		return key
	}
	if strings.HasPrefix(key, c.prefix+":") {
		return key
	}
	return fmt.Sprintf("%s:%s", c.prefix, key)
}

type TXCmd struct {
	Cmd  string
	Args []interface{}
}

func (c *TXCmd) DoArgs(keyPre func(string) string) []interface{} {
	var args []interface{}
	args = append(args, c.Cmd)
	args = append(args, c.Args...)
	return args
}

func (c *client) SetTxCmd(ctx context.Context, k string, v interface{}) *TXCmd {
	return &TXCmd{
		Cmd:  "SET",
		Args: []interface{}{c.keyPre(k), v},
	}
}

func (c *client) SetExTxCmd(ctx context.Context, k string, v interface{}, durations ...time.Duration) *TXCmd {
	durs := c.defaultExpire
	if len(durations) > 0 {
		durs = durations[0]
	}
	secs := int64(durs / time.Second)
	return &TXCmd{
		Cmd:  "SET",
		Args: []interface{}{c.keyPre(k), v, "ex", secs},
	}
}

func (c *client) HSetTxCmd(ctx context.Context, k, field string, v interface{}) *TXCmd {
	return &TXCmd{
		Cmd:  "HSET",
		Args: []interface{}{c.keyPre(k), field, v},
	}
}

func (c *client) GetState(ctx context.Context) bool {
	ctx, cancel := context.WithTimeout(ctx, c.requestTimeout)
	defer cancel()

	res, err := c.conn.Ping(ctx).Result()
	if err == nil && res == "PONG" {
		return true
	}
	return false
}

func (c *client) LLen(ctx context.Context, key string) (int64, error) {
	ctx, cancel := context.WithTimeout(ctx, c.requestTimeout)
	defer cancel()
	key = c.keyPre(key)

	var res *redis.IntCmd
	if res = c.conn.LLen(ctx, key); res.Err() != nil {
		return 0, res.Err()
	}
	return res.Val(), nil
}

func (c *client) LIndex(ctx context.Context, key string, index int64) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, c.requestTimeout)
	defer cancel()
	key = c.keyPre(key)

	var res *redis.StringCmd
	if res = c.conn.LIndex(ctx, key, index); res.Err() != nil {
		return "", res.Err()
	}
	return res.Val(), nil
}

// cluster rpush fields长度为零会报错
func (c *client) RPush(ctx context.Context, key string, fields ...interface{}) error {
	ctx, cancel := context.WithTimeout(ctx, c.requestTimeout)
	defer cancel()
	key = c.keyPre(key)

	var res *redis.IntCmd
	if res = c.conn.RPush(ctx, key, fields...); res.Err() != nil {
		return res.Err()
	}
	return nil
}

func (c *client) GetPoolState(ctx context.Context) *redis.PoolStats {
	return c.conn.PoolStats()
}

// HIncrBy hash incr
func (c *client) HIncrBy(ctx context.Context, key, field string, incrBy int) (int64, error) {
	ctx, cancel := context.WithTimeout(ctx, c.requestTimeout)
	defer cancel()
	key = c.keyPre(key)
	res, err := c.conn.HIncrBy(ctx, key, field, int64(incrBy)).Result()
	if err != nil {
		return 0, err
	}
	return res, nil
}

func (c *client) Rename(ctx context.Context, oldKey, newKey string) error {
	ctx, cancel := context.WithTimeout(ctx, c.requestTimeout)
	defer cancel()
	oldKey = c.keyPre(oldKey)
	newKey = c.keyPre(newKey)
	_, err := c.conn.Rename(ctx, oldKey, newKey).Result()
	if err != nil {
		return err
	}
	return nil
}

func (c *client) TTL(ctx context.Context, key string) (time.Duration, error) {
	ctx, cancel := context.WithTimeout(ctx, c.requestTimeout)
	defer cancel()
	key = c.keyPre(key)

	dur, err := c.conn.TTL(ctx, key).Result()
	if err != nil {
		return time.Duration(0), err
	}
	return dur, nil
}

func (c *client) ZAdd(ctx context.Context, key string, score float64, member interface{}) error {
	ctx, cancel := context.WithTimeout(ctx, c.requestTimeout)
	defer cancel()
	key = c.keyPre(key)

	_, err := c.conn.ZAdd(ctx, key, &redis.Z{Score: score, Member: member}).Result()
	if err != nil {
		return err
	}
	return nil
}

func (c *client) ZCount(ctx context.Context, key string, min, max float64) (int64, error) {
	ctx, cancel := context.WithTimeout(ctx, c.requestTimeout)
	defer cancel()
	key = c.keyPre(key)

	count, err := c.conn.ZCount(ctx, key, fmt.Sprintf("%v", min), fmt.Sprintf("%v", max)).Result()
	if err != nil {
		return 0, err
	}
	return count, nil
}

type Members = redis.Z

func (c *client) ZAddBatch(ctx context.Context, key string,
	members []Members) error {
	ctx, cancel := context.WithTimeout(ctx, c.requestTimeout)
	defer cancel()
	key = c.keyPre(key)

	var zmembers []*redis.Z
	for _, v := range members {
		m := &redis.Z{Score: v.Score, Member: v.Member}
		zmembers = append(zmembers, m)
	}

	return c.conn.ZAdd(ctx, key, zmembers...).Err()
}

func (c *client) ZRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, c.requestTimeout)
	defer cancel()
	key = c.keyPre(key)

	return c.conn.ZRange(ctx, key, start, stop).Result()
}

func (c *client) ZRevRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, c.requestTimeout)
	defer cancel()
	key = c.keyPre(key)

	return c.conn.ZRevRange(ctx, key, start, stop).Result()
}

func (c *client) ZRangeWithScore(ctx context.Context, key string, start, stop int64) ([]redis.Z, error) {
	ctx, cancel := context.WithTimeout(ctx, c.requestTimeout)
	defer cancel()
	key = c.keyPre(key)

	return c.conn.ZRangeWithScores(ctx, key, start, stop).Result()
}

func (c *client) ZRemRangeByRank(ctx context.Context, key string, start, stop int64) (int64, error) {
	ctx, cancel := context.WithTimeout(ctx, c.requestTimeout)
	defer cancel()
	key = c.keyPre(key)

	return c.conn.ZRemRangeByRank(ctx, key, start, stop).Result()
}

func (c *client) ZCard(ctx context.Context, key string) (int64, error) {
	ctx, cancel := context.WithTimeout(ctx, c.requestTimeout)
	defer cancel()
	key = c.keyPre(key)

	return c.conn.ZCard(ctx, key).Result()
}

func (c *client) SMembers(ctx context.Context, key string) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, c.requestTimeout)
	defer cancel()
	key = c.keyPre(key)

	return c.conn.SMembers(ctx, key).Result()
}

func (c *client) SIsMember(ctx context.Context, key string, value interface{}) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, c.requestTimeout)
	defer cancel()
	key = c.keyPre(key)

	return c.conn.SIsMember(ctx, key, value).Result()
}

// SAdd  set 新增， 可以使用 []string []uint64 等简单切片类型，也可以使用 多个基本数值，但不能混合使用 切片 和 数值
func (c *client) SAdd(ctx context.Context, key string, members ...interface{}) error {
	ctx, cancel := context.WithTimeout(ctx, c.requestTimeout)
	defer cancel()
	key = c.keyPre(key)

	return c.conn.SAdd(ctx, key, members...).Err()
}

func (c *client) SCard(ctx context.Context, key string) (int64, error) {
	ctx, cancel := context.WithTimeout(ctx, c.requestTimeout)
	defer cancel()
	key = c.keyPre(key)

	return c.conn.SCard(ctx, key).Result()
}

func (c *client) SRem(ctx context.Context, key string, members ...interface{}) error {
	ctx, cancel := context.WithTimeout(ctx, c.requestTimeout)
	defer cancel()
	key = c.keyPre(key)

	return c.conn.SRem(ctx, key, members...).Err()
}

func (c *client) Ping(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, c.requestTimeout)
	defer cancel()

	return c.conn.Ping(ctx).Err()
}

// 重连机制
func (c *client) check() {
	var cnt = 0
	ticker := time.NewTicker(c.opt.Supervisor.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			err := c.Ping(context.Background())
			if err != nil {
				cnt++
				c.logger.Log(LevelError, fmt.Sprintf("redis client ping error: %s", err), "adress", strings.Join(c.opt.Addrs, ","), "retrytimes", cnt)
			} else {
				cnt = 0
			}

			if cnt >= c.opt.Supervisor.CheckMaxRetries {
				c.resetRedisConn()
				cnt = 0
			}

			// 更新 metrics // 姑且先放在 check 下
			updateMetrics(c)
		}
	}

}

func updateMetrics(c *client) {
	states := c.GetPoolState(context.Background())

	MissCounter.WithLabelValues(c.clientid).Set(float64(states.Misses))
	ConnsGuage.WithLabelValues(c.clientid).Set(float64(states.TotalConns))
	IdleConnsGuage.WithLabelValues(c.clientid).Set(float64(states.IdleConns))
}

func (c *client) resetRedisConn() {
	newConn := newConn(c.opt)
	_ = c.conn.Close()
	c.conn = newConn

	c.logger.Log(LevelInfo, "redis connect has been reseted")
}
