package main

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/zoetrope/yiq"
)

var called int = 0

func TestMain(m *testing.M) {
	called = 0
	code := m.Run()
	defer os.Exit(code)
}

func TestyiqRun(t *testing.T) {
	var assert = assert.New(t)

	e := &yiq.Engine{}
	result := run(e, false)
	assert.Zero(result)
	assert.Equal(2, called)

	result = run(e, true)
	assert.Equal(1, called)

	result = run(e, false)
	assert.Zero(result)
}

func TestyiqRunWithError(t *testing.T) {
	called = 0
	var assert = assert.New(t)
	e := &yiq.Engine{}
	result := run(e, false)
	assert.Equal(2, result)
	assert.Equal(0, called)
}

type EngineMock struct{ err error }

func (e *EngineMock) Run() *yiq.EngineResult {
	return &yiq.EngineResult{
		Err:     fmt.Errorf(""),
		Qs:      ".querystring",
		Content: `{"test": "result"}`,
	}
}
