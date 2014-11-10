package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/rancherio/host-api/app/common"
	"github.com/rancherio/host-api/auth"
	"github.com/rancherio/host-api/config"
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
	http.Handle("/", httpInterceptor(router))

	router.Handle("/v1/stats", common.ErrorHandler(stats.GetStats)).Methods("GET")
	router.Handle("/v1/stats/{id}", common.ErrorHandler(stats.GetStats)).Methods("GET")

	var listen = fmt.Sprintf("%s:%d", config.Config.Ip, config.Config.Port)
	err = http.ListenAndServe(listen, nil)

	if err != nil {
		glog.Error(err)
		os.Exit(1)
	}
}

func httpInterceptor(router http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		startTime := time.Now()

		if !auth.Auth(w, req) {
			http.Error(w, "Failed authentication", 401)
			return
		}

		router.ServeHTTP(w, req)

		finishTime := time.Now()
		elapsedTime := finishTime.Sub(startTime)

		switch req.Method {
		case "GET":
			// We may not always want to StatusOK, but for the sake of
			// this example we will
			common.LogAccess(w, req, elapsedTime)
		case "POST":
			// here we might use http.StatusCreated
		}

	})
}
