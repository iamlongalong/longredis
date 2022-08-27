package longredis

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	jsoniter "github.com/json-iterator/go"
	"github.com/stretchr/testify/assert"
)

var (
	RedisClusterOpt = Option{
		Addrs:    []string{"redis-cluster:6379"},
		Password: "",
	}

	RedisSingleOpt = Option{
		Addrs:     []string{"redis.default:6379"},
		Password:  "",
		RedisType: RedisTypeSingleton,
	}
)

func getClusterClient() *Client {
	c, err := NewClient(RedisClusterOpt)
	if err != nil {
		panic(err)
	}

	return c
}

func getSingleClient() *Client {
	c, err := NewClient(RedisSingleOpt)
	if err != nil {
		panic(err)
	}

	return c
}

func TestSingle(t *testing.T) {
	c := getSingleClient()
	runAllTest(c, t)

	c = c.SubPrefix("iredis_pre")
	runAllTest(c, t)
}

func TestCluster(t *testing.T) {
	c := getSingleClient()
	runAllTest(c, t)

	c = c.SubPrefix("iredis_pre")
	runAllTest(c, t)
}

func runAllTest(c *Client, t *testing.T) {

	stringTest(c, t)

	doTest(c, t)

	hashmapTest(c, t)

	listTest(c, t)

	setTest(c, t)

	zsetTest(c, t)
}

// doTest 测试 do
func doTest(c *Client, t *testing.T) {
	keyStr := tid()
	valStr := tid()

	_, err := c.Do(ctx, "set", keyStr, valStr)
	assert.Nil(t, err)

	v, err := c.Do(ctx, "get", keyStr)
	assert.Nil(t, err)
	assert.Equal(t, valStr, v.(string))

	_, err = c.Do(ctx, "expire", keyStr, time.Hour)
	assert.Nil(t, err)

}

// zsetTest 测试 zset
func zsetTest(c *Client, t *testing.T) {
	keyZset := tid()
	v1 := redis.Z{Score: 1, Member: "001"}
	v2 := redis.Z{Score: 2, Member: "002"}
	v3 := redis.Z{Score: 3, Member: "003"}
	v4 := redis.Z{Score: 4, Member: "004"}

	err := c.ZAdd(ctx, keyZset, v4.Score, v4.Member)
	assert.Nil(t, err)
	err = c.ZAdd(ctx, keyZset, v1.Score, v1.Member)
	assert.Nil(t, err)
	err = c.ZAddBatch(ctx, keyZset, []Members{v3, v2})
	assert.Nil(t, err)

	count, err := c.ZCount(ctx, keyZset, 0, 100)
	assert.Nil(t, err)
	assert.Equal(t, 4, int(count))

	count, err = c.ZCount(ctx, keyZset, 0, 3)
	assert.Nil(t, err)
	assert.Equal(t, 3, int(count))

	count, err = c.ZCount(ctx, keyZset, 1, 3)
	assert.Nil(t, err)
	assert.Equal(t, 3, int(count))

	ss, err := c.ZRange(ctx, keyZset, 1, 3)
	assert.Nil(t, err)
	assert.Equal(t, ss[0], v2.Member)
	assert.Equal(t, ss[1], v3.Member)
	assert.Equal(t, ss[2], v4.Member)

	sss, err := c.ZRangeWithScore(ctx, keyZset, 2, 3)
	assert.Nil(t, err)
	assert.Equal(t, v3.Member, sss[0].Member)
	assert.Equal(t, v4.Member, sss[1].Member)

	ss, err = c.ZRevRange(ctx, keyZset, 3, 5)
	assert.Nil(t, err)
	assert.Equal(t, v1.Member, ss[0])

	i, err := c.ZRemRangeByRank(ctx, keyZset, 2, 2)
	assert.Nil(t, err)
	assert.Equal(t, 1, int(i))

	i, err = c.ZCount(ctx, keyZset, 1, 3)
	assert.Nil(t, err)
	assert.Equal(t, 2, int(i))
}

