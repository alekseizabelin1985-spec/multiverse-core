package sender

import (
	"net/http"
	"testing"
	"time"
)

// TestTheBestEffortClientIsCappedAndOfItsOwn (review #2 of T-310, Mi-4): the
// client of a best-effort sender gives up after at most BestEffortHTTPTimeout,
// keeps a shorter Timeout it was given, and never changes the client it was
// built from — that one may serve the deliveries with DefaultHTTPTimeout.
func TestTheBestEffortClientIsCappedAndOfItsOwn(t *testing.T) {
	if BestEffortHTTPTimeout > 5*time.Second {
		t.Fatalf("BestEffortHTTPTimeout = %s, review #2 of T-310 wants at most 5 s", BestEffortHTTPTimeout)
	}
	if c := bestEffortClient(nil); c.Timeout != BestEffortHTTPTimeout {
		t.Fatalf("without a client: Timeout %s, want %s", c.Timeout, BestEffortHTTPTimeout)
	}
	shared := &http.Client{Timeout: DefaultHTTPTimeout}
	c := bestEffortClient(shared)
	if c.Timeout != BestEffortHTTPTimeout || c == shared {
		t.Fatalf("from a %s client: Timeout %s, same client %t", DefaultHTTPTimeout, c.Timeout, c == shared)
	}
	if shared.Timeout != DefaultHTTPTimeout {
		t.Fatalf("the client it was built from now has Timeout %s", shared.Timeout)
	}
	if c := bestEffortClient(&http.Client{}); c.Timeout != BestEffortHTTPTimeout {
		t.Fatalf("from a client without a Timeout: %s", c.Timeout)
	}
	if c := bestEffortClient(&http.Client{Timeout: time.Second}); c.Timeout != time.Second {
		t.Fatalf("a shorter Timeout must stay: %s", c.Timeout)
	}

	s, err := NewBestEffortTelegram(TelegramOptions{Token: "123456:ABCdefGHIjklMNOpqrSTUvwxYZ0123456789", Policy: DefaultPolicy})
	if err != nil {
		t.Fatal(err)
	}
	if s.policy != BestEffortPolicy {
		t.Fatalf("policy %+v, want BestEffortPolicy whatever the options say", s.policy)
	}
}
