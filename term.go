package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"github.com/google/uuid"
)

type Terminal struct {
	title     string
	id        uuid.UUID
	stderr    *os.File
	stdout    *os.File
	output    *os.File
	stdinChan chan string
	/* 	stdinErr   chan error */
	closeEvent chan error
	process    *exec.Cmd
	Running    bool
}

func NewTerminal(title string, args ...string) *Terminal {
	log.Println(args[0], args[1:])

	// Normal killing dosen't work so
	cmd := exec.Command(args[0], args[1:]...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	return &Terminal{
		title:     title,
		process:   cmd,
		stdinChan: make(chan string),
		/* stdinErr:  make(chan error), */
		id:      uuid.New(),
		Running: false,
	}
}

func (t *Terminal) Start() error {
	// Create folder
	dirName := fmt.Sprintf("proc-file-%s", t.id.String())
	dirErr := os.Mkdir(dirName, 0666)

	if dirErr != nil {
		return dirErr
	}

	// Create files
	stdErrFile, stdErrErr := os.Create(filepath.Join(dirName, "stderr.txt"))

	if stdErrErr != nil {
		t.err(stdErrErr.Error())
		return stdErrErr
	}

	t.stderr = stdErrFile

	stdOutFile, stdOutErr := os.Create(filepath.Join(dirName, "stdout.txt"))

	if stdOutErr != nil {
		t.err(stdOutErr.Error())
		return stdOutErr
	}

	t.stdout = stdOutFile

	// Pipe to files
	t.process.Stderr = stdErrFile
	t.process.Stdout = stdOutFile

	// Event channel
	e := make(chan bool)

	go func() {
		// Open stdin pipe
		stdinPipe, inErr := t.process.StdinPipe()

		if inErr != nil {
			t.err(inErr.Error())
			return
		}

		// Open bufio stdin writer
		bufStdin := bufio.NewWriter(stdinPipe)

		e <- true
		go func() {
			for {
				c := <-t.stdinChan
				_, wErr := bufStdin.WriteString(c + "\n")
				/* t.stdinErr <- wErr */
				if wErr != nil {
					t.log(wErr.Error())
					break
				}
				bufStdin.Flush()
				t.log(c)
			}

			bufStdin.Flush()
			stdinPipe.Close()
		}()
	}()

	<-e
	return t.process.Start()
}

func (t *Terminal) StartSingle() error {
	// Create folder
	dirName := fmt.Sprintf("proc-file-%s", t.id.String())
	outFilePath := filepath.Join(dirName, "output.txt")
	dirErr := os.Mkdir(dirName, 0666)

	if dirErr != nil {
		return dirErr
	}

	// Create files
	fFile, fErr := os.Create(outFilePath)

	if fErr != nil {
		return fErr
	}

	fFile.Close()

	outputFile, outputErr := os.OpenFile(outFilePath, os.O_RDWR, 0644)

	if outputErr != nil {
		t.err(outputErr.Error())
		return outputErr
	}

	t.output = outputFile

	stdOut, stdOutErr := t.process.StdoutPipe()

	if stdOutErr != nil {
		return stdOutErr
	}

	t.process.Stderr = t.process.Stdout

	go func() {
		buf := make([]byte, 4)
		defer outputFile.Close()

		for {
			n, e := stdOut.Read(buf)

			if e != nil {
				break
			}

			outputFile.Write(buf[:n])
		}
	}()

	// Event channel
	e := make(chan bool)

	go func() {
		// Open stdin pipe
		stdinPipe, inErr := t.process.StdinPipe()

		if inErr != nil {
			t.err(inErr.Error())
			return
		}

		// Open bufio stdin writer
		bufStdin := bufio.NewWriter(stdinPipe)

		e <- true
		go func() {
			for {
				c := <-t.stdinChan
				_, wErr := bufStdin.WriteString(c + "\n")

				if wErr != nil {
					t.log(wErr.Error())
					break
				}
				bufStdin.Flush()
				t.log(c)
			}

			bufStdin.Flush()
			stdinPipe.Close()
		}()
	}()

	<-e

	sErr := t.process.Start()

	if sErr != nil {
		return sErr
	}

	go func() {
		t.closeEvent <- t.process.Wait()
		t.Running = false
	}()

	t.Running = true
	return nil
}

func (t *Terminal) log(message string) {
	log.Printf("%s : %s\n", t.title, message)
}

func (t *Terminal) err(message string) {
	t.log(message)
	t.Stop()
}

func (t *Terminal) Send(command string) error {
	if !t.Running {
		return fmt.Errorf("Terminal is not running")
	}
	t.stdinChan <- command
	return nil
}

func (t *Terminal) GetOutputFile() *os.File {
	return t.output
}

func (t *Terminal) GetOutputFilePath() string {
	return filepath.Join(fmt.Sprintf("proc-file-%s", t.id), "output.txt")
}

func (t *Terminal) Stop() error {
	pgid, err := syscall.Getpgid(t.process.Process.Pid)
	if err != nil {
		return err
	}

	if kErr := syscall.Kill(-pgid, 15); kErr != nil {
		return kErr
	}

	// kErr := t.process.Wait()

	if t.stdout != nil {
		t.stdout.Close()
	}

	if t.stderr != nil {
		t.stderr.Close()
	}

	close(t.stdinChan)
	t.Running = false
	/* close(t.stdinErr) */
	return nil
}

func (t *Terminal) Wait() {
	<-t.closeEvent
}
