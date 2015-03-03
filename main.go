package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"

	"github.com/rancherio/host-api/app/common"
	"github.com/rancherio/host-api/auth"
	"github.com/rancherio/host-api/config"
	"github.com/rancherio/host-api/logs"
	"github.com/rancherio/host-api/stats"

	"github.com/golang/glog"
	"github.com/gorilla/mux"
)

func main() {
	err := config.Parse()
	if err != nil {
		glog.Error(err)
		os.Exit(1)
		return
	}

	flag.Parse()
	defer glog.Flush()

	router := mux.NewRouter()
	http.Handle("/", auth.AuthHttpInterceptor(router))

	router.Handle("/v1/stats", common.ErrorHandler(stats.GetStats)).Methods("GET")
	router.Handle("/v1/stats/{id}", common.ErrorHandler(stats.GetStats)).Methods("GET")
	router.Handle("/v1/logs/", common.ErrorHandler(logs.GetLogs)).Methods("GET")

	var listen = fmt.Sprintf("%s:%d", config.Config.Ip, config.Config.Port)
	err = http.ListenAndServe(listen, nil)

	if err != nil {
		glog.Error(err)
		os.Exit(1)
	}
}
