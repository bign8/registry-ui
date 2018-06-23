package main

import (
	"encoding/json"
	"flag"
	"html/template"
	"net/http"
	"net/url"
	"sort"
	"time"
)

var (
	addr = flag.String("addr", ":8080", "ui address")
	apis = flag.String("api", "http://localhost:5000", "api address")
	tout = flag.Duration("tout", time.Second, "api cache timeout")
	host string // host:port of api server
)

func main() {
	parseArgs()
	println("Serving on " + *addr)
	http.ListenAndServe(*addr, &memory{
		tout: time.Now().Add(-*tout),
		data: func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "pending first load", http.StatusOK)
		},
	})
}

var htmlTPL = template.Must(template.New("").Parse(`<!DOCTYPE html>
<html lang="en" dir="ltr">
	<head>
		<meta charset="utf-8">
		<title>Registry Listing</title>
	</head>
	<body>
		<ul>
		{{- range .}}
			<li>{{.}}</li>
		{{- else}}<li>No Repositories Found</li>{{end}}
		</ul>
	</body>
</html>`))

func parseArgs() {
	flag.Parse()
	api, err := url.Parse(*apis)
	if err != nil {
		panic(err)
	}
	host = api.Host
}

type memory struct {
	data http.HandlerFunc
	tout time.Time
}

func (mem *memory) update() error {
	res, err := http.Get(*apis + "/v2/_catalog")
	if err != nil {
		return err
	}
	// TODO: check status
	var catalog struct {
		Repos []string `json:"repositories"`
	}
	if err := json.NewDecoder(res.Body).Decode(&catalog); err != nil {
		return err
	}
	if err := res.Body.Close(); err != nil {
		return err
	}

	data := make(map[string][]string, len(catalog.Repos))

	for _, name := range catalog.Repos {
		res, err := http.Get(*apis + "/v2/" + name + "/tags/list")
		if err != nil {
			return err
		}
		// TODO: check status
		var tags struct {
			Name string   `json:"name"`
			Tags []string `json:"tags"`
		}
		if err := json.NewDecoder(res.Body).Decode(&tags); err != nil {
			return err
		}
		data[name] = tags.Tags
		if err := res.Body.Close(); err != nil {
			return err
		}
	}

	data2 := make([]string, 0, len(catalog.Repos))
	for repo, tags := range data {
		for _, tag := range tags {
			data2 = append(data2, host+"/"+repo+":"+tag)
		}
	}
	sort.Strings(data2)

	mem.data = func(w http.ResponseWriter, r *http.Request) {
		if err := htmlTPL.Execute(w, data2); err != nil {
			println("causing failure " + err.Error())
		}
	}
	return nil
}

func (mem *memory) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	now := time.Now()
	if now.After(mem.tout) {
		mem.tout = now.Add(*tout)
		if err := mem.update(); err != nil {
			mem.data = func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		}
	}
	w.Header().Add("X-BIGN8-TIMEOUT", mem.tout.String())
	w.Header().Add("X-BIGN8-NOW", now.String())
	mem.data(w, r)
}
