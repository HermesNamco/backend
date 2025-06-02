package pool

import (
	"context"
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

type Connection struct {
	net.Conn
	lastUsed time.Time
	inUse    int32
}

type ConnectionPool struct {
	factory    func() (net.Conn, error)
	idle       chan *Connection
	active     map[*Connection]struct{}
	mu         sync.RWMutex
	maxIdle    int
	maxActive  int
	idleTime   time.Duration
	activeConn int32
}

func NewConnectionPool(factory func() (net.Conn, error), maxIdle, maxActive int, idleTime time.Duration) *ConnectionPool {
	pool := &ConnectionPool{
		factory:   factory,
		idle:      make(chan *Connection, maxIdle),
		active:    make(map[*Connection]struct{}),
		maxIdle:   maxIdle,
		maxActive: maxActive,
		idleTime:  idleTime,
	}

	// 启动清理协程
	go pool.cleaner()
	return pool
}

func (p *ConnectionPool) Get(ctx context.Context) (*Connection, error) {
	// 尝试从空闲连接获取
	select {
	case conn := <-p.idle:
		if atomic.CompareAndSwapInt32(&conn.inUse, 0, 1) {
			return conn, nil
		}
	default:
	}

	// 检查活跃连接数限制
	if atomic.LoadInt32(&p.activeConn) >= int32(p.maxActive) {
		return nil, errors.New("connection pool exhausted")
	}

	// 创建新连接
	rawConn, err := p.factory()
	if err != nil {
		return nil, err
	}

	conn := &Connection{
		Conn:     rawConn,
		lastUsed: time.Now(),
		inUse:    1,
	}

	p.mu.Lock()
	p.active[conn] = struct{}{}
	p.mu.Unlock()

	atomic.AddInt32(&p.activeConn, 1)
	return conn, nil
}

func (p *ConnectionPool) Put(conn *Connection) {
	if !atomic.CompareAndSwapInt32(&conn.inUse, 1, 0) {
		return
	}

	conn.lastUsed = time.Now()

	select {
	case p.idle <- conn:
	default:
		// 空闲队列满，关闭连接
		p.closeConnection(conn)
	}
}

func (p *ConnectionPool) closeConnection(conn *Connection) {
	conn.Close()

	p.mu.Lock()
	delete(p.active, conn)
	p.mu.Unlock()

	atomic.AddInt32(&p.activeConn, -1)
}

func (p *ConnectionPool) cleaner() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		p.cleanIdleConnections()
	}
}

func (p *ConnectionPool) cleanIdleConnections() {
	now := time.Now()

	for {
		select {
		case conn := <-p.idle:
			if now.Sub(conn.lastUsed) > p.idleTime {
				p.closeConnection(conn)
			} else {
				// 放回队列
				select {
				case p.idle <- conn:
				default:
					p.closeConnection(conn)
				}
				return
			}
		default:
			return
		}
	}
}
