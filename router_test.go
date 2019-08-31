package pxy

import (
  "testing"
  "io"
  "net"
  "net/http"
  "net/http/httptest"
)

func StubHttpServer(url, response string) *httptest.Server{
  server := httptest.NewUnstartedServer(http.HandlerFunc(func(responseWrite http.ResponseWriter, reqeust *http.Request) {
    io.WriteString(responseWrite, response)
  }))
  listener, _ := net.Listen("tcp", url)
  server.Listener.Close()
  server.Listener = listener
  return server
}

func TestConfig(t *testing.T) {
    config := Config{}
    config.load("/root/pxy/config_example.json")

    expectingTTL := 60
    if config.TTL != int64(expectingTTL) {
        t.Errorf("expecting TTL is %d, but get %d", expectingTTL, config.TTL)
    }

    expectingTTL = 10
    if config.Resources[1].TTL != int64(expectingTTL) {
        t.Errorf("expecting TTL is %d, but get %d", expectingTTL, config.Resources[1].TTL)
    }
}

func TestCacheTransport(t *testing.T) {
    router := Initialize("/root/pxy/config_example.json")
    proxy  := router.Proxy

    host := "127.0.0.1:6666"
    key := "/posts/5"
    url := "http://" + host + key
    server := StubHttpServer(host, "{\"userId\":2, \"id\":5, \"title\": \"sint suscipit perspiciatis velit dolorum rerum ipsa laboriosam odio\"}")
    server.Start()
    defer server.Close()

    postRequest, _ := http.NewRequest("GET", url, nil)
    response := httptest.NewRecorder()
    router.ServeHTTP(response, postRequest)

    _, ok := router.Proxy.Transport.(*CacheTransport).Cache.Get(key)
    if !ok {
        t.Errorf("didn't get the expectiing key: %s", key)
    }

    key = "/posts/6"
    url = "http://" + host + key
    flushRequest, _ := http.NewRequest("FLUSH", url, nil)
    flushRequest.SetBasicAuth("flush", "abc")

    _ = proxy
    response = httptest.NewRecorder()
    router.ServeHTTP(response, flushRequest)

    _, ok = proxy.Transport.(*CacheTransport).Cache.Get(key)
    if ok {
        t.Errorf("should't get the unexpectiing key: %s", key)
    }
    router.Logger.Printf("flush result: %s", response.Body.String())

    key = "/posts/7?item=%3Fa%3D3"
    url = "http://" + host + key
    paramsRequest, _ := http.NewRequest("GET", url, nil)

    _ = proxy
    response = httptest.NewRecorder()
    router.ServeHTTP(response, paramsRequest)

}

