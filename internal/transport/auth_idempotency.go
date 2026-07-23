package transport

import (
	"bytes"
	"crypto/sha256"
	"encoding"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

const (
	authIdempotencyKeyDomain             = "multidesk-auth-idempotency-key-v1"
	authIdempotencyBodyDomain            = "multidesk-auth-idempotency-body-v1"
	authIdempotencyRequestIdentityDomain = "multidesk-auth-idempotency-request-identity-v1"
)

// NormalizeIdempotencyKeyV1 implements the P2 wire normalization rule. Only
// outer HTTP optional whitespace (SP/HTAB) is removed; the key's case and all
// other punctuation remain identity-bearing.
func NormalizeIdempotencyKeyV1(value string) (string, error) {
	normalized := strings.Trim(value, " \t")
	if len(normalized) < 16 || len(normalized) > 128 {
		return "", fmt.Errorf("idempotency key length is invalid")
	}
	for _, value := range []byte(normalized) {
		if value < 0x21 || value > 0x7e || value == ',' {
			return "", fmt.Errorf("idempotency key contains a forbidden byte")
		}
	}
	return normalized, nil
}

// AuthIdempotencyKeyDigestV1 returns the server-global digest of a normalized
// P2 auth Idempotency-Key. Actor, route, and request facts deliberately do not
// participate in this digest; they are bound by the request identity below.
func AuthIdempotencyKeyDigestV1(normalizedKey string) ([sha256.Size]byte, error) {
	var result [sha256.Size]byte
	key, err := NormalizeIdempotencyKeyV1(normalizedKey)
	if err != nil || key != normalizedKey {
		return result, fmt.Errorf("idempotency key is not normalized")
	}
	framed, err := Frame([]byte(authIdempotencyKeyDomain), []byte("1"), []byte(key))
	if err != nil {
		return result, err
	}
	return sha256.Sum256(framed), nil
}

// AuthIdempotencyBodyDigestV1 binds the RFC 8785/JCS representation produced
// after endpoint-specific strict schema decoding.
func AuthIdempotencyBodyDigestV1(canonicalStrictJSON []byte) ([sha256.Size]byte, error) {
	var result [sha256.Size]byte
	if len(canonicalStrictJSON) < 2 || !json.Valid(canonicalStrictJSON) {
		return result, fmt.Errorf("canonical request JSON is invalid")
	}
	framed, err := Frame([]byte(authIdempotencyBodyDomain), []byte("1"), canonicalStrictJSON)
	if err != nil {
		return result, err
	}
	return sha256.Sum256(framed), nil
}

// AuthIdempotencyRequestIdentityDigestV1 binds every semantic request fact
// that must match before a global key may replay.
func AuthIdempotencyRequestIdentityDigestV1(serverOrigin, actorClass string, actorIdentityRaw []byte, operation, method, canonicalPath string, bodyDigest [sha256.Size]byte, canonicalIfMatch string) ([sha256.Size]byte, error) {
	var result [sha256.Size]byte
	if serverOrigin == "" || actorClass == "" || len(actorIdentityRaw) != sha256.Size || operation == "" || (method != "POST" && method != "DELETE") || !strings.HasPrefix(canonicalPath, "/v1/") || strings.ContainsAny(canonicalPath, "?#") || strings.HasSuffix(canonicalPath, "/") {
		return result, fmt.Errorf("auth idempotency request identity is invalid")
	}
	framed, err := Frame(
		[]byte(authIdempotencyRequestIdentityDomain), []byte("1"), []byte(serverOrigin),
		[]byte(actorClass), actorIdentityRaw, []byte(operation), []byte(method),
		[]byte(canonicalPath), bodyDigest[:], []byte(canonicalIfMatch),
	)
	if err != nil {
		return result, err
	}
	return sha256.Sum256(framed), nil
}

// CanonicalStrictJSONV1 performs endpoint schema decoding before producing
// RFC 8785 JSON Canonicalization Scheme bytes. Callers must supply the concrete
// generated request DTO as destination so unknown/missing/type-invalid fields
// fail before any idempotency lookup or row creation.
func CanonicalStrictJSONV1(contents []byte, limit int64, destination any) ([]byte, error) {
	if destination == nil {
		return nil, fmt.Errorf("canonical JSON destination is required")
	}
	if err := validateNoLoneJSONSurrogates(contents); err != nil {
		return nil, err
	}
	if err := DecodeStrictJSON(bytes.NewReader(contents), limit, destination); err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(contents))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, fmt.Errorf("decode canonical JSON value: %w", err)
	}
	if err := validateRequiredJSONMembers(reflect.TypeOf(destination), value, "$"); err != nil {
		return nil, err
	}
	var result bytes.Buffer
	if err := writeJCSValue(&result, value); err != nil {
		return nil, err
	}
	return result.Bytes(), nil
}

