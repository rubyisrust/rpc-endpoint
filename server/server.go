package server

import (
	"crypto/ecdsa"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"
)

var Now = time.Now // used to mock time in tests

// No IPs blacklisted right now
var blacklistedIps = []string{"127.0.0.2"}

// Transactions should only be sent once to the relay
var txForwardedToRelay map[string]time.Time = make(map[string]time.Time)

func cleanupOldRelayForwardings() {
	for txHash, t := range txForwardedToRelay {
		if time.Since(t).Minutes() > 20 {
			delete(txForwardedToRelay, txHash)
		}
	}
}

// Metamask fix helper
var MetaMaskFix = NewMetaMaskFixer()

type RpcEndPointServer struct {
	version         string
	startTime       time.Time
	listenAddress   string
	proxyUrl        string
	relayUrl        string
	relaySigningKey *ecdsa.PrivateKey
}

func NewRpcEndPointServer(version string, listenAddress, proxyUrl, relayUrl string, relaySigningKey *ecdsa.PrivateKey) *RpcEndPointServer {
	return &RpcEndPointServer{
		startTime:       Now(),
		version:         version,
		listenAddress:   listenAddress,
		proxyUrl:        proxyUrl,
		relayUrl:        relayUrl,
		relaySigningKey: relaySigningKey,
	}
}

func (s *RpcEndPointServer) Start() {
	log.Printf("Starting rpc endpoint %s at %v...", s.version, s.listenAddress)

	// Handler for root URL (JSON-RPC on POST, public/index.html on GET)
	http.HandleFunc("/", http.HandlerFunc(s.HandleHttpRequest))
	http.HandleFunc("/health", http.HandlerFunc(s.handleHealthRequest))

	// Start serving
	if err := http.ListenAndServe(s.listenAddress, nil); err != nil {
		log.Fatalf("Failed to start rpc endpoint: %v", err)
	}
}

func (s *RpcEndPointServer) HandleHttpRequest(respw http.ResponseWriter, req *http.Request) {
	respw.Header().Set("Access-Control-Allow-Origin", "*")
	respw.Header().Set("Access-Control-Allow-Headers", "Accept,Content-Type")

	if req.Method == "GET" {
		http.Redirect(respw, req, "https://docs.flashbots.net/flashbots-protect/rpc/quick-start/", http.StatusFound)
		return
	}

	if req.Method == "OPTIONS" {
		respw.WriteHeader(http.StatusOK)
		return
	}

	request := NewRpcRequest(&respw, req, s.proxyUrl, s.relayUrl, s.relaySigningKey)
	request.process()
}

type HealthResponse struct {
	Now       time.Time `json:"time"`
	StartTime time.Time `json:"startTime"`
	Version   string    `json:"version"`
}

func (s *RpcEndPointServer) handleHealthRequest(respw http.ResponseWriter, req *http.Request) {
	res := HealthResponse{
		Now:       Now(),
		StartTime: s.startTime,
		Version:   s.version,
	}

	jsonResp, err := json.Marshal(res)
	if err != nil {
		log.Panicln("healthCheck json error:", err)
		respw.WriteHeader(http.StatusInternalServerError)
		return
	}

	respw.WriteHeader(http.StatusOK)
	respw.Write(jsonResp)
}

func IsBlacklisted(ip string) bool {
	for i := range blacklistedIps {
		if strings.HasPrefix(ip, blacklistedIps[i]) {
			return true
		}
	}
	return false
}
