package searching

import (
	"errors"
	"testing"
)

func TestCoalesceDDGThenBing(t *testing.T) {
	ddgOK := []Link{{"https://ddg.example", "ddg"}}
	bingOK := []Link{{"https://bing.example", "bing"}}
	ddgErr := errors.New("duckduckgo blocked the request (status 202)")
	bingErr := errors.New("bing request failed")

	t.Run("ddg success skips bing", func(t *testing.T) {
		links, used, err := coalesceDDGThenBing(ddgOK, nil, nil, bingErr)
		if err != nil || used != EngineDuckDuckGo || len(links) != 1 {
			t.Fatalf("got links=%v used=%q err=%v", links, used, err)
		}
	})

	t.Run("ddg blocked falls back to bing", func(t *testing.T) {
		links, used, err := coalesceDDGThenBing(nil, ddgErr, bingOK, nil)
		if err != nil || used != EngineBing || len(links) != 1 || links[0][0] != "https://bing.example" {
			t.Fatalf("got links=%v used=%q err=%v", links, used, err)
		}
	})

	t.Run("ddg empty falls back to bing", func(t *testing.T) {
		links, used, err := coalesceDDGThenBing(nil, nil, bingOK, nil)
		if err != nil || used != EngineBing || len(links) != 1 {
			t.Fatalf("got links=%v used=%q err=%v", links, used, err)
		}
	})

	t.Run("both fail combines errors", func(t *testing.T) {
		_, used, err := coalesceDDGThenBing(nil, ddgErr, nil, bingErr)
		if err == nil || used != EngineDuckDuckGo {
			t.Fatalf("want combined error, used=ddg; got used=%q err=%v", used, err)
		}
		if want := "duckduckgo: duckduckgo blocked the request (status 202); bing: bing request failed"; err.Error() != want {
			t.Fatalf("err=%q want %q", err.Error(), want)
		}
	})
}
