package test_utils

import (
	"io/ioutil"

	log "github.com/Sirupsen/logrus"
	jwt "github.com/dgrijalva/jwt-go"

	"github.com/rancherio/websocket-proxy/proxy"
)

var privateKey interface{}

func ParseTestPrivateKey() interface{} {
	keyBytes, err := ioutil.ReadFile("../test_utils/private.pem")
	if err != nil {
		log.Fatal("Failed to parse private key.", err)
	}

	privateKey, err := jwt.ParseRSAPrivateKeyFromPEM(keyBytes)
	if err != nil {
		log.Fatal("Failed to parse private key.", err)
	}

	return privateKey
}

func GetTestConfig(addr string) *proxy.Config {
	config := &proxy.Config{
		ListenAddr: addr,
	}

	pubKey, err := proxy.ParsePublicKey("../test_utils/public.pem")
	if err != nil {
		log.Fatal("Failed to parse key. ", err)
	}
	config.PublicKey = pubKey
	return config
}
