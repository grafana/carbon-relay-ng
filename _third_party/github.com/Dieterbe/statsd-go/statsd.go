package statsd

import (
	"fmt"
	"math/rand"
	"net"
	"time"
)

type Client struct {
	Enabled bool
	Host    string
	Port    int
	Prefix  string
	conn    net.Conn
}

func NewClient(enabled bool, host string, port int, prefix string) *Client {
	client := Client{Enabled: enabled, Host: host, Port: port, Prefix: prefix}
	err := client.Open()
	if err != nil {
		fmt.Println(err.Error())
	}
	rand.Seed(time.Now().UnixNano())
	return &client
}

// Open udp connection
func (client *Client) Open() (err error) {
	addr := fmt.Sprintf("%s:%d", client.Host, client.Port)
	conn, err := net.Dial("udp", addr)
	client.conn = conn
	return
}

// Close udp connection
func (client *Client) Close() {
	client.conn.Close()
}

// metrics to be computed into summary stats
func (client *Client) Timing(metric string, val int64) {
	client.Send(metric, fmt.Sprintf("%d", val), "ms", 1)
}

// metrics to be computed into summary stats, sampled
func (client *Client) TimingSampled(metric string, val int64, sampleRate float32) {
	client.Send(metric, fmt.Sprintf("%d", val), "ms", sampleRate)
}

// Increment counter
func (client *Client) Increment(metric string) {
	client.Send(metric, "1", "c", 1)
}

// Increment counter, sampled
func (client *Client) IncrementWithSampling(metric string, sampleRate float32) {
	client.Send(metric, "1", "c", sampleRate)
}

// Decrement counter
func (client *Client) Decrement(metric string) {
	client.Send(metric, "-1", "c", 1)
}

// Decrement counter, sampled
func (client *Client) DecrementSampled(metric string, sampleRate float32) {
	client.Send(metric, "-1", "c", sampleRate)
}

// Update gauge
func (client *Client) Gauge(metric string, val int64) {
	client.Send(metric, fmt.Sprintf("%d", val), "g", 1)
}

// Submit udp packet
func (client *Client) Send(metric, value, metrictype string, sampleRate float32) {
	if !client.Enabled {
		return
	}
	var msg string
	if sampleRate < 1 {
		chance_pct := rand.Intn(100)
		sampleRatePct := int(sampleRate * 100)
		if sampleRatePct < chance_pct {
			return
		}
		msg = fmt.Sprintf("%s%s:%s|%v@%f", client.Prefix, metric, value, metrictype, sampleRate)
	} else {
		msg = fmt.Sprintf("%s%s:%s|%v", client.Prefix, metric, value, metrictype)
	}
	fmt.Fprintf(client.conn, msg)
}
