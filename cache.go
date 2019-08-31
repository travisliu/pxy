package pxy

import (
    "time"
    "sync"
    "container/list"
)

const Deleting  = "deleting"
const Promoting = "promoting"

type LruOperation struct {
    action string
    item   *CacheItem
}

type Cache struct {
    sync.RWMutex
    list        *list.List
    size        uint64
    maxSize     uint64
    sizeToPrung uint64
    items       map[string]*CacheItem
    lruables    chan *LruOperation
}

type CacheItem struct {
    key        string
    Data       []byte
    Expiration int64
    element    *list.Element
    deleted    bool
}

func NewCache(maxSize, sizeToPrung uint64) *Cache {
    cache := &Cache {
        list:        list.New(),
        size:        0,
        sizeToPrung: sizeToPrung,
        maxSize:     maxSize,
        items:       make(map[string]*CacheItem),
        lruables:    make(chan *LruOperation, 10),
    }

    go cache.worker()
    return cache
}

func (cache *Cache) Delete(key string) *CacheItem {
    cache.Lock()
    item := cache.items[key]
    delete(cache.items, key)
    cache.Unlock()
    if (item != nil) {
        cache.deleteable(item)
    }

    return item
}

func (cache *Cache) Get(key string) (cacheItem *CacheItem, ok bool) {
    cache.RLock()
    cacheItem = cache.items[key]
    cache.RUnlock()

    if cacheItem == nil {
        ok = false
        cacheItem = nil
        return
    }

    if time.Now().Unix() > cacheItem.Expiration {
      ok = false
      cacheItem.Data = nil
      return
    }

    cache.promotable(cacheItem)
    ok = true
    return
}

func (cache *Cache) deleteable(item *CacheItem) {
    cache.lruables <- &LruOperation {
      action: Deleting,
      item:      item,
    }
}

func (cache *Cache) promotable(item *CacheItem) {
    cache.lruables <- &LruOperation {
      action: Promoting,
      item:      item,
    }
}

func (cache *Cache) Set(key string, cacheItem *CacheItem) {
    cache.Lock()
    existing := cache.items[key]
    cache.items[key] = cacheItem
    cache.Unlock()

    if (existing != nil) {
      cache.deleteable(existing)
    }
    cache.promotable(cacheItem)
}

func (cache *Cache) promoteItem(cacheItem *CacheItem) bool {
    cache.size += uint64(len(cacheItem.Data))

    if (cacheItem.element != nil) {
      cache.list.MoveToFront(cacheItem.element)
      return true
    }

    cacheItem.element = cache.list.PushFront(cacheItem)
    return true
}

func (cache *Cache) removeFromList(item *CacheItem) {
   cache.size -= uint64(len(item.Data))
   cache.list.Remove(item.element)
}

func (cache *Cache) worker() {
    for {
        operation := <-cache.lruables
        item := operation.item
        switch operation.action {
        case Deleting:
            cache.removeFromList(item)
        case Promoting:
            if cache.promoteItem(item) && cache.size > cache.maxSize {
                cache.gc()
            }
        }
    }
}

func (cache *Cache) gc() {
    currentSize := cache.size
    aimToPrune := cache.maxSize - cache.sizeToPrung
    element := cache.list.Back()
    for currentSize > aimToPrune {
        item := element.Value.(*CacheItem)
        prevElement := element.Prev()

        cache.Lock()
        delete(cache.items, item.key)
        cache.Unlock()
        cache.removeFromList(item)

        element = prevElement
        currentSize -= uint64(len(item.Data))
    }
}