// setTest 测试 set
func setTest(c *Client, t *testing.T) {
	keySet := tid()
	valSet1 := tid()
	valSet2 := tid()
	valSet3 := tid()

	err := c.SAdd(ctx, keySet, valSet1, valSet2, valSet2, valSet2)
	assert.Nil(t, err)

	n, err := c.SCard(ctx, keySet)
	assert.Nil(t, err)
	assert.Equal(t, 2, int(n))

	ok, err := c.SIsMember(ctx, keySet, valSet1)
	assert.Nil(t, err)
	assert.True(t, ok)

	ok, err = c.SIsMember(ctx, keySet, valSet3)
	assert.Nil(t, err)
	assert.False(t, ok)

	err = c.SRem(ctx, keySet, valSet1)
	assert.Nil(t, err)

	ok, err = c.SIsMember(ctx, keySet, valSet1)
	assert.Nil(t, err)
	assert.False(t, ok)

	err = c.SAdd(ctx, keySet, valSet3)
	assert.Nil(t, err)

	n, err = c.SCard(ctx, keySet)
	assert.Nil(t, err)
	assert.Equal(t, 2, int(n))

	m, err := c.SMembers(ctx, keySet)
	assert.Nil(t, err)

	assert.Equal(t, 2, len(m))
	assert.True(t, m[0] == valSet2 || m[0] == valSet3)
	assert.True(t, m[1] == valSet2 || m[1] == valSet3)
}

// hashmapTest 测试 hashmap
func hashmapTest(c *Client, t *testing.T) {
	keyHashMap := tid()
	fieldStr := "string"
	valStr := "hello world"

	fieldStr2 := "string2"
	valStr2 := "True"

	fieldInt := "int"
	valInt := 123

	fieldBool := "bool"
	valBool := true

	err := c.HSet(ctx, keyHashMap, fieldStr, valStr)
	assert.Nil(t, err)

	err = c.HMSet(ctx, keyHashMap, fieldInt, valInt, fieldStr2, valStr2)
	assert.Nil(t, err)

	ok, err := c.HSetNX(ctx, keyHashMap, fieldBool, valBool)
	assert.Nil(t, err)
	assert.True(t, ok)

	ok, err = c.HSetNX(ctx, keyHashMap, fieldStr, "xxxx")
	assert.Nil(t, err)
	assert.False(t, ok)

	b, err := c.HGet(ctx, keyHashMap, fieldStr)
	assert.Nil(t, err)
	assert.Equal(t, valStr, string(b))

	ss, err := c.HMGet(ctx, keyHashMap, fieldStr, fieldStr2, fieldBool)
	assert.Nil(t, err)
	assert.Equal(t, valStr, ss[0])
	assert.Equal(t, valStr2, ss[1])
	assert.Equal(t, "1", ss[2])

	ok, err = c.HGetBool(ctx, keyHashMap, fieldBool)
	assert.Nil(t, err)
	assert.True(t, ok)

	ok, err = c.HGetBool(ctx, keyHashMap, fieldStr2)
	assert.Nil(t, err)
	assert.True(t, ok)

	ok, err = c.HGetBool(ctx, keyHashMap, fieldStr)
	assert.Nil(t, err)
	assert.False(t, ok)

	s, err := c.HGetString(ctx, keyHashMap, fieldStr)
	assert.Nil(t, err)
	assert.Equal(t, valStr, s)

	m, err := c.HGetAll(ctx, keyHashMap)
	assert.Nil(t, err)
	assert.Equal(t, valStr, m[fieldStr])
	assert.Equal(t, valStr2, m[fieldStr2])
	assert.Equal(t, "1", m[fieldBool])

	i, err := c.HGetInt64(ctx, keyHashMap, fieldInt)
	assert.Nil(t, err)
	assert.Equal(t, valInt, int(i))

	err = c.HDel(ctx, keyHashMap, valStr2)
	assert.Nil(t, err)

	i, err = c.HIncrBy(ctx, keyHashMap, fieldInt, 10)
	assert.Nil(t, err)
	assert.Equal(t, valInt+10, int(i))

	fieldJson := "json"
	valJson := map[string]interface{}{
		"name":     "long",
		"age":      float64(18),
		"handsome": true,
	}
	v, err := jsoniter.MarshalToString(valJson)
	assert.Nil(t, err)

	err = c.HSet(ctx, keyHashMap, fieldJson, v)
	assert.Nil(t, err)

	mp := map[string]interface{}{}
	err = c.HGetBind(ctx, keyHashMap, fieldJson, &mp)
	assert.Nil(t, err)

	assert.True(t, mapEqual(mp, valJson))

}

