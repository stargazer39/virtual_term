package main

import "fmt"

type TermManager struct {
	active map[string]*Terminal
}

func NewTermManager() *TermManager {
	return &TermManager{active: make(map[string]*Terminal)}
}

func (tm *TermManager) NewTerminal(title string, args ...string) string {
	t := NewTerminal(title, args...)
	tm.active[t.id.String()] = t

	return t.id.String()
}

func (tm *TermManager) GetTerm(id string) (*Terminal, error) {
	t, found := tm.active[id]

	if found {
		return t, nil
	}

	return nil, fmt.Errorf("Terminal not found")
}

func (tm *TermManager) HasTerm(id string) bool {
	_, found := tm.active[id]

	return found
}

func (tm *TermManager) GetLogWatcher(id string) (*FileWatcher, error) {
	t, tErr := tm.GetTerm(id)

	if tErr != nil {
		return nil, tErr
	}

	o := t.GetOutputFilePath()
	w := NewWatcher(o)

	if sErr := w.Start(); sErr != nil {
		return nil, sErr
	}

	return w, nil
}

func (tm *TermManager) StartTerminal(id string) error {
	t, tErr := tm.GetTerm(id)

	if tErr != nil {
		return tErr
	}

	return t.StartSingle()
}

func (tm *TermManager) StopTerminal(id string) error {
	t, tErr := tm.GetTerm(id)

	if tErr != nil {
		return tErr
	}

	return t.Stop()
}
