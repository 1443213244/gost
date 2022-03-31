package main

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"runtime"

	_ "net/http/pprof"

	"github.com/ginuerzh/gost"
	"github.com/go-log/log"
)

var (
	configureFile string
	baseCfg       = &baseConfig{}
	pprofAddr     string
	pprofEnabled  = os.Getenv("PROFILING") != ""
)

func init() {
	gost.SetLogger(&gost.LogLogger{})

	var (
		printVersion bool
	)

	flag.Var(&baseCfg.route.ChainNodes, "F", "forward address, can make a forward chain")
	flag.Var(&baseCfg.route.ServeNodes, "L", "listen address, can listen on multiple ports (required)")
	flag.StringVar(&configureFile, "C", "", "configure file")
	flag.BoolVar(&baseCfg.Debug, "D", false, "enable debug log")
	flag.BoolVar(&printVersion, "V", false, "print version")
	if pprofEnabled {
		flag.StringVar(&pprofAddr, "P", ":6060", "profiling HTTP server address")
	}
	flag.Parse()

	if printVersion {
		fmt.Fprintf(os.Stdout, "gost %s (%s %s/%s)\n",
			gost.Version, runtime.Version(), runtime.GOOS, runtime.GOARCH)
		os.Exit(0)
	}

	if configureFile != "" {
		_, err := parseBaseConfig(configureFile)
		if err != nil {
			log.Log(err)
			os.Exit(1)
		}
	}
	if flag.NFlag() == 0 {
		flag.PrintDefaults()
		os.Exit(0)
	}
}

func RemoveIndex(r []route, index int) []route {
	return append(r[:index], r[index+1:]...)
}

func saveCfg(cfg *baseConfig, r route) string {
	for _, v := range cfg.Routes {
		if v.ServeNodes[0] == r.ServeNodes[0] {
			return "error"
		}
	}
	cfg.Routes = append(cfg.Routes, r)
	saveBaseConfig(configureFile, cfg)
	return "success"
}

func delCfg(cfg *baseConfig, r route) string {
	for i, v := range cfg.Routes {
		if v.ServeNodes[0] == r.ServeNodes[0] {
			fmt.Println(v.ServeNodes[0])
			cfg.Routes = RemoveIndex(cfg.Routes, i)
			saveBaseConfig(configureFile, cfg)
			return "success"
		}
	}
	return "error"
}

func startServe(r *route) error {
	var rs []router
	rts, err := r.GenRouters()
	if err != nil {
		fmt.Println(err)
		return err
	}
	rs = append(rs, rts...)

	if len(rs) == 0 {
		return errors.New("invalid config")
	}
	for i := range rs {
		go rs[i].Serve()
	}

	return nil
}

func redJosnToRotue(body []byte) (route, error) {
	var r route
	err := json.Unmarshal(body, &r)
	if err != nil {
		fmt.Println(err)
		return r, err
	}
	return r, nil
}

func addHandler(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Log(err)
		w.Write([]byte("failed"))
	}

	r1, _ := redJosnToRotue(body)
	result := saveCfg(baseCfg, r1)
	startServe(&r1)
	w.Write([]byte("{code:200,data:" + result + "}"))
}

func delHandler(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Log(err)
	}
	r1, _ := redJosnToRotue(body)
	result := delCfg(baseCfg, r1)
	w.Write([]byte("{code:200,data:" + result + "}"))
}

func main() {
	if pprofEnabled {
		go func() {
			log.Log("profiling server on", pprofAddr)
			log.Log(http.ListenAndServe(pprofAddr, nil))
		}()
	}

	// NOTE: as of 2.6, you can use custom cert/key files to initialize the default certificate.
	tlsConfig, err := tlsConfig(defaultCertFile, defaultKeyFile, "")
	if err != nil {
		// generate random self-signed certificate.
		cert, err := gost.GenCertificate()
		if err != nil {
			log.Log(err)
			os.Exit(1)
		}
		tlsConfig = &tls.Config{
			Certificates: []tls.Certificate{cert},
		}
	} else {
		log.Log("load TLS certificate files OK")
	}

	gost.DefaultTLSConfig = tlsConfig

	if err := start(); err != nil {
		log.Log(err)
		os.Exit(1)
	}

	http.HandleFunc("/add", addHandler)
	http.HandleFunc("/del", delHandler)
	http.ListenAndServe(":999", nil)

	// select {}
}

func start() error {
	gost.Debug = baseCfg.Debug

	var routers []router
	rts, err := baseCfg.route.GenRouters()
	if err != nil {
		return err
	}
	routers = append(routers, rts...)
	for _, route := range baseCfg.Routes {
		rts, err := route.GenRouters()
		if err != nil {
			return err
		}
		routers = append(routers, rts...)
	}

	if len(routers) == 0 {
		return errors.New("invalid config")
	}
	for i := range routers {
		go routers[i].Serve()
	}

	return nil
}
