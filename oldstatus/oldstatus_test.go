package oldstatus

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGetStatus(t *testing.T) {
	// prepare
	var status StatusData

	mux := getStatusReadMux(&status)
	srv := httptest.NewServer(mux)

	u, err := url.Parse(srv.URL)
	assert.Nil(t, err)
	u.Path = path.Join(u.Path, "json")

	// test1
	status.Lock()
	var p1 float32 = 1.2
	status.Progress = &p1
	w1 := "foobar_img"
	status.What = &w1
	status.Status = "whatever"
	status.Unlock()

	resp, err := http.Get(u.String())
	assert.Nil(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	var responseStatus1 StatusData
	decoder1 := json.NewDecoder(resp.Body)
	assert.Nil(t, decoder1.Decode(&responseStatus1))

	status.RLock()
	assert.Equal(t, p1, *responseStatus1.Progress)
	assert.Equal(t, w1, *responseStatus1.What)
	assert.Equal(t, "whatever", responseStatus1.Status)
	status.RUnlock()

	// test2
	status.Lock()
	status.Progress = nil
	status.What = nil
	status.Status = "other_status"
	status.Unlock()

	resp, err = http.Get(u.String())
	assert.Nil(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	var responseStatus2 StatusData
	decoder2 := json.NewDecoder(resp.Body)
	assert.Nil(t, decoder2.Decode(&responseStatus2))

	status.RLock()
	assert.Nil(t, responseStatus2.Progress)
	assert.Nil(t, responseStatus2.What)
	assert.Equal(t, "other_status", responseStatus2.Status)
	status.RUnlock()
}

func TestGetFavicon(t *testing.T) {
	// prepare
	mux := getStatusReadMux(nil)
	srv := httptest.NewServer(mux)

	u, err := url.Parse(srv.URL)
	assert.Nil(t, err)
	u.Path = path.Join(u.Path, "favicon.ico")

	resp, err := http.Get(u.String())
	assert.Nil(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestGetHTML(t *testing.T) {
	// prepare
	mux := getStatusReadMux(nil)
	srv := httptest.NewServer(mux)

	u, err := url.Parse(srv.URL)
	assert.Nil(t, err)
	u.Path = path.Join(u.Path)

	resp, err := http.Get(u.String())
	assert.Nil(t, err)
	responseBody, err := ioutil.ReadAll(resp.Body)
	assert.Nil(t, err)
	assert.Equal(t, htmlBody, string(responseBody))
}

func TestPutOnUnixSocket(t *testing.T) {
	// prepare path for the socket
	f, err := ioutil.TempFile("", "platconf-unittest-")
	assert.Nil(t, err)
	socketPath := f.Name()
	f.Close()
	os.Remove(socketPath)

	// start
	var status StatusData
	err = listenOnUnixSocket(&status, socketPath)
	defer os.Remove(socketPath)
	assert.Nil(t, err)

	// make a UNIX socket-connected HTTP client
	fakeDial := func(proto, addr string) (conn net.Conn, err error) {
		return net.Dial("unix", socketPath)
	}

	client := http.Client{
		Transport: &http.Transport{
			Dial: fakeDial,
		},
	}

	// test1
	req1, err := http.NewRequest("PUT", "http://foobar/status", strings.NewReader(`{"status": "foobar1"}`))
	assert.Nil(t, err)
	resp1, err := client.Do(req1)
	assert.Nil(t, err)
	assert.Equal(t, http.StatusAccepted, resp1.StatusCode)

	status.RLock()
	assert.Equal(t, "foobar1", status.Status)
	assert.Nil(t, status.Progress)
	assert.Nil(t, status.What)
	status.RUnlock()

	// test2
	req2, err := http.NewRequest("PUT", "http://foobar/status", strings.NewReader(`{"status": "whatever", "what": "something", "progress": 12312.12412}`))
	assert.Nil(t, err)
	resp2, err := client.Do(req2)
	assert.Nil(t, err)
	assert.Equal(t, http.StatusAccepted, resp2.StatusCode)

	status.RLock()
	assert.Equal(t, "whatever", status.Status)
	assert.Equal(t, float32(12312.12412), *status.Progress)
	assert.Equal(t, "something", *status.What)
	status.RUnlock()
}

func TestFileWatcher(t *testing.T) {
	// prepare path for the socket
	f, err := ioutil.TempFile("", "platconf-unittest-")
	assert.Nil(t, err)
	path := f.Name()
	f.Close()
	defer os.Remove(path)

	// start
	var status StatusData
	go func() {
		errWatcher := watchStatusFileForChange(&status, path)
		assert.Nil(t, errWatcher)
	}()
	time.Sleep(time.Second)

	// test1
	err = ioutil.WriteFile(path, []byte(`{"status": "foobar1"}`), 0644)
	assert.Nil(t, err)
	time.Sleep(time.Second)

	status.RLock()
	assert.Equal(t, "foobar1", status.Status)
	assert.Nil(t, status.Progress)
	assert.Nil(t, status.What)
	status.RUnlock()

	// test2
	err = ioutil.WriteFile(path, []byte(`{"status": "whatever", "what": "something", "progress": 12312.12412}`), 0644)
	assert.Nil(t, err)
	time.Sleep(time.Second)

	status.RLock()
	assert.Equal(t, "whatever", status.Status)
	assert.Equal(t, float32(12312.12412), *status.Progress)
	assert.Equal(t, "something", *status.What)
	status.RUnlock()
}

func TestIntegration(t *testing.T) {
	// temporary status file
	f, err := ioutil.TempFile("", "platconf-unittest-")
	assert.Nil(t, err)
	statusFilePath := f.Name()
	f.Close()
	defer os.Remove(statusFilePath)
	assert.Nil(t, err)

	// temporary socket
	f, err = ioutil.TempFile("", "platconf-unittest-")
	assert.Nil(t, err)
	socketPath := f.Name()
	f.Close()
	os.Remove(socketPath)

	// start
	port := (rand.Int()%1000 + 7000) // random port in range 7000-7999
	o := Opts{
		Port:         port,
		StatusFile:   statusFilePath,
		StatusSocket: socketPath,
	}

	go o.Execute([]string{})
	time.Sleep(time.Second)

	var status StatusData

	// test1
	err = ioutil.WriteFile(statusFilePath, []byte(`{"status": "foobar1"}`), 0644)
	assert.Nil(t, err)
	time.Sleep(time.Second)

	resp1, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/json", port))
	assert.Nil(t, err)

	dec1 := json.NewDecoder(resp1.Body)
	err = dec1.Decode(&status)
	assert.Nil(t, err)

	// no locking needed, as we are the only user of this variable
	assert.Equal(t, "foobar1", status.Status)
	assert.Nil(t, status.Progress)
	assert.Nil(t, status.What)

	// test2
	// make a UNIX socket-connected HTTP client
	fakeDial := func(proto, addr string) (conn net.Conn, err error) {
		return net.Dial("unix", socketPath)
	}

	client := http.Client{
		Transport: &http.Transport{
			Dial: fakeDial,
		},
	}

	req1, err := http.NewRequest("PUT", fmt.Sprintf("http://127.0.0.1:%d/status", port), strings.NewReader(`{"status": "whatever", "what": "something", "progress": 12312.12412}`))
	assert.Nil(t, err)
	resp2, err := client.Do(req1)
	assert.Nil(t, err)
	assert.Equal(t, http.StatusAccepted, resp2.StatusCode)

	resp3, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/json", port))
	assert.Nil(t, err)

	dec2 := json.NewDecoder(resp3.Body)
	err = dec2.Decode(&status)
	assert.Nil(t, err)

	// no locking needed, as we are the only user of this variable
	assert.Equal(t, "whatever", status.Status)
	assert.Equal(t, float32(12312.12412), *status.Progress)
	assert.Equal(t, "something", *status.What)
}
