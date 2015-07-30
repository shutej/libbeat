package redis

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/elastic/libbeat/common"
	"github.com/elastic/libbeat/logp"
	"github.com/elastic/libbeat/outputs"

	"github.com/garyburd/redigo/redis"
)

type RedisDataType uint16

const (
	RedisListType RedisDataType = iota
	RedisChannelType
)

type RedisOutput struct {
	Index string
	Conn  redis.Conn

	TopologyExpire     time.Duration
	ReconnectInterval  time.Duration
	Hostname           string
	Password           string
	Db                 int
	DbTopology         int
	Timeout            time.Duration
	DataType           RedisDataType
	FlushInterval      time.Duration
	flush_immediatelly bool

	TopologyMap  map[string]string
	sendingQueue chan RedisQueueMsg
	connected    bool
}

type RedisQueueMsg struct {
	index string
	msg   string
}

func (out *RedisOutput) Init(config outputs.Config, topology_expire int) error {

	out.Hostname = fmt.Sprintf("%s:%d", config.Host, config.Port)

	if config.Password != "" {
		out.Password = config.Password
	}

	if config.Db != 0 {
		out.Db = config.Db
	}

	out.DbTopology = 1
	if config.DbTopology != 0 {
		out.DbTopology = config.DbTopology
	}

	out.Timeout = 5 * time.Second
	if config.Timeout != 0 {
		out.Timeout = time.Duration(config.Timeout) * time.Second
	}

	if config.Index != "" {
		out.Index = config.Index
	} else {
		out.Index = "packetbeat"
	}

	out.FlushInterval = 1000 * time.Millisecond
	if config.FlushInterval != nil {
		if *config.FlushInterval < 0 {
			out.flush_immediatelly = true
			logp.Warn("Flushing to REDIS on each push, performance migh be affected")
		} else {
			out.FlushInterval = time.Duration(*config.FlushInterval) * time.Millisecond
		}
	}

	out.ReconnectInterval = time.Duration(1) * time.Second
	if config.ReconnectInterval != 0 {
		out.ReconnectInterval = time.Duration(config.ReconnectInterval) * time.Second
	}

	exp_sec := 15
	if topology_expire != 0 {
		exp_sec = topology_expire
	}
	out.TopologyExpire = time.Duration(exp_sec) * time.Second

	switch config.DataType {
	case "", "list":
		out.DataType = RedisListType
	case "channel":
		out.DataType = RedisChannelType
	default:
		return errors.New("Bad Redis data type")
	}

	logp.Info("[RedisOutput] Using Redis server %s", out.Hostname)
	if out.Password != "" {
		logp.Info("[RedisOutput] Using password to connect to Redis")
	}
	logp.Info("[RedisOutput] Redis connection timeout %s", out.Timeout)
	logp.Info("[RedisOutput] Redis reconnect interval %s", out.ReconnectInterval)
	logp.Info("[RedisOutput] Redis flushing interval %s", out.FlushInterval)
	logp.Info("[RedisOutput] Using index pattern %s", out.Index)
	logp.Info("[RedisOutput] Topology expires after %s", out.TopologyExpire)
	logp.Info("[RedisOutput] Using db %d for storing events", out.Db)
	logp.Info("[RedisOutput] Using db %d for storing topology", out.DbTopology)
	logp.Info("[RedisOutput] Using %d data type", out.DataType)

	out.sendingQueue = make(chan RedisQueueMsg, 1000)

	out.Reconnect()
	go out.SendMessagesGoroutine()

	return nil
}

func (out *RedisOutput) RedisConnect(db int) (redis.Conn, error) {
	conn, err := redis.DialTimeout(
		"tcp",
		out.Hostname,
		out.Timeout, out.Timeout, out.Timeout)
	if err != nil {
		return nil, err
	}

	if len(out.Password) > 0 {
		_, err = conn.Do("AUTH", out.Password)
		if err != nil {
			return nil, err
		}
	}

	_, err = conn.Do("PING")
	if err != nil {
		return nil, err
	}

	_, err = conn.Do("SELECT", db)
	if err != nil {
		return nil, err
	}

	return conn, nil
}

