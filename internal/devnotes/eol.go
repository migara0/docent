package devnotes

import (
	"bytes"
	"os"
)

// Notes written on Windows usually end their lines with \r\n. Everything that parses a note works on \n, so notes are read through
// readNote; a command that rewrites a note writes it back with the line endings it had (withEOL), so a Windows file stays a Windows file.

// ReadNote reads a note with its line endings turned into \n.
func ReadNote(path string) ([]byte, error) {
	data, _, err := readNoteEOL(path)
	return data, err
}

// readNoteEOL is ReadNote, and also says whether the file used \r\n.
func readNoteEOL(path string) ([]byte, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false, err
	}
	data, crlf := toLF(data)
	return data, crlf, nil
}

func toLF(data []byte) ([]byte, bool) {
	if !bytes.Contains(data, []byte("\r\n")) {
		return data, false
	}
	return bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n")), true
}

// withEOL turns \n back into \r\n for a file that used them.
func withEOL(data []byte, crlf bool) []byte {
	if !crlf {
		return data
	}
	return bytes.ReplaceAll(data, []byte("\n"), []byte("\r\n"))
}
