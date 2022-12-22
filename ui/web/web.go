package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/pprof"
	"os"
	"strconv"
	"time"

	assetfs "github.com/elazarl/go-bindata-assetfs"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/grafana/carbon-relay-ng/aggregator"
	"github.com/grafana/carbon-relay-ng/cfg"
	"github.com/grafana/carbon-relay-ng/destination"
	"github.com/grafana/carbon-relay-ng/matcher"
	"github.com/grafana/carbon-relay-ng/rewriter"
	"github.com/grafana/carbon-relay-ng/route"
	tbl "github.com/grafana/carbon-relay-ng/table"
	log "github.com/sirupsen/logrus"
)

var table *tbl.Table
var config cfg.Config

// error response contains everything we need to use http.Error
type handlerError struct {
	Error   error
	Message string
	Code    int
}

// a custom type that we can use for handling errors and formatting responses
type handler func(w http.ResponseWriter, r *http.Request) (interface{}, *handlerError)

// attach the standard ServeHTTP method to our handler so the http library can call it
func (fn handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// here we could do some prep work before calling the handler if we wanted to

	// call the actual handler
	response, err := fn(w, r)

	// check for errors
	if err != nil {
		//log.Printf("ERROR: %v", err.Error)
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Message+": "+err.Error.Error()), err.Code)
		return
	}
	if response == nil {
		//log.Printf("ERROR: response from method is nil")
		http.Error(w, "Internal server error. Check the logs.", http.StatusInternalServerError)
		return
	}

	// turn the response into JSON
	bytes, e := json.Marshal(response)
	if e != nil {
		http.Error(w, fmt.Sprintf("Error marshalling JSON:'%s'", e), http.StatusInternalServerError)
		return
	}

	// send the response and log
	w.Header().Set("Content-Type", "application/json")
	w.Write(bytes)
}

func showConfig(w http.ResponseWriter, r *http.Request) (interface{}, *handlerError) {
	return config, nil
}

func listTable(w http.ResponseWriter, r *http.Request) (interface{}, *handlerError) {
	t := table.Snapshot()
	return t, nil
}

func badMetricsHandler(w http.ResponseWriter, r *http.Request) (interface{}, *handlerError) {
	timespec := mux.Vars(r)["timespec"]
	duration, err := time.ParseDuration(timespec)
	if err != nil {
		return nil, &handlerError{err, "Could not parse timespec", http.StatusBadRequest}
	}

	records := table.Bad().Get(duration)
	return records, nil
}

func removeRewriter(w http.ResponseWriter, r *http.Request) (interface{}, *handlerError) {
	index := mux.Vars(r)["index"]
	idx, _ := strconv.Atoi(index)
	err := table.DelRewriter(idx)
	if err != nil {
		return nil, &handlerError{nil, "Could not find entry " + index, http.StatusNotFound}
	}
	return make(map[string]string), nil
}

func removeBlocklist(w http.ResponseWriter, r *http.Request) (interface{}, *handlerError) {
	index := mux.Vars(r)["index"]
	idx, _ := strconv.Atoi(index)
	err := table.DelBlocklist(idx)
	if err != nil {
		return nil, &handlerError{nil, "Could not find entry " + index, http.StatusNotFound}
	}
	return make(map[string]string), nil
}

func removeAggregator(w http.ResponseWriter, r *http.Request) (interface{}, *handlerError) {
	index := mux.Vars(r)["index"]
	idx, _ := strconv.Atoi(index)
	err := table.DelAggregator(idx)
	if err != nil {
		return nil, &handlerError{nil, err.Error(), http.StatusNotFound}
	}
	return make(map[string]string), nil
}

func removeDestination(w http.ResponseWriter, r *http.Request) (interface{}, *handlerError) {
	key := mux.Vars(r)["key"]
	index := mux.Vars(r)["index"]
	idx, _ := strconv.Atoi(index)
	err := table.DelDestination(key, idx)
	if err != nil {
		return nil, &handlerError{nil, "Could not find entry " + key + "/" + index, http.StatusNotFound}
	}
	return make(map[string]string), nil
}

func listRoutes(w http.ResponseWriter, r *http.Request) (interface{}, *handlerError) {
	t := table.Snapshot()
	return t.Routes, nil
}