func (out *RedisOutput) Connect() error {
	var err error
	out.Conn, err = out.RedisConnect(out.Db)
	if err != nil {
		return err
	}
	out.connected = true

	return nil
}

func (out *RedisOutput) Close() {
	out.Conn.Close()
}

func (out *RedisOutput) SendMessagesGoroutine() {

	var err error
	var pending int
	flushChannel := make(<-chan time.Time)

	if !out.flush_immediatelly {
		flushTicker := time.NewTicker(out.FlushInterval)
		flushChannel = flushTicker.C
	}

	for {
		select {
		case queueMsg := <-out.sendingQueue:

			if !out.connected {
				logp.Debug("output_redis", "Droping pkt ...")
				continue
			}
			logp.Debug("output_redis", "Send event to redis")
			command := "RPUSH"
			if out.DataType == RedisChannelType {
				command = "PUBLISH"
			}

			if !out.flush_immediatelly {
				err = out.Conn.Send(command, queueMsg.index, queueMsg.msg)
				pending += 1
			} else {
				_, err = out.Conn.Do(command, queueMsg.index, queueMsg.msg)
			}
			if err != nil {
				logp.Err("Fail to publish event to REDIS: %s", err)
				out.connected = false
				go out.Reconnect()
			}
		case _ = <-flushChannel:
			if pending > 0 {
				out.Conn.Flush()
				_, err = out.Conn.Receive()
				if err != nil {
					logp.Err("Fail to publish event to REDIS: %s", err)
					out.connected = false
					go out.Reconnect()
				}
				logp.Debug("output_redis", "Flushed %d pending commands", pending)
				pending = 0
			}
		}
	}
}

func (out *RedisOutput) Reconnect() {

	for {
		err := out.Connect()
		if err != nil {
			logp.Warn("Error connecting to Redis (%s). Retrying in %s", err, out.ReconnectInterval)
			time.Sleep(out.ReconnectInterval)
		} else {
			break
		}
	}
}

func (out *RedisOutput) GetNameByIP(ip string) string {
	name, exists := out.TopologyMap[ip]
	if !exists {
		return ""
	}
	return name
}

func (out *RedisOutput) PublishIPs(name string, localAddrs []string) error {

	logp.Debug("output_redis", "[%s] Publish the IPs %s", name, localAddrs)

	// connect to db
	conn, err := out.RedisConnect(out.DbTopology)
	if err != nil {
		return err
	}
	defer conn.Close()

	_, err = conn.Do("HSET", name, "ipaddrs", strings.Join(localAddrs, ","))
	if err != nil {
		logp.Err("[%s] Fail to set the IP addresses: %s", name, err)
		return err
	}

	_, err = conn.Do("EXPIRE", name, int(out.TopologyExpire.Seconds()))
	if err != nil {
		logp.Err("[%s] Fail to set the expiration time: %s", name, err)
		return err
	}

	out.UpdateLocalTopologyMap(conn)

	return nil
}

func (out *RedisOutput) UpdateLocalTopologyMap(conn redis.Conn) {

	TopologyMapTmp := make(map[string]string)

	hostnames, err := redis.Strings(conn.Do("KEYS", "*"))
	if err != nil {
		logp.Err("Fail to get the all shippers from the topology map %s", err)
		return
	}
	for _, hostname := range hostnames {
		res, err := redis.String(conn.Do("HGET", hostname, "ipaddrs"))
		if err != nil {
			logp.Err("[%s] Fail to get the IPs: %s", hostname, err)
		} else {
			ipaddrs := strings.Split(res, ",")
			for _, addr := range ipaddrs {
				TopologyMapTmp[addr] = hostname
			}
		}
	}

	out.TopologyMap = TopologyMapTmp

	logp.Debug("output_redis", "Topology %s", TopologyMapTmp)
}

func (out *RedisOutput) PublishEvent(ts time.Time, event common.MapStr) error {

	json_event, err := json.Marshal(event)
	if err != nil {
		logp.Err("Fail to convert the event to JSON: %s", err)
		return err
	}

	out.sendingQueue <- RedisQueueMsg{index: out.Index, msg: string(json_event)}

	logp.Debug("output_redis", "Publish event")
	return nil
}