// otherTest 测试 other
func otherTest(c *Client, t *testing.T) {
	keyRenameBefore := tid()
	keyRenameAfter := tid()
	valRename := tid()

	f, err := c.Exists(ctx, keyRenameBefore)
	assert.Nil(t, err)
	assert.False(t, f)

	f, err = c.Exists(ctx, keyRenameAfter)
	assert.Nil(t, err)
	assert.False(t, f)

	err = c.Set(ctx, keyRenameBefore, valRename, time.Hour)
	assert.Nil(t, err)

	f, err = c.Exists(ctx, keyRenameBefore)
	assert.Nil(t, err)
	assert.True(t, f)

	err = c.Rename(ctx, keyRenameBefore, keyRenameAfter)
	assert.Nil(t, err)

	f, err = c.Exists(ctx, keyRenameAfter)
	assert.Nil(t, err)
	assert.True(t, f)

	f, err = c.Exists(ctx, keyRenameBefore)
	assert.Nil(t, err)
	assert.False(t, f)

	s, err := c.GetString(ctx, keyRenameAfter)
	assert.Nil(t, err)
	assert.Equal(t, valRename, s)

	keyRNxBefore := tid()
	keyRNxAfter := tid()

	ok, err := c.RenameNX(ctx, keyRNxBefore, keyRNxAfter)
	assert.False(t, ok)
	assert.Nil(t, err)

	err = c.Set(ctx, keyRNxAfter, 1, time.Hour)
	assert.Nil(t, err)

	ok, err = c.RenameNX(ctx, keyRNxBefore, keyRNxAfter)
	assert.False(t, ok)
	assert.Nil(t, err)

	err = c.Set(ctx, keyRNxBefore, 2, time.Hour)
	assert.Nil(t, err)

	ok, err = c.RenameNX(ctx, keyRNxBefore, keyRNxAfter)
	assert.False(t, ok)
	assert.Nil(t, err)

	err = c.Del(ctx, keyRNxAfter)
	assert.Nil(t, err)

	ok, err = c.RenameNX(ctx, keyRNxBefore, keyRNxAfter)
	assert.True(t, ok)
	assert.Nil(t, err)

	ok, err = c.Exists(ctx, keyRNxBefore)
	assert.Nil(t, err)
	assert.False(t, ok)

	// type test

	keyTypeString := tid()
	keyTypeList := tid()
	keyTypeHashMap := tid()

	err = c.Set(ctx, keyTypeString, "123", time.Hour)
	assert.Nil(t, err)

	err = c.RPush(ctx, keyTypeList, "123")
	assert.Nil(t, err)

	err = c.HSet(ctx, keyTypeHashMap, "123", "123")
	assert.Nil(t, err)

	ty, err := c.Type(ctx, keyTypeString)
	assert.Nil(t, err)
	assert.Equal(t, "string", ty)

	ty, err = c.Type(ctx, keyTypeList)
	assert.Nil(t, err)
	assert.Equal(t, "list", ty)

	ty, err = c.Type(ctx, keyTypeHashMap)
	assert.Nil(t, err)
	assert.Equal(t, "hash", ty)

	d, err := c.TTL(ctx, keyTypeString)
	assert.Nil(t, err)
	assert.True(t, d > time.Hour-time.Second*10)

	// ping test
	err = c.Ping(ctx)
	assert.Nil(t, err)

	stats := c.RedisStatus()
	assert.True(t, stats.Hits > 2)
	assert.True(t, stats.IdleConns > 1)
	assert.True(t, stats.Timeouts == 0)

}

// listTest 测试 list
func listTest(c *Client, t *testing.T) {
	keyList := tid()
	valList1 := tid()
	valList2 := tid()
	valList3 := tid()

	err := c.RPush(ctx, keyList, valList1, valList2)
	assert.Nil(t, err)

	s, err := c.LPop(ctx, keyList)
	assert.Nil(t, err)
	assert.Equal(t, valList1, s)

	err = c.RPush(ctx, keyList, valList3)
	assert.Nil(t, err)

	s, err = c.LIndex(ctx, keyList, 0)
	assert.Nil(t, err)
	assert.Equal(t, valList2, s)

	i, err := c.LLen(ctx, keyList)
	assert.Nil(t, err)
	assert.Equal(t, 2, int(i))
}

// setTest 测试 pipline
func pipeTest(c *Client, t *testing.T) {
	pipe := c.TxPipeline(ctx)

	keyPipeStr := tid()
	valPipeStr := tid()

	keyPipeHmap := tid()
	fieldPipeHmap := tid()
	valPipeHmap := tid()

	_ = pipe.Set(ctx, keyPipeStr, valPipeStr, time.Hour)
	existsCmd := pipe.Exists(ctx, keyPipeStr)
	_ = pipe.HSet(ctx, keyPipeHmap, fieldPipeHmap, valPipeHmap)
	hgetCmd := pipe.HGet(ctx, keyPipeHmap, fieldPipeHmap)

	cmds, err := pipe.Exec(ctx)
	assert.Nil(t, err)

	assert.Equal(t, 4, len(cmds))

	for _, cmd := range cmds {
		assert.Nil(t, cmd.Err())
	}

	assert.Equal(t, 1, int(existsCmd.Val()))
	assert.Equal(t, valPipeHmap, hgetCmd.Val())
}