// CanonicalEmptyJSONObjectV1 is the only canonical body for bodyless P2 POST
// and DELETE operations.
func CanonicalEmptyJSONObjectV1(contents []byte) ([]byte, error) {
	var empty struct{}
	canonical, err := CanonicalStrictJSONV1(contents, 16, &empty)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(canonical, []byte("{}")) {
		return nil, fmt.Errorf("request body must be an empty JSON object")
	}
	return canonical, nil
}

// EmptyJSONObjectV1 is the strict Go transport type for P2 operations whose
// wire body is required to be exactly an empty JSON object. It deliberately
// implements JSON marshaling itself: struct{} would silently accept unknown
// members when used as a generated request model.
type EmptyJSONObjectV1 struct{}

// MarshalJSON always emits the sole canonical representation.
func (EmptyJSONObjectV1) MarshalJSON() ([]byte, error) {
	return []byte("{}"), nil
}

// UnmarshalJSON rejects every value except an empty JSON object.
func (*EmptyJSONObjectV1) UnmarshalJSON(contents []byte) error {
	_, err := CanonicalEmptyJSONObjectV1(contents)
	return err
}

func writeJCSValue(destination *bytes.Buffer, value any) error {
	switch value := value.(type) {
	case nil:
		destination.WriteString("null")
	case bool:
		if value {
			destination.WriteString("true")
		} else {
			destination.WriteString("false")
		}
	case string:
		if err := writeJCSString(destination, value); err != nil {
			return err
		}
	case json.Number:
		encoded, err := jcsNumber(value)
		if err != nil {
			return err
		}
		destination.WriteString(encoded)
	case []any:
		destination.WriteByte('[')
		for index, item := range value {
			if index != 0 {
				destination.WriteByte(',')
			}
			if err := writeJCSValue(destination, item); err != nil {
				return err
			}
		}
		destination.WriteByte(']')
	case map[string]any:
		keys := make([]string, 0, len(value))
		for key := range value {
			keys = append(keys, key)
		}
		sort.Slice(keys, func(left, right int) bool { return lessUTF16(keys[left], keys[right]) })
		destination.WriteByte('{')
		for index, key := range keys {
			if index != 0 {
				destination.WriteByte(',')
			}
			if err := writeJCSString(destination, key); err != nil {
				return err
			}
			destination.WriteByte(':')
			if err := writeJCSValue(destination, value[key]); err != nil {
				return err
			}
		}
		destination.WriteByte('}')
	default:
		return fmt.Errorf("unsupported canonical JSON value %T", value)
	}
	return nil
}

func writeJCSString(destination *bytes.Buffer, value string) error {
	if !utf8.ValidString(value) {
		return fmt.Errorf("canonical JSON string is not valid UTF-8")
	}
	destination.WriteByte('"')
	for _, value := range value {
		switch value {
		case '"', '\\':
			destination.WriteByte('\\')
			destination.WriteRune(value)
		case '\b':
			destination.WriteString(`\b`)
		case '\t':
			destination.WriteString(`\t`)
		case '\n':
			destination.WriteString(`\n`)
		case '\f':
			destination.WriteString(`\f`)
		case '\r':
			destination.WriteString(`\r`)
		default:
			if value < 0x20 {
				destination.WriteString(`\u00`)
				destination.WriteString(strconv.FormatInt(int64(value), 16))
				if value < 0x10 {
					bytes := destination.Bytes()
					last := bytes[len(bytes)-1]
					bytes[len(bytes)-1] = '0'
					destination.WriteByte(last)
				}
			} else {
				destination.WriteRune(value)
			}
		}
	}
	destination.WriteByte('"')
	return nil
}

func jcsNumber(value json.Number) (string, error) {
	parsed, err := strconv.ParseFloat(value.String(), 64)
	if err != nil || math.IsInf(parsed, 0) || math.IsNaN(parsed) {
		return "", fmt.Errorf("canonical JSON number is outside the IEEE-754 range")
	}
	if parsed == 0 {
		return "0", nil
	}
	absolute := math.Abs(parsed)
	if absolute >= 1e-6 && absolute < 1e21 {
		return strconv.FormatFloat(parsed, 'f', -1, 64), nil
	}
	encoded := strconv.FormatFloat(parsed, 'e', -1, 64)
	mantissa, exponent, ok := strings.Cut(encoded, "e")
	if !ok {
		return "", fmt.Errorf("canonical JSON exponent is invalid")
	}
	sign := ""
	if strings.HasPrefix(exponent, "+") || strings.HasPrefix(exponent, "-") {
		sign, exponent = exponent[:1], exponent[1:]
	}
	exponent = strings.TrimLeft(exponent, "0")
	if exponent == "" {
		exponent = "0"
	}
	return mantissa + "e" + sign + exponent, nil
}

