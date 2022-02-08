package logging

import (
	"sync"
)

type node struct {
	tid       int64
	offsetABs []offsetAB // 解析的时候不排序 展示的时候排序。 查询的频率也不高；入队列的时候直接是有序的
	prev      *node      // 判断第一个节点 nil
	next      *node      // 判断最后一个节点 nil
}
type offsetAB struct {
	A int64
	B int64
}

// logList  双端链表 并发安全
type logList struct {
	mux    sync.RWMutex // mux protect the followings
	first  *node
	last   *node
	length int
	cap    int //  cap 容量 小于=0 则不限制容量
}

// newLogList 新建一个 logList . cap 限定 _logList 的容量. 小于等于0则不限制
func newLogList(cap int) *logList {
	return &logList{mux: sync.RWMutex{}, cap: cap}
}

func (d *logList) Insert(tid int64, ab offsetAB) {
	if element, ok := d.Find(tid); ok {
		element.offsetABs = append(element.offsetABs, ab)
		return
	}
	d.mux.Lock()
	defer d.mux.Unlock()
	defer func() {
		if d.cap > 0 && d.length > d.cap {
			if first := d.first; first != nil {
				next := first.next
				if next != nil {
					first = nil
					next.prev = nil
					d.first = next
					d.length--
				}
			}
		}
	}()
	n := node{
		tid:       tid,
		offsetABs: []offsetAB{ab},
		prev:      nil,
		next:      nil,
	}
	// 没有节点 等同于 d.Length==0
	if d.length == 0 {
		d.first = &n
		d.last = &n
		d.length++
		return
	}
	switch d.length {
	case 0:
		d.first = &n
		d.last = &n
		d.length++
		return
	case 1:
		switch {
		case tid > d.last.tid:
			d.last.next = &n
			n.prev = d.last
			d.last = &n
			d.length++
			return
		case tid < d.last.tid:
			d.last.prev = &n
			n.next = d.last
			d.first = &n
			d.length++
			return
		}
	}
	if tid > d.last.tid {
		d.last.next = &n
		n.prev = d.last
		d.last = &n
		d.length++
		return
	}
	if tid < d.first.tid {
		d.first.prev = &n
		n.next = d.first
		d.first = &n
		d.length++
		return
	}
	preNode := d.last.prev
	for tid < preNode.tid {
		preNode = preNode.prev
	}
	next := preNode.next
	n.prev = preNode
	n.next = next
	preNode.next = &n
	next.prev = &n
	d.length++
}
func (d *logList) Find(tid int64) (*node, bool) {
	d.mux.RLock()
	defer d.mux.RUnlock()
	element := d.last
	if element == nil {
		return nil, false
	}
	for element != nil {
		if element.tid == tid {
			return element, true
		}
		element = element.prev
	}
	return nil, false
}

func (n *node) OffsetABs() []offsetAB {
	if n == nil {
		return nil
	}
	return n.offsetABs
}
