package main

import (
	"fmt"
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
	watcher   *fsnotify.Watcher
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

	fw.watcher = watcher

	log.Println("Started")
	// done := make(chan bool)
	go func() {
		defer fw.Close()
	E:
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					break E
				}
				if event.Op&fsnotify.Write == fsnotify.Write {
					fw.eventChan <- true
				}
			case _, ok := <-watcher.Errors:
				if !ok {
					break E
				}
			}
		}
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
		<-fw.eventChan
		if !fw.close {
			goto read
		}
	}

	//fw.connect = false
	return nRead, rErr
}

func (fw *FileWatcher) Close() error {
	if !fw.close {
		return nil
	}

	fw.close = true
	close(fw.eventChan)
	wErr := fw.watcher.Close()
	cErr := fw.file.Close()

	if wErr != nil || cErr != nil {
		return fmt.Errorf("watcher : %s, file %s", wErr, cErr)
	}

	return nil
}
