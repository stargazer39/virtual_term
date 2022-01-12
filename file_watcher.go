package main

import (
	"io"
	"log"
	"os"

	"github.com/fsnotify/fsnotify"
)

type FileWatcher struct {
	file      *os.File
	path      string
	started   bool
	connect   bool
	close     bool
	eventChan chan bool
}

func NewWatcher(file string) *FileWatcher {
	return &FileWatcher{
		path:      file,
		started:   false,
		connect:   false,
		close:     false,
		eventChan: make(chan bool),
	}
}

func (fw *FileWatcher) Start() error {
	oFile, oErr := os.Open(fw.path)

	if oErr != nil {
		return oErr
	}
	fw.file = oFile

	watcher, wErr := fsnotify.NewWatcher()

	if wErr != nil {
		return wErr
	}

	log.Println("Started")
	// done := make(chan bool)
	go func() {
		defer watcher.Close()
	E:
		for {
			select {
			case event, ok := <-watcher.Events:
				//log.Println("event:", event)
				if !ok {
					break E
				}
				if event.Op&fsnotify.Write == fsnotify.Write {
					// if fw.connect {
					// 	fw.eventChan <- true
					// }
					fw.eventChan <- true
					//log.Println("modified file:", event.Name)
				}
			case _, ok := <-watcher.Errors:
				// log.Println("error:", err)
				if !ok {
					break E
				}
			}
		}
		fw.Close()
	}()

	if err := watcher.Add(fw.path); err != nil {
		return err
	}
	return nil
}

func (fw *FileWatcher) Read(buf []byte) (n int, e error) {
read:
	nRead, rErr := fw.file.Read(buf)

	if rErr == io.EOF {
		//log.Println("io.EOF")
		//fw.connect = true
		<-fw.eventChan
		if !fw.close {
			goto read
		}
	}

	//fw.connect = false
	return nRead, rErr
}

func (fw *FileWatcher) Close() {
	if !fw.close {
		return
	}
	fw.close = true
	select {
	case fw.eventChan <- true:
	default:
	}
	fw.file.Close()
}
