package ofx

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"math/big"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// AccountType indicates type of account represented by OFX document.
type AccountType int

const (
	// UNKNOWN - Account type could not be determined
	UNKNOWN AccountType = iota
	// CHECKING - Checking account
	CHECKING AccountType = iota
	// SAVING - Savings account
	SAVING AccountType = iota
)

//go:generate stringer -type=TransactionType
// TransactionType indicates type of transaction (Debit/Credit).
type TransactionType int

const (
	DEBIT  TransactionType = iota
	CREDIT TransactionType = iota
)

type nextKey int

const (
	none            nextKey = iota
	acctID          nextKey = iota
	branchID        nextKey = iota
	bankID          nextKey = iota
	transAmount     nextKey = iota
	transDatePosted nextKey = iota
	transUserDate   nextKey = iota
	transID         nextKey = iota
	transDesc       nextKey = iota
	transMemo       nextKey = iota
)

type Amount struct {
	Value big.Rat
}

func (a *Amount) ParseFromString(s string) error {
	_, ok := a.Value.SetString(s)
	if !ok {
		return fmt.Errorf("Unable to parse string '%s' as an amount\n", s)
	}

	return nil
}

type Transaction struct {
	Type        TransactionType
	Description string
	Memo        string
	PostedDate  time.Time
	UserDate    time.Time
	ID          string
	Amount      Amount
}

func (t Transaction) String() string {
	return fmt.Sprintf("T: %s Desc: %s Post Date: %s ID: %s Amount: %s", t.Type, t.Description, t.PostedDate, t.ID, t.Amount.Value.String())
}

// Ofx contains a parsed Ofx document.
type Ofx struct {
	Type          AccountType
	BankCode      string
	BranchCode    string
	AccountNumber string
	Transactions  []*Transaction
}

func (o Ofx) String() string {
	var buf bytes.Buffer

	buf.WriteString(fmt.Sprintf("Account Type: %v\nBank Code: %s\nBranch Code: %s\nAccount Number: %s\n", o.Type, o.BankCode, o.BranchCode, o.AccountNumber))

	for _, t := range o.Transactions {
		buf.WriteString(fmt.Sprintf("%s\n", t))
	}

	return buf.String()
}

// Parse parses an input stream and produces an Ofx instance summarizing it. In case of any errors
// during the parse, a non-nil error is returned.
func Parse(f io.Reader) (*Ofx, error) {
	ofx := &Ofx{Transactions: []*Transaction{}}
	stack := make([]string, 1000)
	stackPos := 0

	next := none
	var trans *Transaction = nil

	dec := xml.NewDecoder(f)

	tok, err := dec.RawToken()
	for err == nil {
		switch t := tok.(type) {
		case xml.StartElement:
			stack[stackPos] = t.Name.Local
			stackPos++

			switch t.Name.Local {
			case "ACCTID":
				next = acctID

			case "BRANCHID":
				next = branchID

			case "BANKID":
				next = bankID

			case "STMTTRN":
				trans = &Transaction{}

			case "DTPOSTED":
				next = transDatePosted

			case "FITID":
				next = transID

			case "TRNAMT":
				next = transAmount

			case "NAME":
				next = transDesc
			case "MEMO":
				next = transMemo
			}

		case xml.CharData:
			var b bytes.Buffer
			if _, err := b.Write(t); err != nil {
				return nil, err
			}
			res := strings.TrimSpace(b.String())

			switch next {
			case acctID:
				ofx.AccountNumber = res

			case branchID:
				ofx.BranchCode = res

			case bankID:
				ofx.BankCode = res

			case transDesc:
				trans.Description = res

			case transMemo:
				trans.Memo = res

			case transID:
				trans.ID = res

			case transDatePosted:
				var t time.Time
				if t, err = parseDateTime(res); err != nil {
					return nil, err
				}
				trans.PostedDate = t

			case transAmount:
				if err := trans.Amount.ParseFromString(res); err != nil {
					return nil, err
				}

				if trans.Amount.Value.Sign() == 1 {
					trans.Type = CREDIT
				} else {
					trans.Type = DEBIT
				}
			}

			next = none

		case xml.EndElement:
			for stackPos != 0 {
				if stack[stackPos-1] == "STMTTRN" {
					ofx.Transactions = append(ofx.Transactions, trans)
					trans = nil
				}

				if stack[stackPos-1] == t.Name.Local {
					stackPos--
					break
				}
				stackPos--
			}

		default:
			log.Printf("Unknown: %T %s\n", t, t)
		}

		tok, err = dec.RawToken()

		if err != nil && err != io.EOF {
			log.Printf("Error: %s\n", err)
		}
	}

	return ofx, nil
}

func reFindStringSubmatchMap(re *regexp.Regexp, s string) map[string]string {
	m := re.FindStringSubmatch(s)
	result := make(map[string]string)
	for i, name := range re.SubexpNames() {
		if i != 0 && name != "" {
			result[name] = m[i]
		}
	}
	return result
}

func parseDateTime(res string) (t time.Time, err error) {
	re := regexp.MustCompile(`(?P<datetime>[.0-9]+)(?:\[(?P<offset>.+):(?P<name>.+)\])?`)
	m := reFindStringSubmatchMap(re, res)

	loc := time.UTC
	if m["offset"] != "" {
		offset, _ := strconv.ParseInt(m["offset"], 10, 32)
		loc = time.FixedZone(m["name"], int(offset))
	}

	pattern := []string{
		"20060102150405.999",
		"20060102150405",
		"20060102",
	}

	for _, pat := range pattern {
		t, err = time.Parse(pat, m["datetime"])
		t = time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), loc)
		if err == nil {
			return
		}
	}
	err = fmt.Errorf("Invalid date posted string: '%s'", res)
	return
}
