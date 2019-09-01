# Pxy
Pxy is a Resource-oriented configuration proxy cache server, focused on speed up legacy REST API services.

## Features
* Flexible configuration for each individual REST API resource.
* Low lock contention within the same thread as the worker for LRU processes.
* Lightweight and zero dependencies

## Installation
Simple install the package to your $GOPATH with the go tool from the shell:
```
$ go get  github.com/travisliu/pxy
```

## Configuration
`max_memory` is the limitation of memory usage in megabytes. default value is 128 megabytes.

`size_to_prung` is the size you want LRU to prung evertime memory usage hitted the limitation, default value is 1 megabyte.

`auth` is http BasicAuth for clients use to flush cache item with the `FLUSH` requeset method.

``` json
{
  "log_path": "/var/log/pxy/hello.log",
  "target_scheme": "https",
  "target_host": "jsonplaceholder.typicode.com",
  "max_memory": 1024,
  "size_to_prung": 5,
  "ttl": 60,
  "auth": {
    "user":   "flush",
    "password": "pwd!"
  },
  "resources": [
    {
      "name": "/posts/:id",
      "ttl": 3600,
      "resources": [
        {
          "name": "/comments",
          "ttl": 60    
        }
      ]
    },{
      "name": "/api/users",
      "target_host": "reqres.in"
    }
  ]
}
```

# Usage
Once everything is setup, you could  refer to the example `app.go` to build your own service.

``` go
package main

import (
    "fmt"
    "net/http"
    "github.com/travisliu/pxy"
)

func main() {
    proxy := pxy.Initialize("/root/pxy/config.json")
    fmt.Println("Listening port 789")
    err := http.ListenAndServe(":789", proxy)
    if err != nil {
        fmt.Println("Port 789 required is already in use")
    }
}
```
