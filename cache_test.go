package pxy

import (
    "testing"
    "time"
    "strconv"
)

func TestCache(t *testing.T) {
    data := []byte("abcdefeg")

    key := "stringkey"
    cache     := NewCache(1024, 10)
    cacheItem := &CacheItem{
        key:  key,
        Data: data,
        Expiration: time.Now().Unix() + 60,
        deleted: false,
    }

    cache.Set(key, cacheItem)

    item, ok := cache.Get(key)
    if !ok || string(data) != string(item.Data) {
        t.Errorf("get item %s", string(data))
    }
}

func TestPromoteCache(t *testing.T) {
    data      := []byte("abcdefeg")
    cache     := NewCache(1024, 10)
    cacheItem := &CacheItem{
        key: "key",
        Data: data,
        Expiration: time.Now().Unix() + 60,
        deleted: false,
    }

    cache.promoteItem(cacheItem)

    if (uint64(len(data)) != cache.size) {
        t.Errorf("promoted wrong size: %d", cache.size)
    }
}

func TestGC(t *testing.T) {
    cache     := NewCache(170, 20)
    for i := 0; i < 20; i++ {
        key       := "key:" + strconv.Itoa(i)
        data      := []byte("abcdefeg" + strconv.Itoa(i))
        cacheItem := &CacheItem{
            key:  key,
            Data: data,
            Expiration: time.Now().Unix() + 60,
            deleted: false,
        }

        cache.Set(key, cacheItem)
    }
    time.Sleep(time.Duration(10) * time.Millisecond)

    _, ok := cache.Get("key:1")
    if ok {
        t.Errorf("key:1 should be pruned")
    }
}

func TestDelete(t *testing.T) {
    key       := "delete:test"
    data      := []byte("abcdefeg")

    cache     := NewCache(170, 0)
    cacheItem := &CacheItem{
        key:  key,
        Data: data,
        Expiration: time.Now().Unix() + 60,
        deleted: false,
    }

    cache.Set(key, cacheItem)
    cache.Delete(key)
    item, ok := cache.Get(key)
    if ok || item != nil{
        t.Error("Item should be deleted.")
    }

    time.Sleep(time.Duration(10) * time.Millisecond)
    if cache.size > 0 {
        t.Error("Cache list should be empty after delete.")
    }
}
