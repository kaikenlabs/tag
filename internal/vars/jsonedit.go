package vars

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
)

// JSON member names that carry variable names as data rather than as template
// expressions, and so need renaming alongside the {{ vars.x }} references.
const (
	varsMember     = "vars"
	requiresMember = "requires"
)

// span is a byte range within the original document.
type span struct {
	start, end int
}

// renameJSONDeclarations renames a variable where a TAG config file records it
// as data rather than as a template expression: the `vars` object key, and any
// entry of the top-level `requires` array. It returns the rewritten document and
// the number of replacements.
//
// The edit is a byte-level splice on the original document, which preserves key
// order, indentation and comments-by-formatting that a decode/encode round trip
// through map[string]any would destroy. Spans are applied back-to-front so
// earlier offsets stay valid.
func renameJSONDeclarations(data []byte, oldName, newName string) ([]byte, int, error) {
	spans, err := findDeclarationSpans(data, oldName)
	if err != nil {
		return nil, 0, err
	}
	if len(spans) == 0 {
		return data, 0, nil
	}

	quoted, err := json.Marshal(newName)
	if err != nil {
		return nil, 0, fmt.Errorf("encode variable name: %w", err)
	}

	// Apply back-to-front so each splice leaves earlier offsets valid.
	slices.SortFunc(spans, func(a, b span) int { return b.start - a.start })

	out := slices.Clone(data)
	for _, s := range spans {
		out = append(out[:s.start], append(slices.Clone(quoted), out[s.end:]...)...)
	}

	return out, len(spans), nil
}

// findDeclarationSpans locates the byte ranges of every JSON string token that
// declares oldName: the key `vars.<oldName>` and elements of the top-level
// `requires` array equal to oldName.
func findDeclarationSpans(data []byte, oldName string) ([]span, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	var spans []span

	err := walkJSONStrings(dec, nil, func(path []string, value string, isKey bool, end int) {
		if value != oldName {
			return
		}
		match := (isKey && len(path) == 1 && path[0] == varsMember) ||
			(!isKey && len(path) == 1 && path[0] == requiresMember)
		if !match {
			return
		}
		if start, ok := stringTokenStart(data, end); ok {
			spans = append(spans, span{start: start, end: end})
		}
	})
	if err != nil {
		return nil, fmt.Errorf("parse json: %w", err)
	}

	return spans, nil
}

// walkJSONStrings performs a recursive descent over the decoder's token stream,
// invoking fn for every string token with the path of enclosing object keys.
// Array elements inherit their array's path, so an element of the top-level
// "requires" array is reported with path ["requires"]. end is the input offset
// just past the token's closing quote.
func walkJSONStrings(
	dec *json.Decoder,
	path []string,
	fn func(path []string, value string, isKey bool, end int),
) error {
	tok, err := dec.Token()
	if err != nil {
		return err
	}

	delim, ok := tok.(json.Delim)
	if !ok {
		if s, isString := tok.(string); isString {
			fn(path, s, false, int(dec.InputOffset()))
		}
		return nil
	}

	switch delim {
	case '{':
		for dec.More() {
			keyTok, keyErr := dec.Token()
			if keyErr != nil {
				return keyErr
			}
			key, _ := keyTok.(string)
			fn(path, key, true, int(dec.InputOffset()))

			if valErr := walkJSONStrings(dec, append(slices.Clone(path), key), fn); valErr != nil {
				return valErr
			}
		}
	case '[':
		for dec.More() {
			if elemErr := walkJSONStrings(dec, path, fn); elemErr != nil {
				return elemErr
			}
		}
	}

	// Consume the matching closing delimiter.
	_, err = dec.Token()
	return err
}

// stringTokenStart walks backwards from the offset just past a JSON string
// token to the offset of its opening quote. Scanning for the quote rather than
// subtracting the decoded length is what makes escaped names splice correctly:
// a key written with a \u escape is longer on the wire than its decoded value,
// so offset arithmetic would cut into the middle of it.
func stringTokenStart(data []byte, end int) (int, bool) {
	if end <= 0 || end > len(data) || data[end-1] != '"' {
		return 0, false
	}
	for i := end - 2; i >= 0; i-- {
		if data[i] != '"' {
			continue
		}
		backslashes := 0
		for j := i - 1; j >= 0 && data[j] == '\\'; j-- {
			backslashes++
		}
		if backslashes%2 == 0 {
			return i, true
		}
	}
	return 0, false
}
