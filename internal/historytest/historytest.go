// Package historytest holds the gollem.History values that tests of different LLM
// providers exchange to check cross-provider history conversion.
//
// The conversion functions of each provider package are unexported, so a single test
// can no longer convert one provider's messages into another's. Each conversion is
// instead split at the gollem.History in between: the source provider's test checks
// that its messages convert to the stored History, and the target provider's test
// converts the same stored History to its own messages. Both tests read one file, so
// the History produced on one side is exactly the History consumed on the other.
//
// The file is chosen by the running subtest, so a test using a fixture must have the
// same name in every provider package that takes part in the conversion. This also
// makes `go test ./llm/... -run TestOpenAIRoundTrip` run every part of a conversion.
// Fixtures live in testdata/<conversion>/<subtest name>.from_<provider>.json, where
// <provider> is the provider whose messages the History was converted from.
//
// The fixtures are serialized Histories, so they carry gollem.HistoryVersion. When
// the version is incremented, Load fails with gollem.ErrHistoryVersionMismatch until
// every fixture is updated to the new format and version.
package historytest

import (
	"embed"
	"encoding/json"
	"strings"
	"testing"

	"github.com/gollem-dev/gollem"
	"github.com/gollem-dev/gollem/internal/jsonutil"
	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/gt"
)

//go:embed testdata
var fixtures embed.FS

func read(t *testing.T, conversion, provider string) []byte {
	t.Helper()
	_, subtest, ok := strings.Cut(t.Name(), "/")
	if !ok {
		t.Fatalf("historytest must be called from a subtest, got %q", t.Name())
	}
	data, err := fixtures.ReadFile("testdata/" + conversion + "/" + subtest + ".from_" + provider + ".json")
	gt.NoError(t, err).Required()
	return data
}

// Load returns the History that the current subtest of the given conversion stores
// as converted from the messages of provider.
func Load(t *testing.T, conversion, provider string) *gollem.History {
	t.Helper()
	var h gollem.History
	gt.NoError(t, json.Unmarshal(read(t, conversion, provider), &h)).Required()
	return &h
}

// Equal checks that actual serializes to the same JSON as the History that Load
// returns for the same arguments. It compares the serialized form because that is
// what Load hands to the other provider, so every field the other side can observe
// is covered.
func Equal(t *testing.T, conversion, provider string, actual *gollem.History) {
	t.Helper()
	expected, err := decode(read(t, conversion, provider))
	gt.NoError(t, err).Required()

	data, err := json.Marshal(actual)
	gt.NoError(t, err).Required()
	got, err := decode(data)
	gt.NoError(t, err).Required()

	gt.Equal(t, expected, got)
}

// decode turns serialized JSON into a value that compares equal only for equal JSON
// documents. jsonutil.Decode keeps integers wider than a float64 can hold exactly,
// which the providers also keep, so two such integers do not compare as equal.
func decode(data []byte) (any, error) {
	var v any
	if err := jsonutil.Decode(data, &v); err != nil {
		return nil, goerr.Wrap(err, "failed to decode history JSON")
	}
	return v, nil
}
