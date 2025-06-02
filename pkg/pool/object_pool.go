package pool

import (
	"bytes"
	"sync"
)

// 高性能字节缓冲池
var bufferPool = sync.Pool{
	New: func() interface{} {
		return &bytes.Buffer{}
	},
}

// 获取缓冲区
func GetBuffer() *bytes.Buffer {
	return bufferPool.Get().(*bytes.Buffer)
}

// 回收缓冲区
func PutBuffer(buf *bytes.Buffer) {
	buf.Reset() // 清空内容但保留底层数组
	bufferPool.Put(buf)
}

// 高性能JSON处理器
type JSONProcessor struct {
	pool sync.Pool
}

func NewJSONProcessor() *JSONProcessor {
	return &JSONProcessor{
		pool: sync.Pool{
			New: func() interface{} {
				return make([]byte, 0, 1024) // 预分配1KB
			},
		},
	}
}

func (jp *JSONProcessor) Process(data interface{}) []byte {
	buf := jp.pool.Get().([]byte)
	defer jp.pool.Put(buf[:0]) // 重置长度但保留容量

	// JSON序列化逻辑
	return buf
}