func getRoute(w http.ResponseWriter, r *http.Request) (interface{}, *handlerError) {
	key := mux.Vars(r)["key"]
	route := table.GetRoute(key)
	if route == nil {
		return nil, &handlerError{nil, "Could not find route " + key, http.StatusNotFound}
	}
	return route, nil
}

func removeRoute(w http.ResponseWriter, r *http.Request) (interface{}, *handlerError) {
	key := mux.Vars(r)["key"]
	err := table.DelRoute(key)
	if err != nil {
		return nil, &handlerError{nil, "Could not find entry " + key, http.StatusNotFound}
	}
	return make(map[string]string), nil
}
func parseRouteRequest(r *http.Request) (route.Route, *handlerError) {
	var req struct {
		Key                  string
		Type                 string
		Prefix               string
		NotPrefix            string
		Sub                  string
		NotSub               string
		Regex                string
		NotRegex             string
		Address              string
		Spool                bool
		Pickle               bool
		periodFlush          int
		periodReconn         int
		ConnBufSize          int
		ConnIoBufSize        int
		SpoolBufSize         int
		SpoolMaxBytesPerFile int
		SpoolSyncEvery       int
		spoolSyncPeriod      int
		SpoolSleep           int
		UnspoolSleep         int
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return nil, &handlerError{err, "Couldn't parse json", http.StatusBadRequest}
	}
	dest, err := destination.New(
		req.Key,
		matcher.Matcher{},
		req.Address,
		table.SpoolDir,
		req.Spool,
		req.Pickle,
		time.Duration(req.periodFlush)*time.Millisecond,
		time.Duration(req.periodReconn)*time.Millisecond,
		req.ConnBufSize,
		req.ConnIoBufSize,
		req.SpoolBufSize,
		int64(req.SpoolMaxBytesPerFile),
		int64(req.SpoolSyncEvery),
		time.Duration(req.spoolSyncPeriod)*time.Millisecond,
		time.Duration(req.SpoolSleep)*time.Microsecond,
		time.Duration(req.UnspoolSleep)*time.Microsecond,
	)
	if err != nil {
		return nil, &handlerError{err, "unable to create destination", http.StatusBadRequest}
	}

	matcher, err := matcher.New(req.Prefix, req.NotPrefix, req.Sub, req.NotSub, req.Regex, req.NotRegex)
	if err != nil {
		return nil, &handlerError{err, "unable to create matcher for route", http.StatusBadRequest}
	}

	var ro route.Route
	var e error
	switch req.Type {
	case "sendAllMatch":
		ro, e = route.NewSendAllMatch(req.Key, matcher, []*destination.Destination{dest})
	case "sendFirstMatch":
		ro, e = route.NewSendFirstMatch(req.Key, matcher, []*destination.Destination{dest})
	default:
		return nil, &handlerError{nil, "unknown route type: " + req.Type, http.StatusBadRequest}
	}
	if e != nil {
		return nil, &handlerError{e, "could not create route", http.StatusBadRequest}
	}
	return ro, nil
}

