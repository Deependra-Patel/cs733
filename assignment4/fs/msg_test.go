package fs

import (
	"bufio"
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func mkReader(str string) *bufio.Reader {
	return bufio.NewReader(strings.NewReader(str))
}

func msgExpect(t *testing.T, msg *Msg, expectedMsg *Msg, msgerr error, fatalerr error) {
	if msgerr != nil {
		t.Fatalf("Expected no msg errors, got '%s'", msgerr)
	}
	if fatalerr != nil {
		t.Fatalf("Expected no msg errors, got '%s'", fatalerr)
	}
	if msg.Kind != expectedMsg.Kind {
		t.Fatalf("Expected msg kind '%c', got '%c'", expectedMsg.Kind, msg.Kind)
	}
	if expectedMsg.Filename != "" && msg.Filename != expectedMsg.Filename {
		t.Fatalf("Expected filename '%s', got '%s'", expectedMsg.Filename, msg.Filename)
	}
	if msg.Exptime != expectedMsg.Exptime {
		t.Fatalf("Expected exptime '%d', got '%d'", expectedMsg.Exptime, msg.Exptime)
	}

	if expectedMsg.Contents != nil && bytes.Compare(msg.Contents, expectedMsg.Contents) != 0 {
		t.Fatalf("Expected contents = '%s', got '%s'", expectedMsg.Contents, msg.Contents)
	}
}

func TestMsg_Read(t *testing.T) {
	r := mkReader("read foobar\r\n")
	msg, msgerr, fatalerr := GetMsg(r)
	msgExpect(t, msg, &Msg{Kind: 'r', Filename: "foobar"}, msgerr, fatalerr)
}

func TestMsg_InvalidMsg(t *testing.T) {
	r := mkReader("dummy")
	_, _, fatalerr := GetMsg(r)
	if fatalerr == nil {
		t.Fatal("Expected Failure on Invalid Msg")
	}

	
}

func TestMsg_Write(t *testing.T) {
	r := mkReader("write foobar 3\r\nabc\r\n")
	msg, msgerr, fatalerr := GetMsg(r)

	msgExpect(t, msg, &Msg{Kind: 'w', Filename:"foobar", Contents: []byte("abc")}, msgerr, fatalerr)
}

func TestMsg_WriteNoExp(t *testing.T) {
	str := "\r\n\r\n\r\n\r\n\r\n" // contents
	r := mkReader(fmt.Sprintf("write foobar %d 20\r\n", len(str)) + str + "\r\n")
	msg, msgerr, fatalerr := GetMsg(r)
	msgExpect(t, msg, &Msg{Kind:'w', Filename: "foobar", Exptime: 20, Contents: []byte(str)}, msgerr, fatalerr)
}

func TestMsg_CasExpiry(t *testing.T) {
	contents := "\x00\r\n\x01abcdef"
	version := 101
	exptime := 20
	msgstr := fmt.Sprintf("cas foobar %d %d %d\r\n", version, len(contents), exptime) + contents + "\r\n"

	r := mkReader(msgstr)
	msg, msgerr, fatalerr := GetMsg(r)
	msgExpect(t, msg,
		&Msg{
			Kind: 'c',
			Filename: "foobar",
			Version: version,
			Numbytes: len(contents),
			Exptime: exptime,
		}, msgerr, fatalerr)
}

func TestMsg_Delete(t *testing.T) {
	r := mkReader("delete xyz\r\n") //  'dummy' in place of exptime
	msg, msgerr, fatalerr := GetMsg(r)
	msgExpect(t, msg, &Msg{Kind: 'd', Filename: "xyz"}, msgerr, fatalerr)
}

func TestMsg_RecoverableErrors(t *testing.T) {
	checkerr := func(str string) {
		r := mkReader(str)
		_, msgerr, fatalerr := GetMsg(r)
		if fatalerr != nil {
			t.Fatalf("Expected recoverable error, but got unrecoverable. Cmd = %s", str)
		}
		if msgerr == nil {
			t.Fatalf("Expected recoverable error, but got none. Cmd =%s", str)
		}
	}

	checkerr("write foobar 3 dummy\r\nabc\r\n") //  'dummy' in place of exptime
	checkerr("cas foobar ver 3 \r\nabc\r\n") // 'ver' in place of version
}

func TestMsg_UnrecoverableErrors(t *testing.T) {
	checkfatal := func(str string) {
		r := mkReader(str)
		_, _, fatalerr := GetMsg(r)
		if fatalerr == nil {
			t.Fatalf("Expected unrecoverable error for msg = %s", str)
		}
	}

	// Non-numeric length
	checkfatal("cas foobar 100 dummy \r\nabc\r\n") // 'dummy' in place of numbytes

	// Nonexistent command
	checkfatal("dummy foobar ver dummy \r\nabc\r\n") // 'dummy' in place of numbytes

	// Line too long
	filename := strings.Repeat("a", 500)
	checkfatal("read " + filename + "\r\n")

	// Less fields than expected
	checkfatal("write foobar\r\ncontents\r\nread")
}

func TestMsg_Responses(t *testing.T) {
	// CONTENTS response 
	version := 100
	contents := "\x00\r\n\x01\r\r\n"
	exptime := 10
	msgstr := fmt.Sprintf("CONTENTS %d %d %d\r\n", version, len(contents), exptime) + contents + "\r\n"
	r := mkReader(msgstr)
	msg, msgerr, fatalerr := GetMsg(r)
	msgExpect(t, msg,
		&Msg{Kind:'C', Contents: []byte(contents),
			Numbytes: len(contents), Exptime: exptime, Version: version},
		msgerr, fatalerr)


	// ERR RESPONSES
	r = mkReader("ERR_VERSION 203\r\n")
	msg, msgerr, fatalerr = GetMsg(r)
	msgExpect(t, msg, &Msg{Kind:'V', Version: version}, msgerr, fatalerr)

	r = mkReader("ERR_CMD_ERR\r\n")
	msg, msgerr, fatalerr = GetMsg(r)
	msgExpect(t, msg, &Msg{Kind:'M'}, msgerr, fatalerr)

	r = mkReader("ERR_INTERNAL\r\n")
	msg, msgerr, fatalerr = GetMsg(r)
	msgExpect(t, msg, &Msg{Kind:'I'}, msgerr, fatalerr)

	r = mkReader("ERR_FILE_NOT_FOUND\r\n")
	msg, msgerr, fatalerr = GetMsg(r)
	msgExpect(t, msg, &Msg{Kind:'F'}, msgerr, fatalerr)
}

func TestMsg_MultipleCmds(t *testing.T) {
	str := fmt.Sprintf("write xx 3 10\r\nabc\r\nread yy\r\nwrite zz 3 10\r\nabc\r\n")
	r := mkReader(str)
	msg, msgerr, fatalerr := GetMsg(r)
	msgExpect(t, msg, &Msg{Kind: 'w', Filename: "xx", Contents: []byte("abc"), Numbytes: 3, Exptime: 10}, msgerr, fatalerr)

	msg, msgerr, fatalerr = GetMsg(r)
	msgExpect(t, msg, &Msg{Kind: 'r', Filename: "yy"}, msgerr, fatalerr)

	msg, msgerr, fatalerr = GetMsg(r)
	msgExpect(t, msg, &Msg{Kind: 'w', Filename: "zz", Contents: []byte("abc"), Numbytes: 3, Exptime: 10}, msgerr, fatalerr)
}
