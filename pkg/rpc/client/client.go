package rpcclient

import (
	"bytes"
	"context"
	"net"
	"net/http"

	"github.com/0xef53/kvmrun/pkg/rpc/common"

	jsonrpc "github.com/gorilla/rpc/v2/json2"
	"golang.org/x/net/http2"
)

type TlsClient struct {
	client   *http.Client
	endpoint string
}

func NewTlsClient(endpoint, certFile, keyFile string) (*TlsClient, error) {
	tlsConfig, err := rpccommon.TlsConfig(certFile, keyFile)
	if err != nil {
		return nil, err
	}

	transport := &http.Transport{TLSClientConfig: tlsConfig}
	if err := http2.ConfigureTransport(transport); err != nil {
		return nil, err
	}

	c := TlsClient{
		client:   &http.Client{Transport: transport},
		endpoint: endpoint,
	}

	return &c, nil
}

func (c *TlsClient) Request(addr, method string, args, res interface{}) error {
	return do(c.client, "https://"+addr+":9393"+c.endpoint, method, args, res)
}

type UnixClient struct {
	client   *http.Client
	endpoint string
}

func NewUnixClient(endpoint string) (*UnixClient, error) {
	transport := &http.Transport{
		DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
			return net.Dial("unix", "@/run/kvmrund.sock"+string(0))
		},
	}

	c := UnixClient{
		client:   &http.Client{Transport: transport},
		endpoint: endpoint,
	}

	return &c, nil
}

func (c *UnixClient) Request(method string, args, res interface{}) error {
	return do(c.client, "http://127.0.0.1"+c.endpoint, method, args, res)
}

func do(client *http.Client, url, method string, args, res interface{}) error {
	message, err := jsonrpc.EncodeClientRequest(method, args)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(message))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Method-Name", method)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return jsonrpc.DecodeClientResponse(resp.Body, &res)
}
