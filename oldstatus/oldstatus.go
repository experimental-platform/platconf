package oldstatus

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"

	"github.com/fsnotify/fsnotify"
)

// Opts contains command line parameters for the 'oldstatus' command
type Opts struct {
	Port int `short:"p" long:"port" description:"Port on which to listen" default:"7887"`
}

// StatusData is the data structure sent to the status page
type StatusData struct {
	Status       string   `json:"status"`
	Progress     *float32 `json:"progress"`
	What         *string  `json:"what"`
	sync.RWMutex `json:"-"`
}

var statusFilePath = "/etc/protonet/system/configure-script-status"

func updateStatusFromFile(status *StatusData, filePath string) error {
	var tempStatus StatusData
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}

	decoder := json.NewDecoder(f)
	err = decoder.Decode(&tempStatus)
	if err != nil {
		return err
	}

	status.Lock()
	defer status.Unlock()
	status.Status = tempStatus.Status
	status.Progress = tempStatus.Progress
	status.What = tempStatus.What
	return nil
}

func watchStatusFileForChange(status *StatusData, filePath string) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatalf("Failed to initalize a file watcher: %s", err.Error())
	}
	defer watcher.Close()

	err = watcher.Add(filePath)
	if err != nil {
		log.Fatalf("Failed to add file '%s' to the file watcher: %s", filePath, err.Error())
	}

	log.Println("Status file watcher started on", statusFilePath)

	for {
		select {
		case event := <-watcher.Events:
			if event.Op&fsnotify.Write == fsnotify.Write {
				err := updateStatusFromFile(status, statusFilePath)
				if err != nil {
					log.Println("ERROR: failed to read status from SKVS file:", err.Error())
				}
			}
		case err := <-watcher.Errors:
			log.Println("ERROR: failed while watching SKVS update status file:", err.Error())
		}
	}
}

// Execute is the function ran when the 'oldstatus' command is used
func (o *Opts) Execute(args []string) error {
	var status StatusData

	err := updateStatusFromFile(&status, statusFilePath)
	if err != nil {
		log.Printf("ERROR: failed to read status from SKVS file: %s", err.Error())
	} else {
		go watchStatusFileForChange(&status, statusFilePath)
	}

	http.HandleFunc("/json", func(w http.ResponseWriter, r *http.Request) {
		status.RLock()
		defer status.RUnlock()
		w.Header().Set("Content-Type", "application/json")
		encoder := json.NewEncoder(w)
		encoder.Encode(&status)
	})

	http.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Not found.", http.StatusNotFound)
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(htmlBody))
		log.Printf("Serving the status HTML page to '%s'", r.RemoteAddr)
	})

	log.Println("Starting platform-install-status")
	err = http.ListenAndServe(fmt.Sprintf(":%d", o.Port), nil)
	return err
}