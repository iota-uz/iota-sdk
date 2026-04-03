package shared

import (
	"net/url"
	"testing"
)

func TestDecoderSupportsFormTag(t *testing.T) {
	type query struct {
		Query string `form:"q"`
	}

	dst := &query{}
	if err := Decoder.Decode(dst, url.Values{"q": []string{"alpha"}}); err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if dst.Query != "alpha" {
		t.Fatalf("Decode() query = %q, want %q", dst.Query, "alpha")
	}
}
