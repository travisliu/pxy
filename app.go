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
