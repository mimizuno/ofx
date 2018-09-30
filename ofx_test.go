package ofx

import (
	"bytes"
	"io/ioutil"
	"os"
	"testing"
	"time"
)

func verifyOfx(t *testing.T, _ofx *Ofx, acctNum string, routingID string) {
	if _ofx == nil {
		t.Errorf("Nil ofx\n")
	}

	if _ofx.AccountNumber != acctNum {
		t.Errorf("Wrong account number. Expected: %s Actual: %s\n", acctNum, _ofx.AccountNumber)
	}

	if _ofx.BankCode != routingID {
		t.Errorf("Wrong routing number. Expected: %s Actual: %s\n", routingID, _ofx.BankCode)
	}
}

func TestParseV102(t *testing.T) {
	f, err := os.Open("testdata/v102.ofx")
	if err != nil {
		t.Fatal(err)
	}

	_ofx, err := Parse(f)
	if err != nil {
		t.Error(err)
	}

	verifyOfx(t, _ofx, "098-121", "987654321")
}

func TestParseV103(t *testing.T) {
	f, err := os.Open("testdata/v103.ofx")
	if err != nil {
		t.Fatal(err)
	}

	_ofx, err := Parse(f)
	if err != nil {
		t.Error(err)
	}

	if _ofx == nil {
		t.Errorf("Nil ofx\n")
	}

	verifyOfx(t, _ofx, "098-121", "987654321")
}

func BenchmarkOFXParse(b *testing.B) {
	bts, err := ioutil.ReadFile("testdata/v103.ofx")
	if err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		r := bytes.NewReader(bts)
		if _, err := Parse(r); err != nil {
			b.Errorf("Error while parsing: %v\n", err)
		}
	}
}

func TestParseDateTime(t *testing.T) {
	pst := time.FixedZone("PST", -7)
	cases := []struct {
		format   string
		expected time.Time
	}{
		{format: "20070329", expected: time.Date(2007, 3, 29, 0, 0, 0, 0, time.UTC)},
		{format: "20070329131415", expected: time.Date(2007, 3, 29, 13, 14, 15, 0, time.UTC)},
		{format: "20070329131415.123", expected: time.Date(2007, 3, 29, 13, 14, 15, 123*1000*1000, time.UTC)},
		{format: "20070329[-8:PST]", expected: time.Date(2007, 3, 29, 0, 0, 0, 0, pst)},
		{format: "20070329131415[-8:PST]", expected: time.Date(2007, 3, 29, 13, 14, 15, 0, pst)},
		{format: "20070329131415.123[-8:PST]", expected: time.Date(2007, 3, 29, 13, 14, 15, 123*1000*1000, pst)},
	}

	for _, c := range cases {
		actual, err := parseDateTime(c.format)
		if err != nil {
			t.Errorf("Error occured: %v by %v", err, c.format)
		}
		if actual.Format(time.RFC3339) != c.expected.Format(time.RFC3339) {
			t.Errorf("expected: %v, actual: %v, diff:%v", c.expected, actual, c.expected.Sub(actual))
		}
	}
}
