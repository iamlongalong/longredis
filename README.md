## 背景
由于业务需要同时兼容 redis 及 redis-cluster，且希望能直接通过 配置 进行切换，因此决定开发该 redis 包。同时，添加了一些统一的 hook 以减少业务方的工作量。

## longredis说明

longredis 是在 go-redis 的基础上做了封装。

主要封装了如下能力：
1. Client 和 连接池管理 (统一的连接池)
2. Client 和 Prefix 绑定
3. Opentracing
4. Metrics
5. Error log
6. Slow query log
7. 超时管理
8. 探活与重新连接

### longredis 基本使用方式

```golang
// 基础 client 初始化
NormalClient, err := longredis.NewClient(longredis.Option{
    Addrs: []string{"127.0.0.1:6379"},
    Password: "xxxxxx",
    RedisType: longredis.ParseRedisType("single"), // 默认为 cluster
    RequestTimeout: time.Second * 1,
})

if err != nil {
    return err
}

// 增加不同的 prefix client => 不同业务使用不同的前缀以做区分
SessionCLient := NormalClient.SubPrefix("session")
FreqLimitClient := NormalClient.SubPrefix("limitation")
// ……

// 使用
sess, err := SessionCLient.Get(ctx, user.ID)
if err != nil {
    // ……
}
```

### 特别的：

特别的：
1. GetBind / HGetBind

```golang
targetObj := &TarObj{}

err := xxClient.GetBind(ctx, "xxkey", targetObj)
if err != nil {
    // ……
}

// 同理，HGetBind 
```

2. GetBool

```golang
isTrue, err := xxClient.GetBool(ctx, "xxkey")
if err != nil {
    // ……
}
// 注意，bool 的 parse 方法为 strconv.ParseBool()，因此只能识别以下字符
// true => "1", "t", "T", "true", "TRUE", "True"
// false => "0", "f", "F", "false", "FALSE", "False"
// 当不是上述字符时，会返回 error 
```

3. TxPipeline

目前对 pipeline 没有做太多封装，是直接返回的 go-redis 的 pipeline，在使用时，需要注意 prefix 的问题。在一些场景下，需要显式使用 xxClient.WithPrefix(key) 进行 prefix 的添加。

```golang
c := redis.XxClient
pipe := c.TxPipeline(ctx)
pipe.Set(ctx, c.WithPrefix(key1), "xxx")
pipe.HSet(ctx, c.WithPrefix(key2), "field1", "xxx")

results, err := pipe.Exec()
if err != nil {
    // ……
}
```

4. 默认值

在 longredis 中，设置了一系列的默认值，具体可以查看 longredis/opt 每项的注释。
不过，绝大多数默认值你都无需关心，除非你遇到了特殊需求。

这几项重要的默认值你应当知道：
- RedisType: redis 类型，枚举值，可为 cluster single sentinel , 默认为 cluster。
- RequestTimeout: 请求超时时间，time.Duration ，默认为 3s，但会以你传的 ctx 的最小超时时间为准。
- SlowLog.SlowTime: 慢查询时间，time.Duration， 默认为 300ms
- Metrics、Tracing、ErrorLog、SlowLog 默认都是开启状态。

### 小技巧
Option 设置了 yaml 和 json 的字段，为蛇形命名风格，因此可以通过 scan 的方式将 配置项中的字段 扫描到 option 中。
假设 config.yaml 中的内容有一段是这样的：
```yaml
redis:
  api_prefix: "api"
  sess_prefix: "session"
  figma_prefix: "fig"
  options:
     addrs: ["127.0.0.1:6379"]
     redis_type: "cluster"
     password: "xxx"
     request_timeout: 1s
     metrics:
        namespace: mastergo
        
```

我们在项目中可以用这样的方式实例化 redis client:

