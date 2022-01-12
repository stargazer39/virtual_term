package main

import (
	"bufio"
	"log"
	"os"
	"os/exec"
)

func main() {
	s, sErr := NewServer()
	check(sErr)

	s.InitServer()
	check(s.Start())

	// w := NewWatcher("test.txt")
	// w.Start()

	// buf := make([]byte, 1)

	// for {
	// 	nRead, rErr := w.Read(buf)
	// 	check(rErr)
	// 	fmt.Print(string(buf[:nRead]))
	// }

	/* watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()

	done := make(chan bool)
	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				log.Println("event:", event)
				if event.Op&fsnotify.Write == fsnotify.Write {
					log.Println("modified file:", event.Name)
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Println("error:", err)
			}
		}
	}()

	err = watcher.Add("test.txt")
	if err != nil {
		log.Fatal(err)
	}
	<-done */
	// TestCLI()
}

func TestCLI() {
	bPath, lErr := exec.LookPath(os.Args[1])
	check(lErr)

	os.Args[1] = bPath
	t := NewTerminal("Terminal 1", os.Args[1:]...)
	stdin := bufio.NewReader(os.Stdin)

	check(t.StartSingle())

	for {
		line, _, rErr := stdin.ReadLine()
		check(rErr)

		if string(line) == "exitpls" {
			t.Stop()
			break
		}
		check(t.Send(string(line)))
	}

	t.Wait()
	log.Println(bPath)
}

func check(e error) {
	if e != nil {
		log.Panic(e)
	}
}