func stringTest(c *Client, t *testing.T) {
	keyGetSet := tid()
	valGetSet := tid()

	b, err := c.Get(ctx, keyGetSet)
	assert.Nil(t, b)
	assert.ErrorIs(t, err, Nil)

	err = c.Set(ctx, keyGetSet, valGetSet, time.Hour)
	assert.Nil(t, err)

	b, err = c.Get(ctx, keyGetSet)
	assert.Nil(t, err)
	assert.Equal(t, valGetSet, string(b))

	s, err := c.GetString(ctx, keyGetSet)
	assert.Nil(t, err)
	assert.Equal(t, valGetSet, s)

	keySetEx := tid()
	valSetEx := tid()

	err = c.SetEx(ctx, keySetEx, valSetEx, time.Second)
	assert.Nil(t, err)

	s, err = c.GetString(ctx, keySetEx)
	assert.Nil(t, err)
	assert.Equal(t, valSetEx, s)

	time.Sleep(time.Second)

	s, err = c.GetString(ctx, keySetEx)
	assert.Equal(t, "", s)
	assert.ErrorIs(t, err, Nil)

	keyGetInt := tid()
	err = c.Set(ctx, keyGetInt, 10001, time.Hour)
	assert.Nil(t, err)

	i, err := c.GetUInt64(ctx, keyGetInt)
	assert.Nil(t, err)
	assert.Equal(t, 10001, int(i))

	keyGetBind := tid()
	v := map[string]interface{}{
		"hello":    "world",
		"areyouok": true,
		"today":    float64(6),
		"boom":     nil,
		"sub": map[string]interface{}{
			"name": "long",
			"ok":   "yes",
		},
	}
	s, err = jsoniter.MarshalToString(v)
	assert.Nil(t, err)

	err = c.Set(ctx, keyGetBind, s, time.Hour)
	assert.Nil(t, err)

	tv := map[string]interface{}{}
	err = c.GetBind(ctx, keyGetBind, &tv)
	assert.Nil(t, err)
	assert.True(t, mapEqual(v, tv))

	type st struct {
		Name    string
		Age     int
		Married bool
	}

	sv := st{Name: "hello world", Age: 25, Married: true}
	s, err = jsoniter.MarshalToString(sv)
	assert.Nil(t, err)

	err = c.Set(ctx, keyGetBind, s, time.Hour)
	assert.Nil(t, err)

	tsv := st{}
	err = c.GetBind(ctx, keyGetBind, &tsv)
	assert.Nil(t, err)
	assert.True(t, reflect.DeepEqual(sv, tsv))

	keySetNx := tid()
	ok, err := c.SetNx(ctx, keySetNx, "hello", time.Second)
	assert.Nil(t, err)
	assert.True(t, ok)

	s, err = c.GetString(ctx, keySetNx)
	assert.Nil(t, err)
	assert.Equal(t, "hello", s)

	ok, err = c.SetNx(ctx, keySetNx, 1)
	assert.Nil(t, err)
	assert.False(t, ok)

	time.Sleep(time.Second)
	ok, err = c.SetNx(ctx, keySetNx, "world", time.Second*10)
	assert.Nil(t, err)
	assert.True(t, ok)

	s, err = c.GetString(ctx, keySetNx)
	assert.Nil(t, err)
	assert.Equal(t, "world", s)
}

func tid() string {
	return fmt.Sprintf("iredis_test:%s", genID(8))
}

func mapEqual(m1 map[string]interface{}, m2 map[string]interface{}) bool {
	return reflect.DeepEqual(m1, m2)
}

func TestSmember(t *testing.T) {
	c := getClusterClient()

	keySmem := tid()

	mem := []string{"long001", "long002", "long003", "long004"}
	err := c.SAdd(ctx, keySmem, mem)
	assert.Nil(t, err)

	ok, err := c.SIsMember(ctx, keySmem, mem[0])
	assert.Nil(t, err)
	assert.True(t, ok)
	fmt.Println("key1 : ", keySmem)

	keySmem2 := tid()
	err = c.SAdd(ctx, keySmem2, "long001", "long002", "long003")
	assert.Nil(t, err)

	fmt.Println("key2 : ", keySmem2)
	ok, err = c.SIsMember(ctx, keySmem, "long001")
	assert.Nil(t, err)
	assert.True(t, ok)

}
