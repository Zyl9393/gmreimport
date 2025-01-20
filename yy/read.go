package yy

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
)

// Reader for GameMaker .yy files; basically JSON with extra trailing commas in arrays and objects.
// Not sure how strings are escaped; this implementation currently returns an error when a backslash is encountered. If you find one let me know.
type Reader struct {
	r            io.Reader
	b            []byte
	l            int // length of b
	lineNumber   int
	columnNumber int
	p            int // parser scan position
}

func FromFile(filePath string) (interface{}, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r := Reader{r: f}
	return r.ParseYY()
}

func NewReader(r io.Reader) *Reader {
	return &Reader{r: r}
}

func (r *Reader) ParseYY() (v interface{}, err error) {
	r.b, err = io.ReadAll(r.r)
	if err != nil {
		return nil, err
	}
	r.l = len(r.b)
	r.lineNumber = 1
	r.columnNumber = 1
	r.skipWhitespace()
	v, err = r.parseValue()
	if err != nil {
		return nil, err
	}
	r.skipWhitespace()
	if r.p != r.l {
		return nil, fmt.Errorf("extra non-whitespace content after value in line %d, column %d", r.lineNumber, r.columnNumber)
	}
	return v, nil
}

func (r *Reader) parseValue() (interface{}, error) {
	if r.p >= r.l {
		return nil, errors.New("unexpected EOF. Expected yy value")
	}
	token := r.b[r.p]
	if token == '[' {
		return r.parseArray()
	} else if token == '{' {
		return r.parseObject()
	} else if token == '"' {
		return r.parseString()
	} else if token == '-' || token == '+' || (token >= '0' && token <= '9') || token == '.' || token == 'e' || token == 'E' {
		return r.parseNumber()
	} else if token == 't' || token == 'f' {
		return r.parseBoolean()
	} else if token == 'n' {
		return r.parseNull()
	}
	return nil, fmt.Errorf("unexpected symbol in line %d, column %d. Expected yy value", r.lineNumber, r.columnNumber)
}

func (r *Reader) parseArray() (arr []interface{}, err error) {
	err = r.parseLiteral([]byte(`[`))
	if err != nil {
		return nil, fmt.Errorf("unexpected symbol in line %d, column %d. Expected '['", r.lineNumber, r.columnNumber)
	}
	for {
		r.skipWhitespace()
		if r.p >= r.l {
			return nil, errors.New("unexpected EOF while parsing array")
		}
		if r.b[r.p] == ',' {
			r.advance(1)
			r.skipWhitespace()
			if r.p >= r.l {
				return nil, errors.New("unexpected EOF while parsing array")
			}
		}
		if r.b[r.p] == ']' {
			break
		}
		v, err := r.parseValue()
		if err != nil {
			return nil, err
		}
		arr = append(arr, v)
	}
	err = r.parseLiteral([]byte(`]`))
	if err != nil {
		return nil, err
	}
	return arr, nil
}

func (r *Reader) parseObject() (v interface{}, err error) {
	err = r.parseLiteral([]byte(`{`))
	if err != nil {
		return nil, fmt.Errorf("unexpected symbol in line %d, column %d. Expected '{'", r.lineNumber, r.columnNumber)
	}
	var keyValues map[string]interface{} = make(map[string]interface{})
	for {
		r.skipWhitespace()
		if r.p >= r.l {
			return nil, errors.New("unexpected EOF while parsing object")
		}
		if r.b[r.p] == ',' {
			r.advance(1)
			r.skipWhitespace()
			if r.p >= r.l {
				return nil, errors.New("unexpected EOF while parsing object")
			}
		}
		if r.b[r.p] == '}' {
			break
		}
		key, err := r.parseString()
		if err != nil {
			return nil, err
		}
		r.skipWhitespace()
		err = r.parseLiteral([]byte(":"))
		if err != nil {
			return nil, err
		}
		r.skipWhitespace()
		value, err := r.parseValue()
		if err != nil {
			return nil, err
		}
		keyValues[key] = value
	}
	err = r.parseLiteral([]byte(`}`))
	if err != nil {
		return nil, err
	}
	return keyValues, nil
}

func (r *Reader) parseString() (v string, err error) {
	err = r.parseLiteral([]byte(`"`))
	if err != nil {
		return "", fmt.Errorf("unexpected symbol in line %d, column %d. Expected string", r.lineNumber, r.columnNumber)
	}
	var numBytes int
	for r.p+numBytes < r.l {
		c := r.b[r.p+numBytes]
		if c == '"' {
			break
		} else if c == '\\' {
			return "", fmt.Errorf("no idea what to do with backslash in string in line %d, column %d", r.lineNumber, r.columnNumber)
		}
		numBytes++
	}
	s := string(r.b[r.p : r.p+numBytes])
	r.advance(numBytes)
	err = r.parseLiteral([]byte(`"`))
	if err != nil {
		return "", fmt.Errorf("unexpected symbol in line %d, column %d. Expected end of string", r.lineNumber, r.columnNumber)
	}
	return s, nil
}

func (r *Reader) parseNumber() (v interface{}, err error) {
	if r.p >= r.l {
		return nil, errors.New("unexpected EOF trying to read number")
	}
	var isFloat bool
	var numBytes int
	for r.p+numBytes < r.l {
		c := r.b[r.p+numBytes]
		if numBytes == 0 && (c == '-' || c == '+') {
		} else if c == '.' || c == 'e' || c == 'E' {
			isFloat = true
		} else if c >= '0' && c <= '9' {
		} else {
			break
		}
		numBytes++
	}
	if isFloat {
		v, err = strconv.ParseFloat(string(r.b[r.p:r.p+numBytes]), 64)
	} else {
		v, err = strconv.ParseInt(string(r.b[r.p:r.p+numBytes]), 10, 64)
	}
	if err != nil {
		return nil, fmt.Errorf("could not parse number in line %d, column %d: %w", r.lineNumber, r.columnNumber, err)
	}
	r.advance(numBytes)
	return v, nil
}

func (r *Reader) parseBoolean() (interface{}, error) {
	if r.p >= r.l {
		return nil, errors.New("unexpected EOF trying to read boolean")
	}
	if r.b[r.p] == 't' {
		return true, r.parseLiteral([]byte("true"))
	}
	return false, r.parseLiteral([]byte("false"))
}

func (r *Reader) parseNull() (interface{}, error) {
	return nil, r.parseLiteral([]byte("null"))
}

func (r *Reader) parseLiteral(literal []byte) error {
	litLen := len(literal)
	if litLen > r.l-r.p || !bytes.Equal(literal, r.b[r.p:r.p+litLen]) {
		return fmt.Errorf("unexpected literal in line %d, column %d. Expected %#q", r.lineNumber, r.columnNumber, string(literal))
	}
	r.advance(litLen)
	return nil
}

func (r *Reader) advance(amount int) error {
	for amount > 0 {
		if r.p >= r.l {
			return errors.New("unexpected EOF")
		}
		c := r.b[r.p]
		r.p++
		r.columnNumber++
		if c == '\n' {
			r.lineNumber++
			r.columnNumber = 1
		}
		amount--
	}
	return nil
}

func (r *Reader) skipWhitespace() {
	for r.p < r.l {
		c := r.b[r.p]
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
			r.p++
			r.columnNumber++
			if c == '\n' {
				r.lineNumber++
				r.columnNumber = 1
			}
		} else {
			break
		}
	}
}
