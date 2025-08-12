// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package geerpc

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"reflect"
)

const debugText = `<html>
	<body>
	<title>GeeRPC Services</title>
	{{range .}}
	<hr>
	Service {{.Name}}
	<hr>
		<table>
		<th align=center>Method</th><th align=center>Calls</th>
		{{range $name, $mtype := .Method}}
			<tr>
			<td align=left font=fixed>{{$name}}({{$mtype.ArgType}}, {{$mtype.ReplyType}}) error</td>
			<td align=center>{{$mtype.NumCalls}}</td>
			</tr>
		{{end}}
		</table>
	{{end}}
	</body>
	</html>`

var debug = template.Must(template.New("RPC debug").Parse(debugText))

type debugHTTP struct {
	*Server
}

type debugService struct {
	Name   string
	Method map[string]*methodType
}

// Runs at /debug/geerpc
func (server debugHTTP) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	// Build a sorted version of the data.
	var services []debugService
	server.serviceMap.Range(func(namei, svci interface{}) bool {
		svc := svci.(*service)
		services = append(services, debugService{
			Name:   namei.(string),
			Method: svc.method,
		})
		return true
	})
	err := debug.Execute(w, services)
	if err != nil {
		_, _ = fmt.Fprintln(w, "rpc: error executing template:", err.Error())
	}
}

type RPCWeb struct {
	*Server
}

// NewRPCWeb returns a new RPCWeb instance with the default server.
func NewRPCWeb() *RPCWeb {
	rpc_web := &RPCWeb{
		Server: DefaultServer,
	}
	// Register the debug HTTP handler
	rpc_web.RegisterDebugHTTP()
	return rpc_web
}

// RegisterDebugHTTP registers the debug HTTP handler at the default debug path.
func (web *RPCWeb) RegisterDebugHTTP() {
	http.Handle("/", web)
}

type RpcWebRequestBody struct {
	Method string        `json:"method"`
	Params []interface{} `json:"params"`
}
type RpcWebResponse struct {
	Result interface{} `json:"result"`
}

// ServeHTTP implements the http.Handler interface for RPCWeb.
func (web *RPCWeb) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	var requestBody *RpcWebRequestBody
	defer req.Body.Close()
	readCloser := req.Body

	err := json.NewDecoder(readCloser).Decode(&requestBody)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	svc, mtype, err := web.findService(requestBody.Method)
	if err != nil {
		http.Error(w, fmt.Sprintf("Service not found: %s", requestBody.Method), http.StatusNotFound)
		return
	}
	argv := mtype.newArgv()
	argvi := argv.Interface()
	if argv.Type().Kind() != reflect.Ptr {
		argvi = argv.Addr().Interface()
	}
	// todo 将requestBody的params 转为argv
	paramsBytes, err := json.Marshal(requestBody.Params[0])
	if err != nil {
		http.Error(w, "Invalid parameters", http.StatusBadRequest)
		return
	}
	if err := json.Unmarshal(paramsBytes, argvi); err != nil {
		http.Error(w, fmt.Sprintf("Invalid parameter types: %s", err.Error()), http.StatusBadRequest)
		return
	}

	replyv := mtype.newReplyv()
	err = svc.call(mtype, argv, replyv)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error calling method: %s", err.Error()), http.StatusInternalServerError)
		return
	}
	response := &RpcWebResponse{
		Result: replyv.Interface(),
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error encoding response: %s", err.Error()), http.StatusInternalServerError)
		return
	}
}
