package historytest_test

import (
	"testing"

	"github.com/gollem-dev/gollem/internal/historytest"
	"github.com/m-mizutani/gt"
)

func TestDecode(t *testing.T) {
	decode := func(t *testing.T, data string) any {
		t.Helper()
		v, err := historytest.Decode([]byte(data))
		gt.NoError(t, err).Required()
		return v
	}

	t.Run("integers a float64 cannot tell apart stay different", func(t *testing.T) {
		gt.NotEqual(t,
			decode(t, `{"id":9007199254740992}`),
			decode(t, `{"id":9007199254740993}`),
		)
	})

	t.Run("documents differing only in key order and spacing are equal", func(t *testing.T) {
		gt.Equal(t,
			decode(t, `{"a":1,"b":[true,"x"]}`),
			decode(t, "{\n  \"b\": [true, \"x\"],\n  \"a\": 1\n}"),
		)
	})

	t.Run("invalid JSON is an error", func(t *testing.T) {
		_, err := historytest.Decode([]byte(`{"id":`))
		gt.Error(t, err)
	})
}

func TestLoadAndEqual(t *testing.T) {
	// Fixtures are chosen by subtest name, so this subtest reads
	// testdata/openai_round_trip/text_messages.from_openai.json.
	t.Run("text_messages", func(t *testing.T) {
		h := historytest.Load(t, "openai_round_trip", "openai")
		gt.A(t, h.Messages).Length(2)
		historytest.Equal(t, "openai_round_trip", "openai", h)
	})
}
