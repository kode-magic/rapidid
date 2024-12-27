package rapidid

import (
	"bytes"
	"crypto/rand"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/mr-tron/base58"
	"strings"
	"time"
)

type ID []byte

const (
	prefixAllowedLen           = 3
	timeBytesLen               = 7
	randomBytesLen             = 12
	byteLength                 = timeBytesLen + randomBytesLen
	stringEncodedLen           = 25 // Math.Ceil(byteLength * 8 / 6)
	stringEncodedLenWithPrefix = 3 /*prefix*/ + 1 /*hyphen*/ + stringEncodedLen
)

var (
	separator             = "-"
	separatorBytes        = []byte(separator)
	alphabets             = base58.NewAlphabet("123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz")
	epochTime             = time.Date(2022, 01, 01, 0, 0, 0, 0, time.UTC)
	errBytesSizeMismatch  = fmt.Errorf("invalid identifier bytes; must have at least length %d", byteLength)
	errStringSizeMismatch = fmt.Errorf("invalid identifier string; must have %v or %v characters",
		stringEncodedLen, stringEncodedLenWithPrefix)
)

// Generate is the same as GenerateWithPrefix("")
func Generate() string {
	return GenerateWithPrefix("")
}

// GenerateWithPrefix is syntactic sugar to New().String() and panic if New() returns error
func GenerateWithPrefix(prefix string) string {
	prefix = strings.TrimSuffix(prefix, separator)
	id, err := New(prefix)
	if err != nil {
		panic(err)
	}
	return id.String()
}

// New creates a 152 bits time ordered universal ID
// with the specified prefix used to identifying similar IDs.
// The prefix must be and empty string to a 3-letter word
// without whitespace or hyphens
func New(prefix string) (ID, error) { return newID(prefix) }

// Bytes gives the raw byte representation of ID
func (t ID) Bytes() []byte {
	return t[:]
}

// String returns the string encoded representation of ID
// The length of the string can be 26 or 30 if a prefix was set.
func (t ID) String() string {
	prefixPart := ""
	valuePart := t[:]
	if len(t) > byteLength {
		separatorIndex := bytes.Index(t[:], separatorBytes)
		if separatorIndex == -1 {
			panic(fmt.Sprintf("epxpecting the separator "+
				"'%s' but non was found", separator))
		}
		prefixPart = string(t[0 : separatorIndex+1])
		valuePart = t[separatorIndex+1:]
	}
	// We Base58 implementation as the encoding to use for the generated ID;
	// the beauty of this implementation is that it preserves lexical ordering
	// as defined as in the ASCII table.
	return prefixPart + base58.EncodeAlphabet(valuePart, alphabets)
}

// Value converts the ID into a SQL driver value
func (t ID) Value() (driver.Value, error) {
	return t[:], nil
}

// Scan implements the sql.Scanner interface.
// it converts bytes.Buff, []byte, string, or nil into ID
func (t *ID) Scan(src interface{}) error {
	switch v := src.(type) {
	case nil:
		return t.scan(nil)
	case []byte:
		return t.scan(v)
	case bytes.Buffer:
		return t.scan(v.Bytes())
	case *bytes.Buffer:
		return t.scan(v.Bytes())
	case string:
		return t.scan([]byte(v))
	default:
		return fmt.Errorf("unable to scan type %T into ID", v)
	}
}

func (t ID) MarshalText() ([]byte, error) {
	return []byte(t.String()), nil
}

func (t *ID) UnmarshalText(b []byte) error {
	id, err := Parse(string(b))
	if err != nil {
		return err
	}
	*t = id
	return nil
}

func (t ID) MarshalBinary() ([]byte, error) {
	return t.Bytes(), nil
}

func (t *ID) UnmarshalBinary(b []byte) error {
	var err error
	*t, err = FromBytes(b)
	return err
}

func (t ID) MarshalJSON() ([]byte, error) {
	return []byte(`"` + t.String() + `"`), nil
}

func (t *ID) UnmarshalJSON(data []byte) error {
	var s string
	err := json.Unmarshal(data, &s)
	if err != nil {
		return err
	}
	*t, err = Parse(s)
	return err
}

func (t *ID) scan(b []byte) error {
	switch len(b) {
	case 0:
		return nil
	case byteLength, prefixAllowedLen + byteLength:
		return t.UnmarshalBinary(b)
	case stringEncodedLen, prefixAllowedLen + stringEncodedLen:
		return t.UnmarshalText(b)
	default:
		return errBytesSizeMismatch
	}
}

func newID(prefix string) (ID, error) {
	prefixLen := 0
	if err := validatePrefix(prefix, true); err != nil {
		return nil, err
	} else if len(prefix) > 0 {
		prefix += separator
		prefixLen = prefixAllowedLen + len(separator)
	}
	ts := uint64(time.Since(epochTime) / 100) // timestamp measured in 100 unit nanoseconds
	ts = (ts << 8) & 0xFFFFFFFFFFFFFF00       // the 56 least significant bits of the time
	rnd := getRandomBytes()                   // 96 bits randomness for time collisions
	id := make(ID, prefixLen+byteLength)
	copy(id[0:prefixLen], prefix)
	id[prefixLen+0] = byte(ts >> 56)
	id[prefixLen+1] = byte(ts >> 48)
	id[prefixLen+2] = byte(ts >> 40)
	id[prefixLen+3] = byte(ts >> 32)
	id[prefixLen+4] = byte(ts >> 24)
	id[prefixLen+5] = byte(ts >> 16)
	id[prefixLen+6] = byte(ts >> 8)
	copy(id[prefixLen+7:], rnd)
	return id, nil
}

func Parse(text string) (ID, error) {
	prefixIndex := strings.Index(text, separator)
	if prefixIndex != -1 {
		return parseInternal(text[:prefixIndex+1], text[prefixIndex+1:])
	} else if len(text) >= stringEncodedLen {
		return parseInternal("", text)
	}
	return nil, errStringSizeMismatch
}

func parseInternal(prefix, text string) (ID, error) {
	if len(text) < stringEncodedLen {
		return nil, errStringSizeMismatch
	}
	if err := validatePrefix(prefix, false); err != nil {
		return nil, err
	}
	bs, err := base58.DecodeAlphabet(text, alphabets)
	if err != nil {
		return nil, fmt.Errorf("invalid ID: must be a valid base58 text")
	}
	return FromBytes(append([]byte(prefix), bs...))
}

func FromBytes(bytes []byte) (ID, error) {
	id := make([]byte, len(bytes))
	if len(bytes) < byteLength {
		return nil, errBytesSizeMismatch
	}
	copy(id, bytes)
	return id, nil
}

func getRandomBytes() []byte {
	b := make([]byte, randomBytesLen)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return b
}

func validatePrefix(str string, separatorAsInvalid bool) error {
	if !separatorAsInvalid {
		str = strings.TrimSuffix(str, separator)
	}
	if strings.Contains(str, separator) {
		return errors.New("prefix must not contain '-'")
	}
	if strings.Contains(str, " ") {
		return errors.New("prefix must not contain whitespace")
	}
	if len(str) > 0 && len(str) != prefixAllowedLen {
		return fmt.Errorf("prefix must be %d characters", prefixAllowedLen)
	}
	return nil
}
