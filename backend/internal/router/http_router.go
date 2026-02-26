package router

import "net/http"

type RouteRegistrar interface {
	RegisterRoutes(mux *http.ServeMux)
	WrapHTTPHandler(next http.Handler) http.Handler
}

func NewHTTPHandler(registrar RouteRegistrar) http.Handler {
	mux := http.NewServeMux()
	registrar.RegisterRoutes(mux)
	return registrar.WrapHTTPHandler(mux)
}