func lessUTF16(left, right string) bool {
	leftUnits := utf16.Encode([]rune(left))
	rightUnits := utf16.Encode([]rune(right))
	limit := min(len(leftUnits), len(rightUnits))
	for index := range limit {
		if leftUnits[index] != rightUnits[index] {
			return leftUnits[index] < rightUnits[index]
		}
	}
	return len(leftUnits) < len(rightUnits)
}

func validateNoLoneJSONSurrogates(contents []byte) error {
	insideString := false
	for index := 0; index < len(contents); index++ {
		switch contents[index] {
		case '"':
			insideString = !insideString
		case '\\':
			if !insideString || index+1 >= len(contents) {
				continue
			}
			index++
			if contents[index] != 'u' {
				continue
			}
			unit, ok := parseJSONHexUnit(contents, index+1)
			if !ok {
				// The ordinary JSON decoder reports the malformed escape.
				continue
			}
			index += 4
			switch {
			case unit >= 0xd800 && unit <= 0xdbff:
				if index+6 >= len(contents) || contents[index+1] != '\\' || contents[index+2] != 'u' {
					return fmt.Errorf("canonical JSON contains a lone high surrogate")
				}
				low, ok := parseJSONHexUnit(contents, index+3)
				if !ok || low < 0xdc00 || low > 0xdfff {
					return fmt.Errorf("canonical JSON contains a lone high surrogate")
				}
				index += 6
			case unit >= 0xdc00 && unit <= 0xdfff:
				return fmt.Errorf("canonical JSON contains a lone low surrogate")
			}
		}
	}
	return nil
}

func parseJSONHexUnit(contents []byte, offset int) (uint16, bool) {
	if offset < 0 || offset+4 > len(contents) {
		return 0, false
	}
	var result uint16
	for _, value := range contents[offset : offset+4] {
		result <<= 4
		switch {
		case value >= '0' && value <= '9':
			result |= uint16(value - '0')
		case value >= 'a' && value <= 'f':
			result |= uint16(value-'a') + 10
		case value >= 'A' && value <= 'F':
			result |= uint16(value-'A') + 10
		default:
			return 0, false
		}
	}
	return result, true
}

var (
	jsonUnmarshalerType = reflect.TypeFor[json.Unmarshaler]()
	textUnmarshalerType = reflect.TypeFor[encoding.TextUnmarshaler]()
)

func validateRequiredJSONMembers(schema reflect.Type, value any, path string) error {
	if schema == nil {
		return fmt.Errorf("canonical JSON schema is missing")
	}
	for schema.Kind() == reflect.Pointer {
		schema = schema.Elem()
	}
	if schema.Kind() == reflect.Interface || reflect.PointerTo(schema).Implements(jsonUnmarshalerType) || reflect.PointerTo(schema).Implements(textUnmarshalerType) {
		return nil
	}
	switch schema.Kind() {
	case reflect.Struct:
		object, ok := value.(map[string]any)
		if !ok {
			return nil
		}
		for index := 0; index < schema.NumField(); index++ {
			field := schema.Field(index)
			if !field.IsExported() {
				continue
			}
			tag := field.Tag.Get("json")
			name, options, _ := strings.Cut(tag, ",")
			if name == "-" {
				continue
			}
			if name == "" {
				name = field.Name
			}
			member, present := object[name]
			optional := false
			for _, option := range strings.Split(options, ",") {
				optional = optional || option == "omitempty" || option == "omitzero"
			}
			if !present {
				if !optional {
					return fmt.Errorf("required JSON member %s.%s is missing", path, name)
				}
				continue
			}
			if err := validateRequiredJSONMembers(field.Type, member, path+"."+name); err != nil {
				return err
			}
		}
	case reflect.Slice, reflect.Array:
		items, ok := value.([]any)
		if !ok {
			return nil
		}
		for index, item := range items {
			if err := validateRequiredJSONMembers(schema.Elem(), item, path+"["+strconv.Itoa(index)+"]"); err != nil {
				return err
			}
		}
	case reflect.Map:
		object, ok := value.(map[string]any)
		if !ok {
			return nil
		}
		for key, item := range object {
			if err := validateRequiredJSONMembers(schema.Elem(), item, path+"."+key); err != nil {
				return err
			}
		}
	}
	return nil
}
