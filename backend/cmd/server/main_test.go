package main

import (
	"net"
	"strings"
	"testing"
)

func TestListenHTTPListenerBindsAvailableAddress(t *testing.T) {
	listener, err := listenHTTPListener("127.0.0.1:0")
	if err != nil {
		t.Fatalf("listenHTTPListener() error = %v", err)
	}
	_ = listener.Close()
}

func TestListenHTTPListenerReportsAddressInUse(t *testing.T) {
	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen() error = %v", err)
	}
	defer occupied.Close()

	_, err = listenHTTPListener(occupied.Addr().String())
	if err == nil {
		t.Fatal("listenHTTPListener() error = nil, want address already in use")
	}
	if !strings.Contains(err.Error(), "address already in use") {
		t.Fatalf("listenHTTPListener() error = %q, want address already in use hint", err.Error())
	}
	if !strings.Contains(err.Error(), "PORT/ZFLOW_ADDR") {
		t.Fatalf("listenHTTPListener() error = %q, want PORT/ZFLOW_ADDR hint", err.Error())
	}
}
