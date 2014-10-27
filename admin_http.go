package main

import (
	"encoding/json"
	"fmt"
	"github.com/elazarl/go-bindata-assetfs"
	//    "errors"
	"github.com/gorilla/mux"
	"io/ioutil"
	"net/http"
	"os"
)

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
		//log.Printf("ERROR: %v\n", err.Error)
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Message), err.Code)
		return
	}
	if response == nil {
		//log.Printf("ERROR: response from method is nil\n")
		http.Error(w, "Internal server error. Check the logs.", http.StatusInternalServerError)
		return
	}

	// turn the response into JSON
	bytes, e := json.Marshal(response)
	if e != nil {
		http.Error(w, "Error marshalling JSON", http.StatusInternalServerError)
		return
	}

	// send the response and log
	w.Header().Set("Content-Type", "application/json")
	w.Write(bytes)
}

func listRoutes(w http.ResponseWriter, r *http.Request) (interface{}, *handlerError) {
	t := table.Snapshot()
	return t, nil
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

func parseRouteRequest(r *http.Request) (Route, *handlerError) {
	data, e := ioutil.ReadAll(r.Body)
	if e != nil {
		return Route{}, &handlerError{e, "Could not read request", http.StatusBadRequest}
	}

	var payload Route
	e = json.Unmarshal(data, &payload)
	if e != nil {
		return Route{}, &handlerError{e, "Could not parse JSON", http.StatusBadRequest}
	}
	return payload, nil
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
/* needs updating. not sure what's the best way to get the route from the http data
func addRoute(w http.ResponseWriter, r *http.Request) (interface{}, *handlerError) {
	payload, err := parseRouteRequest(r)
	if err != nil {
		return nil, err
	}
    var t interface{}

	if payload.Type == "sendAllMatch" {
		t = sendAllMatch(1)
	} else if payload.Type == "sendFirstMatch" {
		t = sendFirstMatch(1)
	} else {
		return nil, &handlerError{errors.New("unknown route type"), fmt.Sprintf("unknown route type '%v'", payload.Type), http.StatusBadRequest}
	}

	route, err := NewRoute(t, payload.Key, payload.Matcher.Prefix, payload.Matcher.Sub, payload.Matcher.Regex)
	if err != nil {
		return nil, &handlerError{err, "Could not create route (" + err.Error() + ")", http.StatusBadRequest}
	}
	err = table.AddRoute(route)
	if err != nil {
		return nil, &handlerError{err, "Could not add route to table (" + err.Error() + ")", http.StatusInternalServerError}
	}
	return routes.Map[payload.Key], nil
}
*/

func HttpListener(addr string, t *Table) {
	table = t

	// setup routes
	router := mux.NewRouter()
	router.Handle("/routes", handler(listRoutes)).Methods("GET")
	//router.Handle("/routes", handler(addRoute)).Methods("POST")
	router.Handle("/routes/{key}", handler(getRoute)).Methods("GET")
	//router.Handle("/routes/{key}", handler(updateRoute)).Methods("POST")
	router.Handle("/routes/{key}", handler(removeRoute)).Methods("DELETE")
	router.PathPrefix("/").Handler(http.FileServer(&assetfs.AssetFS{Asset, AssetDir, "admin/data/"}))
	http.Handle("/", router)

	log.Notice("admin HTTP listener starting on %v", addr)
	err := http.ListenAndServe(addr, nil)
	if err != nil {
		fmt.Println("Error listening:", err.Error())
		os.Exit(1)
	}
}
