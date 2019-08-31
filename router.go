package pxy

import (
    "os"
    "io"
    "log"
    "math/rand"
    "time"
    "bytes"
    "strings"
    "bufio"
    "net/http"
    "net/http/httputil"
    "io/ioutil"
    "encoding/json"
)

type Resource struct {
    Name         string `json:"name"`
    TTL          int64  `json:"ttl"`
    Resources   []Resource
}

type Auth struct {
    User     string `json:user`
    Password string `json:password`
}

type Config struct {
    LogPath      string     `json:"log_path"`
    TargetScheme string     `json:"target_scheme"`
    TargetHost   string     `json:"target_host"`
    MaxMemory    uint16     `json:"max_memory"`
    SizeToPrung  uint16     `json:"size_to_prung"`
    TTL          int64      `json:"ttl"`
    Resources    []Resource `json:"resources"`
    Auth         Auth       `json:"auth"`
}

func (config *Config) load(path string) {
    jsonFile, jsonFileError := os.Open(path)
    defer jsonFile.Close()

    if jsonFileError != nil {
        return
    }

    byteValue, byteValueError := ioutil.ReadAll(jsonFile)
    if byteValueError != nil {
        return
    }

    json.Unmarshal(byteValue, config)
}


type Node struct {
    Name        string
    Children    map[string]*Node
    Config      *NodeConfig
}

type NodeConfig struct {
    TargetScheme string
    TargetHost   string
    TTL          int64
    Auth         Auth
}

type Router struct {
    Logger        *log.Logger
    LogFile       *os.File
    Proxy         *httputil.ReverseProxy
    Cache         *Cache
}

func InitCache(config Config) *Cache{
    megabyte := uint64(1024 * 1024)

    maxMemory := uint64(config.MaxMemory) * megabyte
    if maxMemory == 0 {
        maxMemory = 128 * megabyte
    }

    sizeToPrung := uint64(config.SizeToPrung)
    if sizeToPrung == 0 {
        sizeToPrung = megabyte
    }

    return NewCache(maxMemory, sizeToPrung)
}

func Initialize(configPath string) *Router {
    config := Config{}
    config.load(configPath)

    file, err := os.OpenFile(config.LogPath,
        os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
    if err != nil {
        panic("Log file not accessible.")
    }
    logger := log.New(file, "prefix", log.LstdFlags)

    nodeConfig := &NodeConfig{
        TTL:          config.TTL,
        TargetScheme: config.TargetScheme,
        TargetHost:   config.TargetHost,
        Auth:         config.Auth,
    }

    cache := InitCache(config)
    transport := &CacheTransport {
        Cache: cache,
        DefaultConfig: nodeConfig,
        Logger: logger,
    }

    tree := map[string]*Node{}
    for _, resource:= range config.Resources {
        transport.addNode(resource, tree, nodeConfig)
    }

    newRouter := &Router{
        LogFile: file,
        Logger: logger,
        Proxy: &httputil.ReverseProxy {
            Transport: transport,
            Director: func(request *http.Request) {},
        },
        Cache: cache,
    }

    return newRouter
}

type CacheTransport struct {
    Cache         *Cache
    DefaultConfig *NodeConfig
    Logger        *log.Logger
    Tree          map[string]*Node
}

func(transport *CacheTransport) Lookup(path string) *NodeConfig {
    pathNodes := strings.Split(path[1:], "/")

    config := transport.DefaultConfig
    tree   := transport.Tree
    for _, pathNode := range pathNodes {
        node := tree[pathNode]
        if node == nil {
            node = tree[":id"]
            if node == nil { break }
        }

        config = node.Config
        tree = node.Children
    }

    return config
}

func(transport *CacheTransport) addNode(resource Resource, tree map[string]*Node, nodeConfig *NodeConfig) {
    pathNodes := strings.Split(resource.Name[1:], "/")

    if (resource.TTL != 0) {
        newConfig  := *nodeConfig
        nodeConfig = &newConfig
        nodeConfig.TTL = resource.TTL
    }

    currentTree := &tree
    for _, pathNode := range pathNodes {
        node := (*currentTree)[pathNode]
        if node == nil {
            node = &Node{
                Name:     pathNode,
                Children: make(map[string]*Node),
                Config:   nodeConfig,
            }
        }

        (*currentTree)[pathNode] = node
        currentTree = &(node.Children)
    }

    for _, childResource := range resource.Resources {
        transport.addNode(childResource, *currentTree, nodeConfig)
    }
}

func (transport *CacheTransport) RoundTrip(request *http.Request) (*http.Response, error) {
    cache, ok := transport.Cache.Get(request.URL.Path)
    if ok {
        transport.Logger.Printf("cache hitted with %s for %s", request.URL.Path, request.RemoteAddr)
        buf := bytes.NewBuffer(cache.Data)
        return http.ReadResponse(bufio.NewReader(buf), request)
    }

    path:= request.URL.Path
    config := transport.Lookup(path)

    request.URL.Scheme = config.TargetScheme
    request.URL.Host   = config.TargetHost
    request.Header.Set("X-Forwarded-Host", request.Header.Get("Host"))
    request.Host       = request.URL.Host

    transport.Logger.Printf("cache missed, request to %s for %s", request.URL.String(), request.RemoteAddr)

    response, err := http.DefaultTransport.RoundTrip(request)
    body, err := httputil.DumpResponse(response, true)
    if err != nil {
      return nil, err
    }

    newItem := &CacheItem {
        Data: body,
        Expiration: time.Now().Unix() + config.TTL,
    }

    transport.Logger.Printf("cache added for %s", request.URL.String())
    transport.Cache.Set(path, newItem)

    return response, err
}

func (router *Router) defaultConfig() *NodeConfig {
    return router.Proxy.Transport.(*CacheTransport).DefaultConfig
}

func (router *Router) cache() *Cache {
    return router.Proxy.Transport.(*CacheTransport).Cache
}

func (router *Router) flushCache(responseWriter http.ResponseWriter, request *http.Request) {
    user, password, ok := request.BasicAuth()
    auth := router.defaultConfig().Auth
    if !ok || user != auth.User || password != auth.Password {
        rand.Seed(time.Now().UnixNano())
        x := rand.Intn(3000)
        time.Sleep(time.Duration(x) * time.Millisecond)
        io.WriteString(responseWriter, "{\"flushed\": false, \"error\": \"auth failed\"}")
        return
    }

    router.Cache.Delete(request.URL.Path)
    io.WriteString(responseWriter, "{\"flushed\": true}")
}

func (router *Router) ServeHTTP(responseWriter http.ResponseWriter, request *http.Request) {
    router.Logger.Printf("request method: %s", request.Method)

    if request.Method == "GET" {
        router.Proxy.ServeHTTP(responseWriter, request)
        return
    }

    if request.Method == "FLUSH" {
        router.flushCache(responseWriter, request)
        return
    }
}

func (router *Router) Shutdown() {
    router.LogFile.Close()
}