func parseAggregateRequest(r *http.Request) (*aggregator.Aggregator, *handlerError) {
	var request struct {
		Fun       string
		OutFmt    string
		Cache     bool
		Interval  uint
		Wait      uint
		DropRaw   bool
		Regex     string `json:"regex,omitempty"`
		NotRegex  string `json:"notRegex,omitempty"`
		Prefix    string `json:"prefix,omitempty"`
		NotPrefix string `json:"notPrefix,omitempty"`
		Sub       string `json:"sub,omitempty"`
		NotSub    string `json:"notSub,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		return nil, &handlerError{err, "Couldn't parse json", http.StatusBadRequest}
	}

	matcher, err := matcher.New(request.Prefix, request.NotPrefix, request.Sub, request.NotSub, request.Regex, request.NotRegex)
	if err != nil {
		return nil, &handlerError{err, "unable to create matcher for route", http.StatusBadRequest}
	}

	aggregate, err := aggregator.New(request.Fun, matcher, request.OutFmt, request.Cache, request.Interval, request.Wait, request.DropRaw, table.In)
	if err != nil {
		return nil, &handlerError{err, "Couldn't create aggregator", http.StatusBadRequest}
	}
	return aggregate, nil
}

func parseRewriterRequest(r *http.Request) (rewriter.RW, *handlerError) {
	var request struct {
		Old string
		New string
		Max int
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		return rewriter.RW{}, &handlerError{err, "Couldn't parse json", http.StatusBadRequest}
	}
	rw, err := rewriter.New(request.Old, request.New, "", request.Max)
	if err != nil {
		return rewriter.RW{}, &handlerError{err, "Couldn't create rewriter", http.StatusBadRequest}
	}
	return rw, nil
}

/* needs updating, but using what api?
func updateRoute(w http.ResponseWriter, r *http.Request) (interface{}, *handlerError) {
	payload, err := parseRouteRequest(r)
	if err != nil {
		return nil, err
	}

	e := routes.Update(payload.Key, &payload.Addr, &payload.Patt)
	if e != nil {
		return nil, &handlerError{e, "Could not update route (" + e.Error() + ")", http.StatusBadRequest}
	}
	return routes.Map[payload.Key], nil
}

*/
func addAggregate(w http.ResponseWriter, r *http.Request) (interface{}, *handlerError) {
	aggregate, err := parseAggregateRequest(r)
	if err != nil {
		return nil, err
	}

	table.AddAggregator(aggregate)
	return map[string]string{"Message": "aggregate added"}, nil
}

func addRewrite(w http.ResponseWriter, r *http.Request) (interface{}, *handlerError) {
	rw, err := parseRewriterRequest(r)
	if err != nil {
		return nil, err
	}

	table.AddRewriter(rw)
	return map[string]string{"Message": "rewriter added"}, nil
}

func addRoute(w http.ResponseWriter, r *http.Request) (interface{}, *handlerError) {
	route, err := parseRouteRequest(r)
	if err != nil {
		return nil, err
	}

	table.AddRoute(route)
	return map[string]string{"Message": "route added"}, nil
}

func Start(addr string, c cfg.Config, t *tbl.Table, enableDebug bool) {
	table = t
	config = c

	router := mux.NewRouter()
	router.Handle("/badMetrics/{timespec}.json", handler(badMetricsHandler)).Methods("GET")
	router.Handle("/config", handler(showConfig)).Methods("GET")
	router.Handle("/table", handler(listTable)).Methods("GET")
	router.Handle("/blocklists/{index}", handler(removeBlocklist)).Methods("DELETE")
	router.Handle("/rewriters/{index}", handler(removeRewriter)).Methods("DELETE")
	router.Handle("/rewriters", handler(addRewrite)).Methods("POST")
	router.Handle("/aggregators/{index}", handler(removeAggregator)).Methods("DELETE")
	router.Handle("/aggregators", handler(addAggregate)).Methods("POST")
	router.Handle("/routes", handler(listRoutes)).Methods("GET")
	router.Handle("/routes", handler(addRoute)).Methods("POST")
	router.Handle("/routes/{key}", handler(getRoute)).Methods("GET")
	//router.Handle("/routes/{key}", handler(updateRoute)).Methods("POST")
	router.Handle("/routes/{key}", handler(removeRoute)).Methods("DELETE")
	router.Handle("/routes/{key}/destinations/{index}", handler(removeDestination)).Methods("DELETE")
	if enableDebug {
		log.Info("Enabled debug endpoints on /debug/pprof")
		router.HandleFunc("/debug/pprof/", pprof.Index)
		router.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		router.HandleFunc("/debug/pprof/profile", pprof.Profile)
		router.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		router.HandleFunc("/debug/pprof/trace", pprof.Trace)
		router.HandleFunc("/debug/pprof/{name}", func(w http.ResponseWriter, r *http.Request) {
			if p, ok := mux.Vars(r)["name"]; ok {
				pprof.Handler(p).ServeHTTP(w, r)
			} else {
				w.WriteHeader(404)
			}
		})
	}

	router.PathPrefix("/").Handler(http.FileServer(&assetfs.AssetFS{Asset: Asset, AssetDir: AssetDir, AssetInfo: AssetInfo, Prefix: "admin_http_assets/"}))
	loggedRouter := handlers.CombinedLoggingHandler(os.Stdout, router)
	http.Handle("/", loggedRouter)

	log.Infof("admin HTTP listener starting on %v", addr)
	err := http.ListenAndServe(addr, nil)
	if err != nil {
		fmt.Println("Error listening:", err.Error())
		os.Exit(1)
	}
}
