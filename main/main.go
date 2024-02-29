package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"

	"github.com/xmx/natts/kuicx"
)

func main1() {
	cert, _ := tls.LoadX509KeyPair("resources/tls/local.pem", "resources/tls/local.key")
	cfg := &tls.Config{Certificates: []tls.Certificate{cert}, NextProtos: []string{"vnet"}}

	mux := kuicx.NewStreamMux()
	srv := &kuicx.Server{
		Addr:      ":8443",
		Handler:   mux,
		TLSConfig: cfg,
	}

	go func() {
		err := srv.ListenAndServe(context.Background())
		fmt.Println(err)
	}()

	lis, _ := mux.Listen(80)
	hs := &http.Server{
		ConnContext: kuicx.ConnContext,
		Handler:     new(ginx),
	}

	_ = hs.Serve(lis)
}

func sss(w http.ResponseWriter, r *http.Request) {
	_ = kuicx.FromContext(r.Context()) // 得到节点 info
}

type ginx struct{}

func (ginx) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	info := kuicx.FromContext(r.Context())
	fmt.Println(info)
}