```golang
var (
    BaseClient  *longredis.Client // 基础 client
    ApiClient   *longredis.Client // 用于 普通 api 的 client
    SessClient  *longredis.Client // 用于 session 的 client
    FigmaClient *longredis.Client // 用于 figma 的 client
)

func Startup() error {
    opt := longredis.Option{}
    err := config.DeepGet("redis.options").Scan(&opt)
    if err != nil {
        return errors.Wrap(err, "init redis scan option fail")
    }
    // 设置 默认 logger
    longredis.SetDefaultLogger(log.NewOctopusLog(longredis.LevelDebug))
    
    apiPrefix := config.DeepGet("redis.api_prefix").String("api")
    sessPrefix := config.DeepGet("redis.sess_prefix").String("session")
    figmaPrefix := config.DeepGet("redis.figma_prefix").String("fig")
    
    BaseClient, err = longredis.NewClient(opt) // prefix 为 ""
    if err != nil {
        return errors.Wrap(err, "init redis create client fail")
    }
    
    ApiClient = BaseClient.SubPrefix(apiPrefix)   // prefix 为 api:
    SessClient = ApiClient.SubPrefix(sessPrefix)  // prefix 为 api:session:
    FigmaClient = ApiClient.SubPrefix(fimaPrefix) // prefix 为 api:fig:

    return nil
}
```

### 常见误用说明

1. 由于一些特殊场景的需要，把内部的 go-redis 的 client 开放了出来 (通过 xxClient.RedisConn() 获取)，虽然这个 client 看起来也能使用一系列的 Get/Set 等方法，但该 client 缺少 prefix 的管理，会造成预期的 key 和实际的 key 不一致的问题，因此，不要直接使用 内部client，除非你知道自己在做什么。

```golang
// !!!  错误例子，引以为戒 !!!!
SessClient.RedisConn().Get(ctx, 1001001) // => 对应到命令为 GET 1001001

// 正确使用
SessClient.Get(ctx, 1001001) // => 对应的命令为 GET api:session:1001001
```

2. client 自带了 连接池的管理能力，自带了 prefix 的管理能力，因此仅需要初始化一个 `基础client` (baseClient、normalClient), 其他业务所需的特定前缀的 client，应当使用 `baseClient.SubPrefix(xxx)` 的方式生成。而无需进行多次 longredis.NewClient(opt) 的实例化。

```golang
// !!!  错误例子，引以为戒 !!!!
BaseClient := longredis.NewClient(longredis.Option{
    Addr: ["127.0.0.1:6379"],
    Prefix: "",
})
ApiClient := longredis.NewClient(longredis.Option{
    Addr: ["127.0.0.1:6379"],
    Prefix: "api",
})
SessClient := longredis.NewClient(longredis.Option{
    Addr: ["127.0.0.1:6379"],
    Prefix: "api:session",
})

// 正确使用
BaseClient := longredis.NewClient(longredis.Option{
    Addr: ["127.0.0.1:6379"],
})
ApiClient := BaseClient.SubPrefix("api")
SessClient := ApiClient.SubPrefix("session")

```

## 最后

由于时间有限、水平有限，有所遗漏之处还请担待，发现 bug、有无法满足的需求 等，欢迎提 issue 或者 pr。

## TODO

希望这个库能成为一个 开箱即用的、工程化的 redis 库，还有如下部分没有完成：
- [ ] pipeline 未封装，容易弄混 prefix 
- [ ] 还有大量命令未封装
- [ ] 需要基于该 redis 库，集成多种常用的 redis 工具，例如 
  - [ ] 布隆过滤器
  - [ ] 分布式锁
  - [ ] metrics backend
  - [ ] session
  - [ ] 分布式 limiter
  - [ ] 排行榜
  - [ ] 任务队列
  - [ ] id 生成器
  - [ ] 签到表
- [ ] 需要更多的 example

## 测试环境搭建

可以参考 https://github.com/iamlongalong/redis-instances 进行操作

## PS

redis 真的是一个很牛逼的项目啊，不仅 redis 本身，还有其扩展部分 redis-stack，包含了 
- redisinsight (图形操作面板 + 教程)
- Search (文档查询、全文索引)
- JSON (文档存储，类似 mongodb)
- graph (图数据库)
- Time series (时间序列数据库)
- 其他常用的使用的封装
  - 布隆过滤器
  - 布谷鸟过滤器
  - 最小计数草图 (可以参考[这篇文档](https://zhuanlan.zhihu.com/p/369981005)介绍)
  - top-k
  - tdigest


具体可以查看[官网信息](https://redis.io/docs/stack/get-started/)

另外，redis 官方也做了很多教程啊！！！ 可以查看 [官方站点](https://university.redis.com/)
不得不说，redis-stack 把实验教程直接融入到 redis-insight 项目中，真的是太新手友好了！具体可以查看 [官方说明](https://redis.io/docs/stack/insight/)
