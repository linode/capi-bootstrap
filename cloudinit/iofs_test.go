package cloudinit

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIoFS(t *testing.T) {
	expectedContent := []byte("test input")
	stdinReader, stdinWriter, err := os.Pipe()
	assert.NoError(t, err, "error creating pipe")
	_, err = stdinWriter.Write(expectedContent)
	assert.NoError(t, err, "error writing to pipe")
	oldStdin := os.Stdin
	os.Stdin = stdinReader
	defer func() { os.Stdin = oldStdin }()

	testFS := IoFS{Reader: os.Stdin}
	ioFile, err := testFS.Open("stdin")
	assert.NoError(t, err, "Failed to open file for reading")
	defer ioFile.Close()
	stat, err := ioFile.Stat()
	assert.NoError(t, err, "Failed to stat file for reading")
	assert.Equal(t, "|0", stat.Name(), "expected size: ", len(expectedContent))
	actualContent := make([]byte, 10)
	_, err = ioFile.Read(actualContent)
	assert.NoError(t, err, "Failed to read file content")
	assert.Equal(t, string(expectedContent), string(actualContent), "File content does not match expected content")
}
