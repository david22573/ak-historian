package duckdbquery

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestQuoteString(t *testing.T) {
	assert.Equal(t, "'foo'", QuoteString("foo"))
	assert.Equal(t, "'foo''bar'", QuoteString("foo'bar"))
	assert.Equal(t, "''", QuoteString(""))
}
