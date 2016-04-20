package fs

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

var MAX_FIRST_LINE_SIZE = 500
var MAX_CONTENT_SIZE = 1 << 32

// This struct encapsulates all messages, including requests,
// responses and errors
// On-the-wire message formats are:
// 1. Write:
//       write <filename> <numbytes> [<exptime>]\r\n
//       <content bytes>\r\n
//    Write response:
//       OK <version>
// 2. Read:
//       read <filename>\r\n
//    Read response:
//       CONTENTS <version> <numbytes> <exptime> \r\n
//       <content bytes>\r\n
// 3. CAS: (Compare and Swap)
//       cas <filename> <version> <numbytes> [<exptime>]\r\n
//       <content bytes>\r\n
//    Cas response:
//       OK <version>\r\n
// 4. Delete:
//       delete <filename>\r\n
//     Delete response:
//       OK\r\n
// 5. Possible errors from these commands (instead of OK)
//     ERR_VERSION\r\n
//     ERR_FILE_NOT_FOUND\r\n
//     ERR_CMD_ERR\r\n
//     ERR_INTERNAL\r\n

type Msg struct {
	// Kind = the first character of the command. For errors, it
	// is the first letter after "ERR_", ('V' for ERR_VERSION, for
	// example), except for "ERR_CMD_ERR", for which the kind is 'M'
	Kind     byte
	Filename string
	Contents []byte
	Numbytes int
	Exptime  int // expiry time in seconds
	Version  int
}

func GetMsg(reader *bufio.Reader) (msg *Msg, msgerr error, fatalerr error) {
	buf := make([]byte, MAX_FIRST_LINE_SIZE)
	msg, msgerr, fatalerr = parseFirst(reader, buf)
	if fatalerr == nil {
		if msg.Kind == 'w' /*write*/|| msg.Kind == 'c' /*cas*/|| msg.Kind == 'C' /*CONTENTS*/{
			msg.Contents, fatalerr = parseSecond(reader, msg.Numbytes)
		}
	}
	return msg, msgerr, fatalerr
}

// The first line is all text. Some errors (such as unknown command, non-integer numbytes
// etc. are fatal errors; it is not possible to know where the command ends, and so we cannot
// recover to read the next command. The other errors are still errors, but we can at least
// consume the entirety of the current command.
func parseFirst(reader *bufio.Reader, buf []byte) (msg *Msg, msgerr error, fatalerr error) {
	var err error
	var msgstr string
	var fields []string
	
	if msgstr, err = fillLine(buf, reader); err != nil { // read until EOL
		return nil, nil, err
	}
	// validation functions checkN and toInt help avoid repeated use of "if err != nil ". Once
	// "err" is set, these functions don't do anything.
	checkN := func(fields []string, min int) {
		if err != nil {
			return
		}
		if len(fields) < min {
			err = errors.New("Incorrect number of fields in command")
			fatalerr = err
		}
	}
	// convert fields[fieldnum] to int if no error
	toInt := func(fieldnum int, recoverable bool) int {
		var i int
		if fatalerr != nil {
			return 0
		}
		i, err = strconv.Atoi(fields[fieldnum])
		if err != nil {
			if recoverable {
				msgerr = err
			} else {
				fatalerr = err
			}
		}
		return i
	}
	version := 0
	numbytes := 0
	exptime := 0
	response := false
	kind := byte(0)

	fields = strings.Fields(msgstr)
	switch fields[0] {
	case "read":
		checkN(fields, 2)
	case "write": // write <filename> <numbytes> [<exptime>]
		checkN(fields, 3)
		numbytes = toInt(2, false)
		if len(fields) >= 4 {
			exptime = toInt(3, true)
		}
	case "cas": // cas <filename> <version> <numbytes> [<exptime>]
		checkN(fields, 4)
		version = toInt(2, true)
		numbytes = toInt(3, false)
		if len(fields) == 5 {
			exptime = toInt(4, true)
		}
	case "delete":
		checkN(fields, 1)

	case "CONTENTS":
		checkN(fields, 4)
		version = toInt(1, true)
		numbytes = toInt(2, false)
		exptime = toInt(3, true)
		response = true

	case "OK":
		checkN(fields, 1)
		if len(fields) > 1 {
			version = toInt(1, true)
		}
		response = true
	case "ERR_VERSION":
		checkN(fields, 2)
		version = toInt(1, true)
		kind = 'V'
		response = true
	case "ERR_FILE_NOT_FOUND":
		kind = 'F'
		response = true
	case "ERR_CMD_ERR":
		kind = 'M' // 'C' is taken for contents
		response = true
	case "ERR_INTERNAL":
		kind = 'I'
		response = true
	default:
		fatalerr = fmt.Errorf("Command %s not recognized", fields[0])
	}
	if fatalerr == nil {
		var filename = ""
		if kind == 0 {
			kind = fields[0][0] // first char
		}
		if !response {
			filename = fields[1]
		}
		return &Msg{Kind: kind, Filename: filename, Numbytes: numbytes, Exptime: exptime, Version: version}, msgerr, nil
	} else {
		return nil, nil, fatalerr
	}
}

func parseSecond(reader *bufio.Reader, numbytes int) (buf []byte, err error) {
	if numbytes > MAX_CONTENT_SIZE {
		return nil, errors.New(fmt.Sprintf("numbytes cannot exceed %d", MAX_CONTENT_SIZE))
	}
	buf = make([]byte, numbytes+2) // includes CRLF
	_, err = io.ReadFull(reader, buf)
	if err == nil {
		last := len(buf) - 1
		if !(buf[last-1] == '\r' && buf[last] == '\n') {
			err = errors.New("Expected CRLF at end of contents line")
		}

	}
	if err == nil {
		return buf[:numbytes], nil // without the crlf
	} else {
		return nil, err
	}
}

func fillLine(buf []byte, reader *bufio.Reader) (string, error) {
	var err error
	count := 0
	for {
		var c byte
		if c, err = reader.ReadByte(); err != nil {
			return "", err
		}
		if c == '\r' {
			if c, err = reader.ReadByte(); err != nil || c != '\n' {
				return "", io.EOF
			}
			return string(buf[:count]), nil
		}
		if count >= len(buf) {
			return "", errors.New("Line too long")
		}
		buf[count] = c
		count += 1
	}
}
