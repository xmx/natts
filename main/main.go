package main

import (
	"context"
	"net/http"

	"github.com/xmx/natts/kuicx"
)

func main() {
	mux := kuicx.NewStreamMux()
	srv := &kuicx.Server{
		Handler: mux,
	}

	go srv.ListenAndServe(context.Background())

	lis, _ := mux.Listen(80)
	hs := &http.Server{
		ConnContext: kuicx.ConnContext,
	}

	_ = hs.Serve(lis)
}

func sss(w http.ResponseWriter, r *http.Request) {
	_ = kuicx.FromContext(r.Context()) // 得到节点 info
}
