package redis

import (
	redisgo "github.com/gomodule/redigo/redis"
)

type StreamValue struct {
	Field string
	Value string
}

type StreamData struct {
	Id string `json:"id"`
	StreamValue
}

/**
    写入命令
	key=>redis key
	id=> 使用唯一id或者输入*自增
	maxLen streams的最大长度，0为不限制 到达最大限制会删除老数据 ~代表不需要那么精致到最大长度，但是一定不能小于给定长度
*/
func (s *Client) XAdd(key, id string, maxLen uint64, value ...StreamValue) error {
	command := redisgo.Args{}.Add(key)
	if maxLen > 0 {
		command = command.AddFlat("MAXLEN").AddFlat("~").AddFlat(maxLen)
	}
	command = command.AddFlat(id)
	for i := range value[:] {
		command = command.AddFlat(value[i].Field)
		command = command.AddFlat(value[i].Value)
	}

	if _, err := s.pool.Do("XADD", command...); err != nil {
		return err
	}
	return nil
}

/**
XGROUP 用于创建，销毁和管理消费者组。
XGROUP CREATE mystream mygroup $ 创建
XGROUP DESTROY mystream consumer-group-name 完全销毁消费者
XGROUP DELCONSUMER mystream consumer-group-name myconsumer123 消费者从消费者组中删除
action=> 操作
streams=> streams的名称
name=> group的名称
id=> id

在创建消费者组时，我们必须指定一个ID，在示例中是$。
这是必需的，因为消费者组在其他状态中必须知道在连接后处理哪些消息，即刚刚创建该组时的最后消息ID是什么？
如果按照我们提供的$，那么只有从现在开始到达Stream的新消息才会提供给该组中的消费者。
如果我们指定0,消费者组将消费所有Stream历史中的消息记录。
当然，您可以指定任何其他有效ID。您所知道的是，消费者组将开始消费ID大于您指定的ID的消息。
因为$表示Stream中当前最大的ID，所以指定$将仅消费新消息。
*/
func (s *Client) XGroup(action, streams, name, id string) error {
	if _, err := s.pool.Do("XGROUP", action, streams, name, id); err != nil {
		return err
	}
	return nil
}

/**
group=>组名称
consumer=>消费者
id=> id 传`>`表示这个消费者只接收从来没有被投递给其他消费者的消息，即新的消息。当然我们也可以指定具体的ID，例如指定0表示访问所有投递给该消费者的历史消息，指定1540081890919-1表示投递给该消费者且大于这个ID的历史消息
key=>可以同时接受多个streams
count=>条数
block=>超时时间
*/
func (s *Client) XReadGroupAck(group, consumer, id string, key []string, count, block int64) (map[string][]map[string]string, error) {
	data := make(map[string][]map[string]string)
	command := redisgo.Args{}.Add("GROUP").AddFlat(group)
	command = command.AddFlat(consumer)
	command = command.AddFlat("COUNT").AddFlat(count)
	command = command.AddFlat("BLOCK").AddFlat(block)
	command = command.AddFlat("STREAMS")
	for i := range key[:] {
		command = command.AddFlat(key[i])
	}
	command = command.AddFlat(id)
	d, err := redisgo.Values(s.pool.Do("XREADGROUP", command...))
	if err != nil {
		return data, err
	}
	if len(d) == 0 {
		return data, nil
	}

	if err := s.pool.Send("MULTI"); err != nil {
		return data, err
	}
	for i := range d[:] {
		keyGroup, err := redisgo.Values(d[i], nil)
		if err != nil {
			return data, err
		}
		k, err := redisgo.String(keyGroup[0], nil)
		if err != nil {
			return data, err
		}
		keyValues, err := redisgo.Values(keyGroup[1], nil)
		if err != nil {
			return data, err
		}
		for v := range keyValues[:] {
			idLevel, err := redisgo.Values(keyValues[v], nil)
			if err != nil {
				return data, err
			}
			valueId, err := redisgo.String(idLevel[0], nil)
			if err != nil {
				return data, err
			}
			valueDatas, err := redisgo.Values(idLevel[1], nil)
			if err != nil {
				return data, err
			}
			streamData := make(map[string]string)
			streamData["id"] = valueId

			if err := s.pool.Send("XACK", k, group, valueId); err != nil {
				return data, err
			}
			for j := range valueDatas[:] {
				if j&1 == 1 {
					continue
				}
				field, err := redisgo.String(valueDatas[j], nil)
				if err != nil {
					return data, err
				}
				fValue, err := redisgo.String(valueDatas[j+1], nil)
				if err != nil {
					return data, err
				}
				streamData[field] = fValue
			}
			data[k] = append(data[k], streamData)
		}

	}

	if _, err := s.pool.Do("EXEC"); err != nil {
		return data, err
	}
	return data, nil
}
func (s *Client) XReadConversionOne(values map[string][]map[string]string, key string) map[string]string {
	return values[key][0]
}
